package power

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type hostMock struct {
	mock.Mock
}

func (m *hostMock) Topology() Topology {
	return m.Called().Get(0).(Topology)
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

func (m *hostMock) NumCoreTypes() uint {
	return m.Called().Get(0).(uint)
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

func (m *hostMock) GetAllCpus() *CpuList {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(*CpuList)
	}
}

func (m *hostMock) GetFreqRanges() CoreTypeList {
	return m.Called().Get(0).(CoreTypeList)
}

func TestHost_initHost(t *testing.T) {
	origGetAllCores := discoverTopology
	defer func() { discoverTopology = origGetAllCores }()
	const hostName = "host"

	// get topology fail
	discoverTopology = func() (Topology, error) { return new(mockCpuTopology), fmt.Errorf("error") }
	host, err := initHost(hostName)
	assert.Nil(t, host)
	assert.Error(t, err)

	core1 := new(cpuMock)
	core1.On("_setPoolProperty", mock.Anything).Return()
	core2 := new(cpuMock)
	core2.On("_setPoolProperty", mock.Anything).Return()

	mockedCores := CpuList{core1, core2}
	topObj := new(mockCpuTopology)
	topObj.On("CPUs").Return(&mockedCores)
	discoverTopology = func() (Topology, error) { return topObj, nil }
	host, err = initHost(hostName)

	assert.NoError(t, err)

	core1.AssertExpectations(t)
	core2.AssertExpectations(t)

	hostObj := host.(*hostImpl)
	assert.Equal(t, hostObj.name, hostName)
	assert.Equal(t, hostObj.topology, topObj)
	assert.ElementsMatch(t, hostObj.reservedPool.(*reservedPoolType).cpus, mockedCores)
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
	assert.Empty(t, poolObj.cpus)

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
	cores := make(CpuList, 4)
	topology := new(mockCpuTopology)
	host := &hostImpl{topology: topology}
	for i := range cores {
		m := new(mockCpuCore)
		core, err := newCpu(uint(i), m)
		s.Nil(err)

		cores[i] = core
	}
	topology.On("CPUs").Return(&cores)
	host.reservedPool = &reservedPoolType{poolImpl{host: host, mutex: &sync.Mutex{}, cpus: make(CpuList, 0)}}
	host.sharedPool = &sharedPoolType{poolImpl{PowerProfile: &profileImpl{}, mutex: &sync.Mutex{}, host: host, cpus: cores}}

	for _, core := range cores {
		core._setPoolProperty(host.sharedPool)
	}
	referenceCores := make(CpuList, 4)
	copy(referenceCores, cores)
	s.Nil(host.GetReservedPool().SetCpus(referenceCores))
	s.ElementsMatch(host.GetReservedPool().Cpus().IDs(), referenceCores.IDs())
	s.Len(host.GetSharedPool().Cpus().IDs(), 0)

}

func (s *hostTestsSuite) TestAddSharedPool() {
	cores := make(CpuList, 4)
	topology := new(mockCpuTopology)
	host := &hostImpl{topology: topology}
	host.sharedPool = &sharedPoolType{poolImpl{PowerProfile: &profileImpl{}, mutex: &sync.Mutex{}, host: host}}
	for i := range cores {
		m := new(mockCpuCore)
		core, err := newCpu(uint(i), m)
		s.Nil(err)

		cores[i] = core
	}
	topology.On("CPUs").Return(&cores)

	host.reservedPool = &reservedPoolType{poolImpl{host: host, mutex: &sync.Mutex{}, cpus: cores}}
	for _, core := range cores {
		core._setPoolProperty(host.reservedPool)
	}

	referenceCores := make(CpuList, 2)
	copy(referenceCores, cores[0:2])
	s.Nil(host.GetSharedPool().SetCpus(referenceCores))

	s.ElementsMatch(host.sharedPool.Cpus().IDs(), referenceCores.IDs())
}

func (s *hostTestsSuite) TestRemoveCoreFromExclusivePool() {
	pool := &poolImpl{
		name:         "test",
		PowerProfile: &profileImpl{},
		mutex:        &sync.Mutex{},
	}
	cores := make(CpuList, 4)
	for i := range cores {
		m := new(mockCpuCore)
		core, err := newCpu(uint(i), m)
		s.Nil(err)

		cores[i] = core
	}
	pool.cpus = cores

	topology := new(mockCpuTopology)
	//topology.On("CPUs").Return(cores)

	host := &hostImpl{
		name:           "test_host",
		exclusivePools: []Pool{pool},
		topology:       topology,
	}
	pool.host = host
	for _, core := range cores {
		core._setPoolProperty(host.exclusivePools[0])
	}

	host.sharedPool = &sharedPoolType{poolImpl{PowerProfile: &profileImpl{}, mutex: &sync.Mutex{}, host: host}}

	coresToRemove := make(CpuList, 2)
	copy(coresToRemove, cores[0:2])
	coresToPreserve := make(CpuList, 2)
	copy(coresToPreserve, cores[2:])
	s.Nil(host.GetSharedPool().MoveCpus(coresToRemove))

	s.ElementsMatch(host.GetExclusivePool("test").Cpus().IDs(), coresToPreserve.IDs())
	s.ElementsMatch(host.GetSharedPool().Cpus().IDs(), coresToRemove.IDs())

}

func (s *hostTestsSuite) TestAddCoresToExclusivePool() {
	topology := new(mockCpuTopology)
	host := &hostImpl{
		topology: topology,
	}
	host.exclusivePools = []Pool{&exclusivePoolType{poolImpl{
		name:         "test",
		cpus:         make([]Cpu, 0),
		mutex:        &sync.Mutex{},
		PowerProfile: &profileImpl{},
		host:         host,
	}}}
	host.name = "test_node"
	cores := make(CpuList, 4)
	for i := range cores {
		m := new(mockCpuCore)
		core, err := newCpu(uint(i), m)
		s.Nil(err)

		cores[i] = core
	}
	topology.On("CPUs").Return(&cores)
	host.sharedPool = &sharedPoolType{poolImpl{PowerProfile: &profileImpl{}, mutex: &sync.Mutex{}, host: host, cpus: cores}}
	for _, core := range cores {
		core._setPoolProperty(host.sharedPool)
	}

	var movedCoresIds []uint
	for _, core := range cores[:2] {
		movedCoresIds = append(movedCoresIds, core.GetID())
	}
	s.Nil(host.GetExclusivePool("test").MoveCpuIDs(movedCoresIds))
	unmoved := cores[2:]
	s.ElementsMatch(host.GetSharedPool().Cpus().IDs(), unmoved.IDs())
	s.Len(host.GetExclusivePool("test").Cpus().IDs(), 2)

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
		featureStates: &FeatureSet{FrequencyScalingFeature: &featureStatus{err: nil}},
	}
	origFeatureList := featureList
	featureList = map[featureID]*featureStatus{
		FrequencyScalingFeature: {
			err:      nil,
			initFunc: initScalingDriver,
		},
		CStatesFeature: {
			err:      nil,
			initFunc: initCStates,
		},
	}
	defer func() { featureList = origFeatureList }()
	pool := &poolImpl{name: "ex", mutex: &sync.Mutex{}, PowerProfile: profile, host: &host}
	host.exclusivePools = []Pool{pool}
	s.Equal(host.GetExclusivePool("ex").GetPowerProfile().MinFreq(), uint(2500))
	s.Equal(host.GetExclusivePool("ex").GetPowerProfile().MaxFreq(), uint(3200))

	s.Nil(host.GetExclusivePool("ex").SetPowerProfile(&profileImpl{name: "powah", min: 1200, max: 2500}))

	s.Equal(host.GetExclusivePool("ex").GetPowerProfile().MinFreq(), uint(1200))
	s.Equal(host.GetExclusivePool("ex").GetPowerProfile().MaxFreq(), uint(2500))
}

