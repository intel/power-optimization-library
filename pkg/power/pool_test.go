package power

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type poolMock struct {
	mock.Mock
}

func (m *poolMock) poolMutex() sync.Locker {
	return m.Called().Get(0).(sync.Locker)
}

func (m *poolMock) SetCStates(states CStates) error {
	return m.Called(states).Error(0)
}

func (m *poolMock) getCStates() *CStates {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	}
	return args.(*CStates)
}

func (m *poolMock) isExclusive() bool {
	return m.Called().Bool(0)
}

func (m *poolMock) Clear() error {
	return m.Called().Error(0)
}

func (m *poolMock) Name() string {
	return m.Called().String(0)
}

func (m *poolMock) Cpus() *CpuList {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	}
	return args.(*CpuList)
}

func (m *poolMock) SetCpus(cores CpuList) error {
	return m.Called(cores).Error(0)
}

func (m *poolMock) SetCpuIDs(cpuIDs []uint) error {
	return m.Called(cpuIDs).Error(0)
}

func (m *poolMock) Remove() error {
	return m.Called().Error(0)
}

func (m *poolMock) MoveCpuIDs(coreIDs []uint) error {
	return m.Called(coreIDs).Error(0)
}

func (m *poolMock) MoveCpus(cores CpuList) error {
	return m.Called(cores).Error(0)
}

func (m *poolMock) getHost() Host {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	}
	return args.(Host)
}

func (m *poolMock) SetPowerProfile(profile Profile) error {
	args := m.Called(profile)
	return args.Error(0)
}

func (m *poolMock) GetPowerProfile() Profile {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	}
	return args.(Profile)
}

func TestPoolList(t *testing.T) {
	p1 := new(poolMock)
	p1.On("Name").Return("pool1")
	p2 := new(poolMock)
	p2.On("Name").Return("pool2")

	var pools PoolList = []Pool{p1, p2}
	// IndexOf
	assert.Equal(t, 1, pools.IndexOf(p2))
	assert.Equal(t, -1, pools.IndexOf(&poolImpl{}))
	// IndexOfName
	assert.Equal(t, 1, pools.IndexOfName("pool2"))
	assert.Equal(t, -1, pools.IndexOfName("not existing"))
	// Contains
	assert.True(t, pools.Contains(p1))
	assert.False(t, pools.Contains(&exclusivePoolType{}))
	// add
	newPool := &poolImpl{
		name: "new",
	}
	pools.add(newPool)
	assert.Contains(t, pools, newPool)
	// remove
	assert.NoError(t, pools.remove(p2))
	assert.NotContains(t, pools, p2)
	assert.Error(t, pools.remove(new(poolImpl)))
	// get by name
	assert.Equal(t, newPool, pools.ByName("new"))
	assert.Nil(t, pools.ByName("not exising"))
}
func TestPoolImpl_MoveCoresIDs(t *testing.T) {
	assert.PanicsWithValue(t, "virtual", func() {
		pool := poolImpl{
			host: &hostImpl{},
		}
		_ = pool.MoveCpuIDs([]uint{})
	})
}

func TestPoolImpl_MoveCores(t *testing.T) {
	assert.PanicsWithValue(t, "virtual", func() {
		pool := poolImpl{}
		_ = pool.MoveCpus(CpuList{})
	})
}
func TestExclusivePoolType_MoveCpuIDs(t *testing.T) {
	host := new(hostMock)
	host.On("GetAllCpus").Return(new(CpuList))
	pool := &exclusivePoolType{poolImpl{
		host: host,
	},
	}
	assert.NoError(t, pool.MoveCpuIDs([]uint{}))
	assert.ErrorContains(t, pool.MoveCpuIDs([]uint{2}), "not in list")
}

func TestExclusivePoolType_MoveCpus(t *testing.T) {
	// happy path
	mockCore := new(cpuMock)
	mockCore2 := new(cpuMock)
	p := new(exclusivePoolType)
	mockCore.On("SetPool", p).Return(nil)
	mockCore2.On("SetPool", p).Return(nil)

	assert.NoError(t, p.MoveCpus(CpuList{mockCore, mockCore2}))

	mockCore.AssertExpectations(t)
	mockCore2.AssertExpectations(t)

	//failed to set
	setPoolErr := fmt.Errorf("")
	mockCore = new(cpuMock)
	mockCore.On("SetPool", p).Return(setPoolErr)

	assert.ErrorIs(t, p.MoveCpus(CpuList{mockCore}), setPoolErr)
	mockCore.AssertExpectations(t)
}
func TestSharedPoolType_MoveCpuIDs(t *testing.T) {
	host := new(hostMock)
	host.On("GetAllCpus").Return(new(CpuList))
	pool := &sharedPoolType{poolImpl{
		host: host,
	},
	}
	assert.NoError(t, pool.MoveCpuIDs([]uint{}))
	assert.ErrorContains(t, pool.MoveCpuIDs([]uint{2}), "not in list")
}

