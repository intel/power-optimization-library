package power

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

func TestNode_SetGetName(t *testing.T) {
	assert := assert.New(t)
	initName := "hi mom"
	newName := "new name"
	n := &nodeImpl{Name: initName}

	assert.Equal(initName, n.GetName())

	n.SetNodeName(newName)
	assert.Equal(n.Name, newName)
}

func TestNodeImpl_AddExclusivePool(t *testing.T) {
	assert := assert.New(t)

	poolName := "poolName"
	profile := &profileImpl{}
	node := &nodeImpl{}

	pool, err := node.AddExclusivePool(poolName, profile)
	assert.Nil(err)

	poolObj, _ := pool.(*poolImpl)
	assert.Contains(node.ExclusivePools, pool)
	assert.Equal(poolObj.Name, poolName)
	assert.Empty(poolObj.Cores)
	assert.Equal(poolObj.PowerProfile, profile)

	_, err = node.AddExclusivePool(poolName, profile)
	assert.Error(err)
}

func TestNodeImpl_RemoveExclusivePool(t *testing.T) {
	assert := assert.New(t)
	core := new(coreMock)
	core.On("GetID").Return(0)
	core.On("updateValues", "", 0, 0).Return(nil)
	core.On("getReserved").Return(false)
	p1 := &poolImpl{Name: "p1"}
	p2 := &poolImpl{
		Name:  "p2",
		Cores: []Core{core},
	}
	node := &nodeImpl{
		ExclusivePools: []Pool{p1, p2},
		SharedPool: &poolImpl{
			PowerProfile: &profileImpl{},
		},
	}

	assert.Nil(node.RemoveExclusivePool("p1"))
	assert.NotContains(node.ExclusivePools, p1)
	assert.Contains(node.ExclusivePools, p2)

	assert.Error(node.RemoveExclusivePool("p1"))

	assert.NoError(node.RemoveExclusivePool("p2"))
}

func TestNodeImpl_initializeDefaultPool(t *testing.T) {
	mockCPUData := map[string]string{
		"min": "100",
		"max": "123",
		"epp": "epp",
	}
	mockedCores := map[string]map[string]string{
		"cpu0": mockCPUData,
		"cpu1": mockCPUData,
		"cpu2": mockCPUData,
		"cpu3": mockCPUData,
	}
	defer setupCoreTests(mockedCores)()
	node := &nodeImpl{}

	assert := assert.New(t)
	assert.Nil(node.initializeDefaultPool())

	assert.Equal(sharedPoolName, node.SharedPool.(*poolImpl).Name)
	assert.Equal(len(mockedCores), len(node.SharedPool.(*poolImpl).Cores))

}
func TestNodeImpl_AddSharedPool(t *testing.T) {
	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("setReserved", false).Return()
		core.On("updateValues", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		cores[i] = core
	}

	node := &nodeImpl{
		SharedPool: &poolImpl{
			Cores: cores,
		},
	}

	profile := &profileImpl{}

	for _, core := range cores[:2] {
		core.(*coreMock).On("getReserved").Return(true)
	}
	for _, core := range cores[2:] {
		core.(*coreMock).On("getReserved").Return(false)
	}
	assert.Nil(t, node.AddSharedPool([]int{0, 1}, profile))

	cores[1].(*coreMock).AssertNotCalled(t, "updateValues", mock.Anything, mock.Anything, mock.Anything)
	cores[2].(*coreMock).AssertCalled(t, "setReserved", false)
	cores[2].(*coreMock).AssertCalled(t, "updateValues", mock.Anything, mock.Anything, mock.Anything)

	assert.ElementsMatch(t, node.SharedPool.(*poolImpl).Cores, cores)
}

func TestNodeImpl_RemoveCoreFromExclusivePool(t *testing.T) {
	assert := assert.New(t)
	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("updateValues", "", 0, 0).Return(nil)
		core.On("getReserved").Return(false)
		cores[i] = core
	}
	pool := &poolImpl{
		Name:         "test",
		Cores:        cores,
		PowerProfile: &profileImpl{},
	}
	node := &nodeImpl{
		Name:           "",
		ExclusivePools: []Pool{pool},
		SharedPool: &poolImpl{
			PowerProfile: &profileImpl{},
		},
	}
	coresToRemove := make([]int, 0)
	for _, core := range cores[:2] {
		coresToRemove = append(coresToRemove, core.GetID())
	}
	coresToPreserve := cores[2:]
	assert.Nil(node.RemoveCoresFromExclusivePool("test", coresToRemove))

	assert.ElementsMatch(node.ExclusivePools[0].(*poolImpl).Cores, coresToPreserve)
	coresToPreserve[0].(*coreMock).AssertNotCalled(t, "updateValues", "", 0, 0)
	node.SharedPool.(*poolImpl).Cores[0].(*coreMock).AssertCalled(t, "updateValues", "", 0, 0)
}

