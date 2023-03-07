package power

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type hostMock struct {
	mock.Mock
}

func (m *hostMock) ValidateCStates(states CStates) error {
	return m.Called(states).Error(0)
}

func (m *hostMock) AvailableCStates() []string {
	return m.Called().Get(0).([]string)
}

func (m *hostMock) GetAllExclusivePools() *PoolList {
	return m.Called().Get(0).(*PoolList)
}

func (m *hostMock) SetName(name string) {
	m.Called(name)
}

func (m *hostMock) GetName() string {
	return m.Called().String(0)
}

func (m *hostMock) GetFeaturesInfo() FeatureSet {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(FeatureSet)
	}
}

func (m *hostMock) GetReservedPool() Pool {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(Pool)
	}
}

func (m *hostMock) GetSharedPool() Pool {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(Pool)
	}
}

func (m *hostMock) AddExclusivePool(poolName string) (Pool, error) {
	args := m.Called(poolName)
	retPool := args.Get(0)
	if retPool == nil {
		return nil, args.Error(1)
	} else {
		return retPool.(Pool), args.Error(1)
	}
}

func (m *hostMock) GetExclusivePool(poolName string) Pool {
	ret := m.Called(poolName).Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(Pool)
	}
}

func (m *hostMock) GetAllCores() *CoreList {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(*CoreList)
	}
}

func TestHost_initHost(t *testing.T) {
	origGetAllCores := getAllCores
	defer func() { getAllCores = origGetAllCores }()
	const hostName = "host"

	// get cores fail
	getAllCores = func() (CoreList, error) { return CoreList{}, fmt.Errorf("error") }
	host, err := initHost(hostName)
	assert.Nil(t, host)
	assert.Error(t, err)

	core1 := new(coreMock)
	core1.On("_setPoolProperty", mock.Anything).Return()
	core2 := new(coreMock)
	core2.On("_setPoolProperty", mock.Anything).Return()

	mockedCores := CoreList{core1, core2}
	getAllCores = func() (CoreList, error) { return mockedCores, nil }
	host, err = initHost(hostName)

	assert.NoError(t, err)

	core1.AssertExpectations(t)
	core2.AssertExpectations(t)

	hostObj := host.(*hostImpl)
	assert.Equal(t, hostObj.name, hostName)
	assert.ElementsMatch(t, hostObj.allCores, mockedCores)
	assert.ElementsMatch(t, hostObj.reservedPool.(*reservedPoolType).cores, mockedCores)
	assert.NotNil(t, hostObj.sharedPool)
}

func TestHostImpl_AddExclusivePool(t *testing.T) {
	// happy path
	poolName := "poolName"
	host := &hostImpl{}

	pool, err := host.AddExclusivePool(poolName)
	assert.Nil(t, err)

	poolObj := pool.(*exclusivePoolType)
	assert.Contains(t, host.exclusivePools, pool)
	assert.Equal(t, poolObj.name, poolName)
	assert.Equal(t, poolObj.host, host)
	assert.Empty(t, poolObj.cores)

	// already exists
	returnedPool, err := host.AddExclusivePool(poolName)
	assert.Equal(t, pool, returnedPool)
	assert.Error(t, err)
}

type hostTestsSuite struct {
	suite.Suite
}

func TestHost(t *testing.T) {
	suite.Run(t, new(hostTestsSuite))
}
func (s *hostTestsSuite) TestRemoveExclusivePool() {
	// happy path
	p1 := new(poolMock)
	p1.On("Name").Return("pool1")
	p1.On("Remove").Return(nil)

	p2 := new(poolMock)
	p2.On("name").Return("pool2")
	p2.On("Remove").Return(nil)
	host := &hostImpl{
		exclusivePools: []Pool{p1, p2},
	}
	s.NoError(host.GetAllExclusivePools().remove(p1))
	s.Assert().NotContains(host.exclusivePools, p1)
	s.Assert().Contains(host.exclusivePools, p2)

	// not existing
	p3 := new(poolMock)
	p3.On("Name").Return("pool3")
	p3.On("Remove").Return(nil)
	s.Error(new(hostImpl).GetAllExclusivePools().remove(p3))
}

