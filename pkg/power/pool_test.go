package power

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type poolMock struct {
	mock.Mock
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

func (m *poolMock) Cores() *CoreList {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	}
	return args.(*CoreList)
}

func (m *poolMock) SetCores(cores CoreList) error {
	return m.Called(cores).Error(0)
}

func (m *poolMock) SetCoreIDs(coreIDs []uint) error {
	return m.Called(coreIDs).Error(0)
}

func (m *poolMock) Remove() error {
	return m.Called().Error(0)
}

func (m *poolMock) MoveCoresIDs(coreIDs []uint) error {
	return m.Called(coreIDs).Error(0)
}

func (m *poolMock) MoveCores(cores CoreList) error {
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
	host := new(hostMock)
	host.On("GetAllCores").Return(new(CoreList))
	pool := &poolImpl{
		host: host,
	}
	assert.NoError(t, pool.MoveCoresIDs([]uint{}))
}
func TestPoolImpl_MoveCores(t *testing.T) {
	// happy path
	mockCore := new(coreMock)
	mockCore2 := new(coreMock)
	p := new(poolImpl)
	mockCore.On("SetPool", p).Return(nil)
	mockCore2.On("SetPool", p).Return(nil)

	assert.NoError(t, p.MoveCores(CoreList{mockCore, mockCore2}))

	mockCore.AssertExpectations(t)
	mockCore2.AssertExpectations(t)
}
func TestPoolImpl_Getters(t *testing.T) {
	name := "pool"
	cores := CoreList{}
	powerProfile := new(profileImpl)
	host := new(hostMock)
	pool := poolImpl{
		name:         name,
		cores:        cores,
		PowerProfile: powerProfile,
		host:         host,
	}
	assert.Equal(t, name, pool.Name())
	assert.Equal(t, &cores, pool.Cores())
	assert.Equal(t, powerProfile, pool.GetPowerProfile())
}

func TestPoolImpl_SetCoreIDs(t *testing.T) {
	assert.Panics(t, func() {
		pool := poolImpl{}
		_ = pool.SetCoreIDs([]uint{})
	})
}
func TestSharedPoolType_SetCoreIDs(t *testing.T) {
	host := new(hostMock)
	host.On("GetAllCores").Return(new(CoreList))

	pool := &sharedPoolType{poolImpl{host: host}}
	assert.NoError(t, pool.SetCoreIDs([]uint{}))
}
func TestReservedPoolType_SetCoreIDs(t *testing.T) {
	host := new(hostMock)
	host.On("GetAllCores").Return(new(CoreList))
	host.On("GetSharedPool").Return(new(poolMock))

	pool := &reservedPoolType{poolImpl{host: host}}
	assert.NoError(t, pool.SetCoreIDs([]uint{}))
}

func TestExclusivePoolType_SetCoreIDs(t *testing.T) {
	host := new(hostMock)
	host.On("GetAllCores").Return(new(CoreList))

	pool := &exclusivePoolType{poolImpl{host: host}}
	assert.NoError(t, pool.SetCoreIDs([]uint{}))
}

func TestPoolImpl_SetCores(t *testing.T) {
	// base struct pool should always panic
	basePool := &poolImpl{}
	assert.Panics(t, func() {
		_ = basePool.SetCores(CoreList{})
	})
}

func TestSharedPoolType_SetCores(t *testing.T) {
	reservedPool := new(poolMock)
	host := new(hostMock)

	sharedPool := &sharedPoolType{poolImpl{
		host: host,
	}}

	allCores := make(CoreList, 8)
	for i := range allCores {
		core := new(coreMock)
		if i >= 2 && i < 5 {
			core.On("SetPool", sharedPool).Return(nil)
		} else {
			core.On("SetPool", reservedPool).Return(nil)
			core.On("getPool").Return(sharedPool)
		}
		allCores[i] = core
	}

	host.On("GetAllCores").Return(&allCores)
	host.On("GetReservedPool").Return(reservedPool)

	assert.NoError(t, sharedPool.SetCores(allCores[2:5]))
	for _, core := range allCores {
		core.(*coreMock).AssertExpectations(t)
	}
	// setPool error
	err := fmt.Errorf("borked")
	allCores[0] = new(coreMock)
	allCores[0].(*coreMock).On("SetPool", mock.Anything).Return(err)
	assert.ErrorIs(t, sharedPool.SetCores(allCores), err)

}