func TestNodeImpl_AddCoresToExclusivePool(t *testing.T) {
	assert := assert.New(t)
	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("updateValues", "", 0, 0).Return(nil)
		core.On("getReserved").Return(false)
		cores[i] = core
	}
	node := &nodeImpl{
		Name: "",
		ExclusivePools: []Pool{
			&poolImpl{
				Name:         "test",
				Cores:        make([]Core, 0),
				PowerProfile: &profileImpl{},
			},
		},
		SharedPool: &poolImpl{
			Cores: cores,
		},
	}
	// cores moved = cores[:2]
	// cores remain = cores[2:]
	var movedCoresIds []int
	for _, core := range cores[:2] {
		movedCoresIds = append(movedCoresIds, core.GetID())
	}
	assert.Nil(node.AddCoresToExclusivePool("test", movedCoresIds))

	assert.ElementsMatch(node.SharedPool.(*poolImpl).Cores, cores[2:])
	assert.Len(node.ExclusivePools[0].(*poolImpl).Cores, 2)
	for _, core := range node.ExclusivePools[0].(*poolImpl).Cores {
		core.(*coreMock).AssertCalled(t, "updateValues", "", 0, 0)
	}
}

func TestNodeImpl_UpdateProfile(t *testing.T) {
	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("getReserved").Return(false)
		core.On("GetID").Return(i)
		core.On("updateValues", "", 1*1000, 100*1000).Return(nil)
		cores[i] = core
	}
	node := nodeImpl{
		ExclusivePools: []Pool{
			&poolImpl{
				Cores:        cores,
				PowerProfile: &profileImpl{Name: "powah"},
			},
		},
		SharedPool: &poolImpl{},
	}

	assert.Nil(t, node.UpdateProfile("powah", 1, 100, ""))
	for _, core := range node.ExclusivePools[0].(*poolImpl).Cores {
		core.(*coreMock).AssertCalled(t, "updateValues", "", 1*1000, 100*1000)
	}
	assert.Equal(t, "powah", node.ExclusivePools[0].(*poolImpl).PowerProfile.(*profileImpl).Name)

}

func TestNodeImpl_RemoveSharedPool(t *testing.T) {
	assert := assert.New(t)
	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("restoreValues").Return(nil)
		core.On("setReserved", mock.Anything).Return()
		if i < 2 {
			core.On("getReserved").Return(true)
		} else {
			core.On("getReserved").Return(false)
		}
		cores[i] = core
	}
	node := &nodeImpl{
		SharedPool: &poolImpl{
			Cores: cores,
		},
	}

	assert.NoError(node.RemoveSharedPool())
	for _, core := range cores[:2] {
		core.(*coreMock).AssertNotCalled(t, "restoreValues")
	}
	for _, core := range cores[2:] {
		core.(*coreMock).AssertCalled(t, "restoreValues")
	}
}

func TestNodeImpl_GetReservedCoreIds(t *testing.T) {
	assert := assert.New(t)
	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("GetID").Return(i)
		if i < 2 {
			core.On("getReserved").Return(true)
		} else {
			core.On("getReserved").Return(false)
		}
		cores[i] = core
	}

	node := &nodeImpl{
		SharedPool: &poolImpl{
			Cores: cores,
		},
	}
	assert.ElementsMatch([]int{0, 1}, node.GetReservedCoreIds())
}

func TestNodeImpl_AddProfile(t *testing.T) {
	core := &coreMock{}
	node := &nodeImpl{
		Name:           "",
		ExclusivePools: make([]Pool, 0),
		SharedPool: &poolImpl{
			Cores: []Core{core},
		},
	}
	profile, err := node.AddProfile("poolname", 0, 100, "epp")
	assert.Nil(t, err)

	assert.Equal(t, profile, node.ExclusivePools[0].(*poolImpl).PowerProfile)
	assert.Equal(t, 100*1000, profile.GetMaxFreq())
	assert.Equal(t, "poolname", node.ExclusivePools[0].(*poolImpl).Name)
}

