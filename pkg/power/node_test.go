package power

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type nodeTestsSuite struct {
	suite.Suite
}

func (s *nodeTestsSuite) BeforeTest(suiteName, testName string) {
	supportedFeatureErrors[SSTBFFeature] = nil
	supportedFeatureErrors[CStatesFeature] = nil
}

func (s *nodeTestsSuite) AfterTest(suiteName, testName string) {
	supportedFeatureErrors[SSTBFFeature] = uninitialisedErr
	supportedFeatureErrors[CStatesFeature] = uninitialisedErr
}

func TestNode(t *testing.T) {
	suite.Run(t, new(nodeTestsSuite))
}

func (s *nodeTestsSuite) TestSetGetName() {
	initName := "hi mom"
	newName := "new name"
	n := &nodeImpl{Name: initName}

	s.Equal(initName, n.GetName())

	n.SetNodeName(newName)
	s.Equal(n.Name, newName)
}

func (s *nodeTestsSuite) TestAddExclusivePool() {
	poolName := "poolName"
	profile := &profileImpl{}
	node := &nodeImpl{}

	pool, err := node.AddExclusivePool(poolName, profile)
	s.Nil(err)

	poolObj, _ := pool.(*poolImpl)
	s.Contains(node.ExclusivePools, pool)
	s.Equal(poolObj.Name, poolName)
	s.Empty(poolObj.Cores)
	s.Equal(poolObj.PowerProfile, profile)

	_, err = node.AddExclusivePool(poolName, profile)
	s.Error(err)
}

func (s *nodeTestsSuite) TestRemoveExclusivePool() {
	core := new(coreMock)
	core.On("GetID").Return(0)
	core.On("updateFreqValues", "", 0, 0).Return(nil)
	core.On("getReserved").Return(false)
	core.On("setPool", mock.Anything).Return()
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

	s.Nil(node.RemoveExclusivePool("p1"))
	s.NotContains(node.ExclusivePools, p1)
	s.Contains(node.ExclusivePools, p2)

	s.Error(node.RemoveExclusivePool("p1"))

	s.NoError(node.RemoveExclusivePool("p2"))
}

func (s *nodeTestsSuite) TestInitializeDefaultPool() {
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

	s.Nil(node.initializeDefaultPool())

	s.Equal(sharedPoolName, node.SharedPool.(*poolImpl).Name)
	s.Equal(len(mockedCores), len(node.SharedPool.(*poolImpl).Cores))
	for _, core := range node.SharedPool.(*poolImpl).Cores {
		s.Equal(node.SharedPool, core.getPool())
	}

}
func (s *nodeTestsSuite) TestAddSharedPool() {
	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("setReserved", false).Return()
		core.On("updateFreqValues", mock.Anything, mock.Anything, mock.Anything).Return(nil)
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
	s.Nil(node.AddSharedPool([]int{0, 1}, profile))

	cores[1].(*coreMock).AssertNotCalled(s.T(), "updateFreqValues", mock.Anything, mock.Anything, mock.Anything)
	cores[2].(*coreMock).AssertCalled(s.T(), "setReserved", false)
	cores[2].(*coreMock).AssertCalled(s.T(), "updateFreqValues", mock.Anything, mock.Anything, mock.Anything)

	s.ElementsMatch(node.SharedPool.(*poolImpl).Cores, cores)
}

func (s *nodeTestsSuite) TestRemoveCoreFromExclusivePool() {
	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("updateFreqValues", "", 0, 0).Return(nil)
		core.On("getReserved").Return(false)
		core.On("setPool", mock.Anything).Return()
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
	s.Nil(node.RemoveCoresFromExclusivePool("test", coresToRemove))

	s.ElementsMatch(node.ExclusivePools[0].(*poolImpl).Cores, coresToPreserve)
	coresToPreserve[0].(*coreMock).AssertNotCalled(s.T(), "updateFreqValues", "", 0, 0)
	node.SharedPool.(*poolImpl).Cores[0].(*coreMock).AssertCalled(s.T(), "updateFreqValues", "", 0, 0)
}