func TestReservedPoolType_SetCores(t *testing.T) {
	sharedPool := new(poolMock)
	sharedPool.On("isExclusive").Return(false)

	exclusivePool := new(poolMock)
	exclusivePool.On("isExclusive").Return(true)

	host := new(hostMock)
	allCores := CoreList{}
	host.On("GetAllCores").Return(&allCores)
	host.On("GetSharedPool").Return(sharedPool)

	requestedSetCores := CoreList{}
	reservedPool := &reservedPoolType{poolImpl{host: host}}
	for i := 1; i <= 6; i++ {
		core := new(coreMock)
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

	assert.NoError(t, reservedPool.SetCores(requestedSetCores))
	for _, core := range allCores {
		core.(*coreMock).AssertExpectations(t)
	}
	// now test case 2 where we expect error
	allCores[0] = new(coreMock)
	allCores[0].(*coreMock).On("getPool").Return(exclusivePool)

	assert.ErrorContains(t, reservedPool.SetCores(CoreList{allCores[0]}), "exclusive to reserved")
}

func TestExclusivePoolType_SetCores(t *testing.T) {
	sharedPool := new(poolMock)
	host := new(hostMock)

	exclusivePool := &exclusivePoolType{poolImpl{
		host: host,
	}}

	allCores := make(CoreList, 3)
	for i := range allCores {
		core := new(coreMock)
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

	host.On("GetAllCores").Return(&allCores)
	host.On("GetSharedPool").Return(sharedPool)
	// exclusive pool
	assert.NoError(t, exclusivePool.SetCores(CoreList{allCores[2]}))
	for _, core := range allCores {
		core.(*coreMock).AssertExpectations(t)
	}
	// setPool error
	err := fmt.Errorf("borked")
	allCores[0] = new(coreMock)
	allCores[0].(*coreMock).On("SetPool", mock.Anything).Return(err)
	assert.ErrorIs(t, exclusivePool.SetCores(CoreList{allCores[0]}), err)
}

func TestPoolImpl_SetPowerProfile(t *testing.T) {
	cores := make(CoreList, 2)
	for i := range cores {
		core := new(coreMock)
		core.On("consolidate").Return(nil)
		cores[i] = core
	}

	pool := &poolImpl{cores: cores}
	powerProfile := new(profileImpl)
	assert.NoError(t, pool.SetPowerProfile(powerProfile))
	assert.True(t, pool.PowerProfile == powerProfile)
	for _, core := range cores {
		core.(*coreMock).AssertExpectations(t)
	}
}

func TestPoolImpl_Remove(t *testing.T) {
	// expecting to call the 'virtual' SetCores that panics
	assert.Panics(t, func() {
		pool := poolImpl{}
		pool.Remove()
	})

}

func TestExclusivePoolType_Remove(t *testing.T) {
	host := new(hostMock)
	host.On("GetAllCores").Return(new(CoreList))

	pool := &exclusivePoolType{poolImpl{host: host}}
	pools := PoolList{pool}
	host.On("GetAllExclusivePools").Return(&pools)
	assert.NoError(t, pool.Remove())
	host.AssertExpectations(t)
	assert.NotContains(t, pools, pool)
}

////
////func (s *poolTestSuite) TestSetCStates() {
////	states := CStates{"C2": true, "C1": false}
////	cores := make([]Core, 2)
////	for i := range cores {
////		core := new(coreMock)
////		core.On("applyCStates", states).Return(nil)
////		core.On("exclusiveCStates").Return(false)
////		cores[i] = core
////	}
////	supportedFeatureErrors[CStatesFeature] = nil
////
////	assert.NoError(s.T(), (&poolImpl{cores: cores}).SetCStates(states))
////
////	for _, core := range cores {
////		core.(*coreMock).AssertExpectations(s.T())
////	}
////
////	// fail to apply c states on one core
////	cores = make([]Core, 2)
////	for i := range cores {
////		core := new(coreMock)
////		core.On("exclusiveCStates").Return(false)
////		cores[i] = core
////	}
////	cores[0].(*coreMock).On("applyCStates", states).Return(errors.New("apply fail"))
////	cores[1].(*coreMock).On("applyCStates", states).Return(nil)
////
////	assert.Error(s.T(), (&poolImpl{cores: cores}).SetCStates(states))
////
////	for _, core := range cores {
////		core.(*coreMock).AssertExpectations(s.T())
////	}
////
////	cores = make([]Core, 2)
////	for i := range cores {
////		core := new(coreMock)
////		cores[i] = core
////	}
////
////	delete(cStatesNamesMap, "C2")
////	assert.Error(s.T(), (&poolImpl{cores: cores}).SetCStates(states))
////	for _, core := range cores {
////		core.(*coreMock).AssertNotCalled(s.T(), "applyCStates")
////	}
////
////	e := errors.New("")
////	supportedFeatureErrors[CStatesFeature] = &e
////	assert.ErrorIs(s.T(), (&poolImpl{}).SetCStates(states), *supportedFeatureErrors[CStatesFeature])
////}

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