func (s *hostTestsSuite) TestRemoveCoresFromSharedPool() {
	topology := new(mockCpuTopology)
	host := &hostImpl{topology: topology}
	host.exclusivePools = []Pool{&poolImpl{
		name:         "test",
		cpus:         make([]Cpu, 0),
		mutex:        &sync.Mutex{},
		PowerProfile: &profileImpl{},
		host:         host,
	}}
	host.name = "test_node"
	cores := make(CpuList, 4)
	for i := range cores {
		m := new(mockCpuCore)
		core, err := newCpu(uint(i), m)
		s.Nil(err)

		cores[i] = core
	}
	//topology.On("CPUs").Return(cores)
	host.sharedPool = &sharedPoolType{poolImpl{PowerProfile: &profileImpl{}, mutex: &sync.Mutex{}, host: host, cpus: cores}}
	host.reservedPool = &reservedPoolType{poolImpl{host: host, mutex: &sync.Mutex{}, cpus: make([]Cpu, 0)}}

	for _, core := range cores {
		core._setPoolProperty(host.sharedPool)
	}
	coresCopy := make(CpuList, 4)
	copy(coresCopy, cores)
	s.Nil(host.GetReservedPool().MoveCpus(coresCopy))
	s.ElementsMatch(host.GetReservedPool().Cpus().IDs(), coresCopy.IDs())
	s.Len(host.GetSharedPool().Cpus().IDs(), 0)
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
	cores := make(CpuList, 4)
	for i := range cores {
		m := new(mockCpuCore)
		core, err := newCpu(uint(i), m)
		s.Nil(err)

		cores[i] = core
	}

	node := &hostImpl{
		sharedPool: &sharedPoolType{poolImpl{
			name:         sharedPoolName,
			cpus:         cores,
			PowerProfile: &profileImpl{},
		}},
	}
	sharedPool := node.GetSharedPool().(*sharedPoolType)
	s.ElementsMatch(cores.IDs(), sharedPool.cpus.IDs())
	s.Equal(node.sharedPool.(*sharedPoolType).PowerProfile, sharedPool.PowerProfile)
}
func (s *hostTestsSuite) TestGetReservedPool() {
	cores := make(CpuList, 4)
	for i := range cores {
		m := new(mockCpuCore)
		core, err := newCpu(uint(i), m)
		s.Nil(err)
		cores[i] = core
	}
	poolImp := &poolImpl{
		name:         reservedPoolName,
		cpus:         cores,
		PowerProfile: &profileImpl{},
	}
	node := &hostImpl{
		reservedPool: poolImp,
	}
	reservedPool := node.GetReservedPool()
	s.ElementsMatch(cores.IDs(), reservedPool.Cpus().IDs())
	s.Equal(reservedPool.GetPowerProfile(), poolImp.PowerProfile)
}
func (s *hostTestsSuite) TestDeleteProfile() {
	allCores := make(CpuList, 12)
	sharedCores := make(CpuList, 4)
	for i := 0; i < 4; i++ {
		m := new(mockCpuCore)
		core, err := newCpu(uint(i), m)
		s.Nil(err)
		allCores[i] = core
		sharedCores[i] = core
	}
	sharedCoresCopy := make(CpuList, len(sharedCores))
	copy(sharedCoresCopy, sharedCores)

	p1cores := make(CpuList, 4)
	for i := 4; i < 8; i++ {
		m := new(mockCpuCore)
		core, err := newCpu(uint(i), m)
		s.Nil(err)
		allCores[i] = core
		p1cores[i-4] = core
	}
	p1copy := make([]Cpu, len(p1cores))
	copy(p1copy, p1cores)

	p2cores := make(CpuList, 4)
	for i := 8; i < 12; i++ {
		m := new(mockCpuCore)
		core, err := newCpu(uint(i), m)
		s.Nil(err)
		allCores[i] = core
		p2cores[i-8] = core
	}
	p2copy := make(CpuList, len(p2cores))
	copy(p2copy, p2cores)

	host := &hostImpl{}
	exclusive := []Pool{
		&exclusivePoolType{poolImpl{
			name:         "pool1",
			cpus:         p1cores,
			mutex:        &sync.Mutex{},
			PowerProfile: &profileImpl{name: "profile1"},
			host:         host,
		}},
		&exclusivePoolType{poolImpl{
			name:         "pool2",
			cpus:         p2cores,
			mutex:        &sync.Mutex{},
			PowerProfile: &profileImpl{name: "profile2"},
			host:         host,
		}},
	}
	shared := &sharedPoolType{poolImpl{
		name:         sharedPoolName,
		cpus:         sharedCores,
		mutex:        &sync.Mutex{},
		PowerProfile: &profileImpl{name: sharedPoolName},
		host:         host,
	}}
	host.exclusivePools = exclusive
	host.sharedPool = shared
	host.reservedPool = &reservedPoolType{poolImpl{host: host}}
	topology := new(mockCpuTopology)
	topology.On("CPUs").Return(&allCores)
	host.topology = topology
	for i := 0; i < 4; i++ {
		sharedCores[i]._setPoolProperty(host.sharedPool)
		p1cores[i]._setPoolProperty(host.exclusivePools[0])
		p2cores[i]._setPoolProperty(host.exclusivePools[1])
	}
	s.NoError(host.GetExclusivePool("pool1").Remove())
	s.Len(host.exclusivePools, 1)
	s.Equal("profile2", host.exclusivePools[0].(*exclusivePoolType).PowerProfile.(*profileImpl).name)
	s.ElementsMatch(host.exclusivePools[0].(*exclusivePoolType).cpus, p2copy)
	newShared := append(sharedCoresCopy, p1copy...)
	s.ElementsMatch(host.GetSharedPool().Cpus().IDs(), newShared.IDs())

}