func TestSharedPoolType_MoveCpus(t *testing.T) {
	// happy path
	mockCore := new(cpuMock)
	mockCore2 := new(cpuMock)
	p := new(sharedPoolType)
	mockCore.On("SetPool", p).Return(nil)
	mockCore2.On("SetPool", p).Return(nil)

	assert.NoError(t, p.MoveCpus(CpuList{mockCore, mockCore2}))

	mockCore.AssertExpectations(t)
	mockCore2.AssertExpectations(t)

	//failed to set
	setPoolErr := fmt.Errorf("")
	mockCore = new(cpuMock)
	mockCore.On("SetPool", p).Return(setPoolErr)

	assert.ErrorIs(t, p.MoveCpus(CpuList{mockCore}), setPoolErr)
	mockCore.AssertExpectations(t)
}
func TestReservedPoolType_MoveCpuIDs(t *testing.T) {
	host := new(hostMock)
	host.On("GetAllCpus").Return(new(CpuList))
	pool := &reservedPoolType{poolImpl{
		host: host,
	},
	}
	assert.NoError(t, pool.MoveCpuIDs([]uint{}))
	assert.ErrorContains(t, pool.MoveCpuIDs([]uint{2}), "not in list")
}

func TestReservedPoolType_MoveCpus(t *testing.T) {
	// happy path
	mockCore := new(cpuMock)
	mockCore2 := new(cpuMock)
	p := new(reservedPoolType)
	mockCore.On("SetPool", p).Return(nil)
	mockCore2.On("SetPool", p).Return(nil)

	assert.NoError(t, p.MoveCpus(CpuList{mockCore, mockCore2}))

	mockCore.AssertExpectations(t)
	mockCore2.AssertExpectations(t)

	//failed to set
	setPoolErr := fmt.Errorf("")
	mockCore = new(cpuMock)
	mockCore.On("SetPool", p).Return(setPoolErr)

	assert.ErrorIs(t, p.MoveCpus(CpuList{mockCore}), setPoolErr)
	mockCore.AssertExpectations(t)
}

func TestPoolImpl_Getters(t *testing.T) {
	name := "pool"
	cores := CpuList{}
	powerProfile := new(profileImpl)
	host := new(hostMock)
	pool := poolImpl{
		name:         name,
		cpus:         cores,
		PowerProfile: powerProfile,
		host:         host,
	}
	assert.Equal(t, name, pool.Name())
	assert.Equal(t, &cores, pool.Cpus())
	assert.Equal(t, powerProfile, pool.GetPowerProfile())
}

func TestPoolImpl_SetCoreIDs(t *testing.T) {
	assert.Panics(t, func() {
		pool := poolImpl{}
		_ = pool.SetCpuIDs([]uint{})
	})
}
func TestSharedPoolType_SetCoreIDs(t *testing.T) {
	host := new(hostMock)
	host.On("GetAllCpus").Return(new(CpuList))

	pool := &sharedPoolType{poolImpl{host: host}}
	assert.NoError(t, pool.SetCpuIDs([]uint{}))
}
func TestReservedPoolType_SetCoreIDs(t *testing.T) {
	host := new(hostMock)
	host.On("GetAllCpus").Return(new(CpuList))
	host.On("GetSharedPool").Return(new(poolMock))

	pool := &reservedPoolType{poolImpl{host: host}}
	assert.NoError(t, pool.SetCpuIDs([]uint{}))
}

func TestExclusivePoolType_SetCoreIDs(t *testing.T) {
	host := new(hostMock)
	host.On("GetAllCpus").Return(new(CpuList))

	pool := &exclusivePoolType{poolImpl{host: host}}
	assert.NoError(t, pool.SetCpuIDs([]uint{}))
}

func TestPoolImpl_SetCores(t *testing.T) {
	// base struct pool should always panic
	basePool := &poolImpl{}
	assert.Panics(t, func() {
		_ = basePool.SetCpus(CpuList{})
	})
}

func TestSharedPoolType_SetCores(t *testing.T) {
	reservedPool := new(poolMock)
	host := new(hostMock)

	sharedPool := &sharedPoolType{poolImpl{
		host: host,
	}}

	allCores := make(CpuList, 8)
	for i := range allCores {
		core := new(cpuMock)
		if i >= 2 && i < 5 {
			core.On("SetPool", sharedPool).Return(nil)
		} else {
			core.On("SetPool", reservedPool).Return(nil)
			core.On("getPool").Return(sharedPool)
		}
		allCores[i] = core
	}

	host.On("GetAllCpus").Return(&allCores)
	host.On("GetReservedPool").Return(reservedPool)

	assert.NoError(t, sharedPool.SetCpus(allCores[2:5]))
	for _, core := range allCores {
		core.(*cpuMock).AssertExpectations(t)
	}
	// setPool error
	err := fmt.Errorf("borked")
	allCores[0] = new(cpuMock)
	allCores[0].(*cpuMock).On("SetPool", mock.Anything).Return(err)
	assert.ErrorIs(t, sharedPool.SetCpus(allCores), err)

}