func (s *hostTestsSuite) TestHostImpl_SetReservedPoolCores() {
	cores := make(CoreList, 4)
	host := &hostImpl{}
	for i := range cores {
		core, err := newCore(uint(i))
		s.Nil(err)

		cores[i] = core
	}
	host.allCores = cores
	host.reservedPool = &reservedPoolType{poolImpl{host: host, cores: make(CoreList, 0)}}
	host.sharedPool = &sharedPoolType{poolImpl{PowerProfile: &profileImpl{}, host: host, cores: cores}}

	for _, core := range host.allCores {
		core._setPoolProperty(host.sharedPool)
	}
	referenceCores := make(CoreList, 4)
	copy(referenceCores, cores)
	s.Nil(host.GetReservedPool().SetCores(referenceCores))
	s.ElementsMatch(host.GetReservedPool().Cores().IDs(), referenceCores.IDs())
	s.Len(host.GetSharedPool().Cores().IDs(), 0)

}

func (s *hostTestsSuite) TestAddSharedPool() {
	cores := make(CoreList, 4)
	host := &hostImpl{}
	host.sharedPool = &sharedPoolType{poolImpl{PowerProfile: &profileImpl{}, host: host}}
	for i := range cores {
		core, err := newCore(uint(i))
		s.Nil(err)

		cores[i] = core
	}
	host.allCores = cores
	host.reservedPool = &reservedPoolType{poolImpl{host: host, cores: cores}}
	for _, core := range host.allCores {
		core._setPoolProperty(host.reservedPool)
	}

	referenceCores := make(CoreList, 2)
	copy(referenceCores, cores[0:2])
	s.Nil(host.GetSharedPool().SetCores(referenceCores))

	//cores[1].(*coreMock).AssertNotCalled(s.T(), "setPStatesValues", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	//cores[2].(*coreMock).AssertCalled(s.T(), "setPStatesValues", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	s.ElementsMatch(host.sharedPool.Cores().IDs(), referenceCores.IDs())
}

func (s *hostTestsSuite) TestRemoveCoreFromExclusivePool() {
	pool := &poolImpl{
		name:         "test",
		PowerProfile: &profileImpl{},
	}
	cores := make(CoreList, 4)
	for i := range cores {
		core, err := newCore(uint(i))
		s.Nil(err)

		cores[i] = core
	}
	pool.cores = cores

	host := &hostImpl{
		name:           "test_host",
		exclusivePools: []Pool{pool},

		allCores: cores,
	}
	pool.host = host
	for _, core := range host.allCores {
		core._setPoolProperty(host.exclusivePools[0])
	}

	host.sharedPool = &poolImpl{PowerProfile: &profileImpl{}, host: host}

	coresToRemove := make(CoreList, 2)
	copy(coresToRemove, cores[0:2])
	coresToPreserve := make(CoreList, 2)
	copy(coresToPreserve, cores[2:])
	s.Nil(host.GetSharedPool().MoveCores(coresToRemove))

	s.ElementsMatch(host.GetExclusivePool("test").Cores().IDs(), coresToPreserve.IDs())
	s.ElementsMatch(host.GetSharedPool().Cores().IDs(), coresToRemove.IDs())

}