func (s *nodeTestsSuite) TestAddCoresToExclusivePool() {
	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("updateFreqValues", "", 0, 0).Return(nil)
		core.On("getReserved").Return(false)
		core.On("setPool", mock.Anything).Return()
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
	s.Nil(node.AddCoresToExclusivePool("test", movedCoresIds))

	s.ElementsMatch(node.SharedPool.(*poolImpl).Cores, cores[2:])
	s.Len(node.ExclusivePools[0].(*poolImpl).Cores, 2)
	for _, core := range node.ExclusivePools[0].(*poolImpl).Cores {
		core.(*coreMock).AssertCalled(s.T(), "updateFreqValues", "", 0, 0)
	}
}

func (s *nodeTestsSuite) TestUpdateProfile() {
	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("getReserved").Return(false)
		core.On("GetID").Return(i)
		core.On("updateFreqValues", "", 1*1000, 100*1000).Return(nil)
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

	s.Nil(node.UpdateProfile("powah", 1, 100, ""))
	for _, core := range node.ExclusivePools[0].(*poolImpl).Cores {
		core.(*coreMock).AssertCalled(s.T(), "updateFreqValues", "", 1*1000, 100*1000)
	}
	s.Equal("powah", node.ExclusivePools[0].(*poolImpl).PowerProfile.(*profileImpl).Name)

}