func TestReservedPoolType_SetCores(t *testing.T) {
	sharedPool := new(poolMock)
	sharedPool.On("isExclusive").Return(false)

	exclusivePool := new(poolMock)
	exclusivePool.On("isExclusive").Return(true)

	host := new(hostMock)
	allCores := CpuList{}
	host.On("GetAllCpus").Return(&allCores)
	host.On("GetSharedPool").Return(sharedPool)

	requestedSetCores := CpuList{}
	reservedPool := &reservedPoolType{poolImpl{host: host}}
	for i := 1; i <= 6; i++ {
		core := new(cpuMock)
		switch i {
		case 1:
			core.On("getPool").Return(exclusivePool)
		case 2:
			core.On("getPool").Return(nil)
			// will test when testing errors, we don't want to return prematurely
		case 3:
			core.On("getPool").Return(sharedPool)
		case 4:
			core.On("getPool").Return(sharedPool)
			requestedSetCores.add(core)
			core.On("SetPool", reservedPool).Return(nil)
		case 5:
			core.On("getPool").Return(reservedPool)
			core.On("SetPool", sharedPool).Return(nil)
		case 6:
			core.On("getPool").Return(reservedPool)
			requestedSetCores.add(core)
			core.On("SetPool", reservedPool).Return(nil)
		}
		allCores.add(core)
	}

	assert.NoError(t, reservedPool.SetCpus(requestedSetCores))
	for _, core := range allCores {
		core.(*cpuMock).AssertExpectations(t)
	}
	// now test case 2 where we expect error
	allCores[0] = new(cpuMock)
	allCores[0].(*cpuMock).On("getPool").Return(exclusivePool)

	assert.ErrorContains(t, reservedPool.SetCpus(CpuList{allCores[0]}), "exclusive to reserved")
}

func TestExclusivePoolType_SetCores(t *testing.T) {
	sharedPool := new(poolMock)
	host := new(hostMock)

	exclusivePool := &exclusivePoolType{poolImpl{
		host: host,
	}}

	allCores := make(CpuList, 3)
	for i := range allCores {
		core := new(cpuMock)
		switch i {
		case 0:
			core.On("getPool").Return(exclusivePool)
			core.On("SetPool", sharedPool).Return(nil)
		case 1:
			core.On("getPool").Return(sharedPool)
		case 2:
			core.On("SetPool", exclusivePool).Return(nil)
		}

		allCores[i] = core
	}

	host.On("GetAllCpus").Return(&allCores)
	host.On("GetSharedPool").Return(sharedPool)
	// exclusive pool
	assert.NoError(t, exclusivePool.SetCpus(CpuList{allCores[2]}))
	for _, core := range allCores {
		core.(*cpuMock).AssertExpectations(t)
	}
	// setPool error
	err := fmt.Errorf("borked")
	allCores[0] = new(cpuMock)
	allCores[0].(*cpuMock).On("SetPool", mock.Anything).Return(err)
	assert.ErrorIs(t, exclusivePool.SetCpus(CpuList{allCores[0]}), err)
}

func TestPoolImpl_SetPowerProfile(t *testing.T) {
	cores := make(CpuList, 2)
	for i := range cores {
		core := new(cpuMock)
		core.On("consolidate").Return(nil)
		cores[i] = core
	}

	poolMutex := new(mutexMock)
	poolMutex.On("Unlock").Return().NotBefore(
		poolMutex.On("Lock").Return(),
	)
	pool := &poolImpl{cpus: cores, mutex: poolMutex}
	powerProfile := new(profileImpl)
	assert.NoError(t, pool.SetPowerProfile(powerProfile))
	assert.True(t, pool.PowerProfile == powerProfile)
	poolMutex.AssertExpectations(t)
	for _, core := range cores {
		core.(*cpuMock).AssertExpectations(t)
	}
}

func TestPoolImpl_Remove(t *testing.T) {
	// expecting to call the 'virtual' SetCpus that panics
	assert.Panics(t, func() {
		pool := poolImpl{}
		pool.Remove()
	})

}

func TestExclusivePoolType_Remove(t *testing.T) {
	host := new(hostMock)
	host.On("GetAllCpus").Return(new(CpuList))

	pool := &exclusivePoolType{poolImpl{host: host}}
	pools := PoolList{pool}
	host.On("GetAllExclusivePools").Return(&pools)
	assert.NoError(t, pool.Remove())
	host.AssertExpectations(t)
	assert.NotContains(t, pools, pool)
}

func TestPoolList_ByName(t *testing.T) {
	pools := PoolList{}
	for _, s := range []string{"pool1", "poo2", "something"} {
		mockPool := new(poolMock)
		mockPool.On("Name").Return(s)
		pools = append(pools, mockPool)
	}
	assert.Equal(t, pools[2], pools.ByName("something"))
	assert.Nil(t, pools.ByName("not existing"))

}