func (s *hostTestsSuite) TestAddCoresToExclusivePool() {
	host := &hostImpl{}
	host.exclusivePools = []Pool{&poolImpl{
		name:         "test",
		cores:        make([]Core, 0),
		PowerProfile: &profileImpl{},
		host:         host,
	}}
	host.name = "test_node"
	cores := make(CoreList, 4)
	for i := range cores {
		core, err := newCore(uint(i))
		s.Nil(err)

		cores[i] = core
	}
	host.sharedPool = &sharedPoolType{poolImpl{PowerProfile: &profileImpl{}, host: host, cores: cores}}
	for _, core := range cores {
		core._setPoolProperty(host.sharedPool)
	}
	host.allCores = cores
	var movedCoresIds []uint
	for _, core := range cores[:2] {
		movedCoresIds = append(movedCoresIds, core.GetID())
	}
	s.Nil(host.GetExclusivePool("test").MoveCoresIDs(movedCoresIds))
	unmoved := cores[2:]
	s.ElementsMatch(host.GetSharedPool().Cores().IDs(), unmoved.IDs())
	s.Len(host.GetExclusivePool("test").Cores().IDs(), 2)
	// for _, core := range host.exclusivePools[0].(*poolImpl).cores {
	// 	core.(*coreMock).AssertCalled(s.T(), "setPStatesValues", "", "", 0, 0)
	// }

}

// //
func (s *hostTestsSuite) TestUpdateProfile() {
	//pool := new(poolMock)
	profile := &profileImpl{name: "powah", min: 2500, max: 3200}
	//pool.On("GetPowerProfile").Return(profile)
	//pool.On("SetPowerProfile", mock.Anything).Return(nil)
	//pool.On("Name").Return("powah")
	host := hostImpl{
		sharedPool:    new(poolMock),
		featureStates: &FeatureSet{PStatesFeature: &featureStatus{err: nil}},
	}
	origFeatureList := featureList
	featureList = map[featureID]*featureStatus{
		PStatesFeature: {
			err:      nil,
			initFunc: initPStates,
		},
		CStatesFeature: {
			err:      nil,
			initFunc: initCStates,
		},
	}
	defer func() { featureList = origFeatureList }()
	pool := &poolImpl{name: "ex", PowerProfile: profile, host: &host}
	host.exclusivePools = []Pool{pool}
	s.Equal(host.GetExclusivePool("ex").GetPowerProfile().MinFreq(), 2500)
	s.Equal(host.GetExclusivePool("ex").GetPowerProfile().MaxFreq(), 3200)

	s.Nil(host.GetExclusivePool("ex").SetPowerProfile(&profileImpl{name: "powah", min: 1200, max: 2500}))

	s.Equal(host.GetExclusivePool("ex").GetPowerProfile().MinFreq(), 1200)
	s.Equal(host.GetExclusivePool("ex").GetPowerProfile().MaxFreq(), 2500)
}

func (s *hostTestsSuite) TestRemoveCoresFromSharedPool() {

	host := &hostImpl{}
	host.exclusivePools = []Pool{&poolImpl{
		name:         "test",
		cores:        make([]Core, 0),
		PowerProfile: &profileImpl{},
		host:         host,
	}}
	host.name = "test_node"
	cores := make(CoreList, 4)
	for i := range cores {
		core, err := newCore(uint(i))
		s.Nil(err)

		cores[i] = core
	}
	host.sharedPool = &sharedPoolType{poolImpl{PowerProfile: &profileImpl{}, host: host, cores: cores}}
	host.reservedPool = &reservedPoolType{poolImpl{host: host, cores: make([]Core, 0)}}

	for _, core := range cores {
		core._setPoolProperty(host.sharedPool)
	}
	host.allCores = cores
	coresCopy := make(CoreList, 4)
	copy(coresCopy, cores)
	s.Nil(host.GetReservedPool().MoveCores(coresCopy))
	s.ElementsMatch(host.GetReservedPool().Cores().IDs(), coresCopy.IDs())
	s.Len(host.GetSharedPool().Cores().IDs(), 0)
}