func (s *nodeTestsSuite) TestRemoveSharedPool() {

	cores := make([]Core, 4)
	for i := range cores {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("restoreFrequencies").Return(nil)
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

	s.NoError(node.RemoveSharedPool())
	for _, core := range cores[:2] {
		core.(*coreMock).AssertNotCalled(s.T(), "restoreFrequencies")
	}
	for _, core := range cores[2:] {
		core.(*coreMock).AssertCalled(s.T(), "restoreFrequencies")
	}
}

func (s *nodeTestsSuite) TestGetReservedCoreIds() {
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
	s.ElementsMatch([]int{0, 1}, node.GetReservedCoreIds())
}

func (s *nodeTestsSuite) TestAddProfile() {
	core := &coreMock{}
	node := &nodeImpl{
		Name:           "",
		ExclusivePools: make([]Pool, 0),
		SharedPool: &poolImpl{
			Cores: []Core{core},
		},
	}
	profile, err := node.AddProfile("poolname", 0, 100, "epp")
	s.Nil(err)

	s.Equal(profile, node.ExclusivePools[0].(*poolImpl).PowerProfile)
	s.Equal(100*1000, profile.GetMaxFreq())
	s.Equal("poolname", node.ExclusivePools[0].(*poolImpl).Name)
}

func (s *nodeTestsSuite) TestGetProfile() {
	node := &nodeImpl{
		ExclusivePools: []Pool{
			&poolImpl{PowerProfile: &profileImpl{Name: "p0"}},
			&poolImpl{PowerProfile: &profileImpl{Name: "p1"}},
			&poolImpl{PowerProfile: &profileImpl{Name: "p2"}},
		},
		SharedPool: &poolImpl{PowerProfile: &profileImpl{Name: sharedPoolName}},
	}
	s.Equal(node.ExclusivePools[1].(*poolImpl).PowerProfile, node.GetProfile("p1"))
	s.Equal(node.SharedPool.(*poolImpl).PowerProfile, node.GetProfile(sharedPoolName))
	s.Nil(node.GetProfile("non existing"))
}

func (s *nodeTestsSuite) TestGetExclusivePool() {
	node := &nodeImpl{
		ExclusivePools: []Pool{
			&poolImpl{Name: "p0"},
			&poolImpl{Name: "p1"},
			&poolImpl{Name: "p2"},
		},
	}
	s.Equal(node.ExclusivePools[1], node.GetExclusivePool("p1"))
	s.Nil(node.GetExclusivePool("non existent"))
}

func (s *nodeTestsSuite) TestGetSharedPool() {
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
	s.Equal(sharedPoolName, sharedPool.Name)
	s.ElementsMatch(cores[2:], sharedPool.Cores)
	s.Equal(node.SharedPool.(*poolImpl).PowerProfile, sharedPool.PowerProfile)
}

func (s *nodeTestsSuite) TestDeleteProfile() {
	var sharedCores []Core
	for i := 0; i < 4; i++ {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("updateFreqValues", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		core.On("getReserved").Return(false)
		core.On("setReserved", true).Return()
		core.On("restoreFrequencies").Return(nil)
		core.On("setPool", mock.Anything).Return()
		sharedCores = append(sharedCores, core)
	}
	sharedCoresCopy := make([]Core, len(sharedCores))
	copy(sharedCoresCopy, sharedCores)

	var p1cores []Core
	for i := 4; i < 8; i++ {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("updateFreqValues", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		core.On("getReserved").Return(false)
		core.On("setReserved", true).Return()
		core.On("restoreFrequencies").Return(nil)
		core.On("setPool", mock.Anything).Return()
		p1cores = append(p1cores, core)
	}
	p1copy := make([]Core, len(p1cores))
	copy(p1copy, p1cores)

	var p2cores []Core
	for i := 8; i < 12; i++ {
		core := new(coreMock)
		core.On("GetID").Return(i)
		core.On("updateFreqValues", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		core.On("getReserved").Return(false)
		core.On("setPool", mock.Anything).Return()
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
	s.NoError(node.DeleteProfile("profile1"))
	s.Len(node.ExclusivePools, 1)
	s.Equal("profile2", node.ExclusivePools[0].(*poolImpl).PowerProfile.(*profileImpl).Name)
	s.ElementsMatch(node.ExclusivePools[0].(*poolImpl).Cores, p2copy)
	s.ElementsMatch(node.SharedPool.(*poolImpl).Cores, append(sharedCoresCopy, p1copy...))

	s.NoError(node.DeleteProfile(sharedPoolName))
	s.Nil(node.SharedPool.(*poolImpl).PowerProfile)
	for _, core := range p1copy {
		core.(*coreMock).AssertCalled(s.T(), "setReserved", true)
	}

	s.Error(node.DeleteProfile("not existing"))
}

func (s *nodeTestsSuite) TestAvailableCStates() {
	cStatesNamesMap = map[string]int{
		"C1": 1,
		"C2": 2,
	}

	node := &nodeImpl{}

	ret, err := node.AvailableCStates()

	s.ElementsMatch(ret, []string{"C1", "C2"})
	s.NoError(err)

	supportedFeatureErrors[CStatesFeature] = errors.New("err")

	ret, err = node.AvailableCStates()

	s.Nil(ret)
	s.ErrorIs(err, supportedFeatureErrors[CStatesFeature])

	cStatesNamesMap = map[string]int{}
}

func (s nodeTestsSuite) TestApplyCStatesToCore() {
	mockedCore := new(coreMock)
	mockedCore.On("applyCStates", mock.Anything).Return(nil)
	mockedCore.On("setReserved", true)
	node := &nodeImpl{
		allCores: []Core{mockedCore},
	}
	s.Nil(node.ApplyCStatesToCore(0, map[string]bool{}))

	mockedCore.AssertExpectations(s.T())
}

func (s *nodeTestsSuite) TestIsCStateValid() {
	cStatesNamesMap = map[string]int{
		"C1": 1,
		"C2": 2,
	}
	node := &nodeImpl{}

	s.True(node.IsCStateValid("C1"))
	s.True(node.IsCStateValid("C1", "C2"))

	s.False(node.IsCStateValid("C3"))
	s.False(node.IsCStateValid("C1", "C3"))

	cStatesNamesMap = map[string]int{}
}