func TestNodeImpl_GetProfile(t *testing.T) {
	node := &nodeImpl{
		ExclusivePools: []Pool{
			&poolImpl{PowerProfile: &profileImpl{Name: "p0"}},
			&poolImpl{PowerProfile: &profileImpl{Name: "p1"}},
			&poolImpl{PowerProfile: &profileImpl{Name: "p2"}},
		},
		SharedPool: &poolImpl{PowerProfile: &profileImpl{Name: sharedPoolName}},
	}
	assert.Equal(t, node.ExclusivePools[1].(*poolImpl).PowerProfile, node.GetProfile("p1"))
	assert.Equal(t, node.SharedPool.(*poolImpl).PowerProfile, node.GetProfile(sharedPoolName))
	assert.Nil(t, node.GetProfile("non existing"))
}

func TestNodeImpl_GetExclusivePool(t *testing.T) {
	node := &nodeImpl{
		ExclusivePools: []Pool{
			&poolImpl{Name: "p0"},
			&poolImpl{Name: "p1"},
			&poolImpl{Name: "p2"},
		},
	}
	assert.Equal(t, node.ExclusivePools[1], node.GetExclusivePool("p1"))
	assert.Nil(t, node.GetExclusivePool("non existent"))
}

func TestNodeImpl_GetSharedPool(t *testing.T) {
	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("GetID").Return(i)
		if i < 2 {
			core.On("getReserved").Return(true)
		} else {
			core.On("getReserved").Return(false)
		}
		cores[i] = core
	}

	node := &nodeImpl{
		SharedPool: &poolImpl{
			Name:         sharedPoolName,
			Cores:        cores,
			PowerProfile: &profileImpl{},
		},
	}
	sharedPool := node.GetSharedPool().(*poolImpl)
	assert.Equal(t, sharedPoolName, sharedPool.Name)
	assert.ElementsMatch(t, cores[2:], sharedPool.Cores)
	assert.Equal(t, node.SharedPool.(*poolImpl).PowerProfile, sharedPool.PowerProfile)

}

func TestNodeImpl_DeleteProfile(t *testing.T) {
	var sharedCores []Core
	for i := 0; i < 4; i++ {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("updateValues", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		core.On("getReserved").Return(false)
		core.On("setReserved", true).Return()
		core.On("restoreValues").Return(nil)
		sharedCores = append(sharedCores, core)
	}
	sharedCoresCopy := make([]Core, len(sharedCores))
	copy(sharedCoresCopy, sharedCores)

	var p1cores []Core
	for i := 4; i < 8; i++ {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("updateValues", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		core.On("getReserved").Return(false)
		core.On("setReserved", true).Return()
		core.On("restoreValues").Return(nil)
		p1cores = append(p1cores, core)
	}
	p1copy := make([]Core, len(p1cores))
	copy(p1copy, p1cores)

	var p2cores []Core
	for i := 8; i < 12; i++ {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("updateValues", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		core.On("getReserved").Return(false)
		p2cores = append(p2cores, core)
	}
	p2copy := make([]Core, len(p2cores))
	copy(p2copy, p2cores)

	node := &nodeImpl{
		ExclusivePools: []Pool{
			&poolImpl{
				Name:         "pool1",
				Cores:        p1cores,
				PowerProfile: &profileImpl{Name: "profile1"},
			},
			&poolImpl{
				Name:         "pool2",
				Cores:        p2cores,
				PowerProfile: &profileImpl{Name: "profile2"}},
		},
		SharedPool: &poolImpl{
			Name:         sharedPoolName,
			Cores:        sharedCores,
			PowerProfile: &profileImpl{Name: sharedPoolName},
		},
	}
	assert.NoError(t, node.DeleteProfile("profile1"))
	assert.Len(t, node.ExclusivePools, 1)
	assert.Equal(t, "profile2", node.ExclusivePools[0].(*poolImpl).PowerProfile.(*profileImpl).Name)
	assert.ElementsMatch(t, node.ExclusivePools[0].(*poolImpl).Cores, p2copy)
	assert.ElementsMatch(t, node.SharedPool.(*poolImpl).Cores, append(sharedCoresCopy, p1copy...))

	assert.NoError(t, node.DeleteProfile(sharedPoolName))
	assert.Nil(t, node.SharedPool.(*poolImpl).PowerProfile)
	for _, core := range p1copy {
		core.(*coreMock).AssertCalled(t, "setReserved", true)
	}

	assert.Error(t, node.DeleteProfile("not existing"))
}