func (s *hostTestsSuite) TestGetExclusivePool() {
	node := &hostImpl{
		exclusivePools: []Pool{
			&poolImpl{name: "p0"},
			&poolImpl{name: "p1"},
			&poolImpl{name: "p2"},
		},
	}
	s.Equal(node.exclusivePools[1], node.GetExclusivePool("p1"))
	s.Nil(node.GetExclusivePool("non existent"))
}
func (s *hostTestsSuite) TestGetSharedPool() {
	cores := make(CoreList, 4)
	for i := range cores {
		core, err := newCore(uint(i))
		s.Nil(err)

		cores[i] = core
	}

	node := &hostImpl{
		sharedPool: &sharedPoolType{poolImpl{
			name:         sharedPoolName,
			cores:        cores,
			PowerProfile: &profileImpl{},
		}},
	}
	sharedPool := node.GetSharedPool().(*sharedPoolType)
	s.ElementsMatch(cores.IDs(), sharedPool.cores.IDs())
	s.Equal(node.sharedPool.(*sharedPoolType).PowerProfile, sharedPool.PowerProfile)
}
func (s *hostTestsSuite) TestGetReservedPool() {
	cores := make(CoreList, 4)
	for i := range cores {
		core, err := newCore(uint(i))
		s.Nil(err)
		cores[i] = core
	}
	poolImp := &poolImpl{
		name:         reservedPoolName,
		cores:        cores,
		PowerProfile: &profileImpl{},
	}
	node := &hostImpl{
		reservedPool: poolImp,
	}
	reservedPool := node.GetReservedPool()
	s.ElementsMatch(cores.IDs(), reservedPool.Cores().IDs())
	s.Equal(reservedPool.GetPowerProfile(), poolImp.PowerProfile)
}
func (s *hostTestsSuite) TestDeleteProfile() {
	allCores := make(CoreList, 12)
	sharedCores := make(CoreList, 4)
	for i := 0; i < 4; i++ {
		core, err := newCore(uint(i))
		s.Nil(err)
		allCores[i] = core
		sharedCores[i] = core
	}
	sharedCoresCopy := make(CoreList, len(sharedCores))
	copy(sharedCoresCopy, sharedCores)

	p1cores := make(CoreList, 4)
	for i := 4; i < 8; i++ {
		core, err := newCore(uint(i))
		s.Nil(err)
		allCores[i] = core
		p1cores[i-4] = core
	}
	p1copy := make([]Core, len(p1cores))
	copy(p1copy, p1cores)

	p2cores := make(CoreList, 4)
	for i := 8; i < 12; i++ {
		core, err := newCore(uint(i))
		s.Nil(err)
		allCores[i] = core
		p2cores[i-8] = core
	}
	p2copy := make(CoreList, len(p2cores))
	copy(p2copy, p2cores)

	host := &hostImpl{}
	exclusive := []Pool{
		&exclusivePoolType{poolImpl{
			name:         "pool1",
			cores:        p1cores,
			PowerProfile: &profileImpl{name: "profile1"},
			host:         host,
		}},
		&exclusivePoolType{poolImpl{
			name:         "pool2",
			cores:        p2cores,
			PowerProfile: &profileImpl{name: "profile2"},
			host:         host,
		}},
	}
	shared := &sharedPoolType{poolImpl{
		name:         sharedPoolName,
		cores:        sharedCores,
		PowerProfile: &profileImpl{name: sharedPoolName},
		host:         host,
	}}
	host.exclusivePools = exclusive
	host.sharedPool = shared
	host.reservedPool = &reservedPoolType{poolImpl{host: host}}
	host.allCores = allCores
	for i := 0; i < 4; i++ {
		sharedCores[i]._setPoolProperty(host.sharedPool)
		p1cores[i]._setPoolProperty(host.exclusivePools[0])
		p2cores[i]._setPoolProperty(host.exclusivePools[1])
	}
	s.NoError(host.GetExclusivePool("pool1").Remove())
	s.Len(host.exclusivePools, 1)
	s.Equal("profile2", host.exclusivePools[0].(*exclusivePoolType).PowerProfile.(*profileImpl).name)
	s.ElementsMatch(host.exclusivePools[0].(*exclusivePoolType).cores, p2copy)
	newShared := append(sharedCoresCopy, p1copy...)
	s.ElementsMatch(host.GetSharedPool().Cores().IDs(), newShared.IDs())

}
