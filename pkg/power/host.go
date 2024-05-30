package power

import (
	"fmt"
	"sync"
)

// The hostImpl is the backing object of Host interface
type hostImpl struct {
	name           string
	exclusivePools PoolList
	reservedPool   Pool
	sharedPool     Pool
	topology       Topology
	featureStates  *FeatureSet
}

// Host represents the actual machine to be managed
type Host interface {
	SetName(name string)
	GetName() string
	GetFeaturesInfo() FeatureSet

	GetReservedPool() Pool
	GetSharedPool() Pool

	AddExclusivePool(poolName string) (Pool, error)
	GetExclusivePool(poolName string) Pool
	GetAllExclusivePools() *PoolList

	GetAllCpus() *CpuList
	GetFreqRanges() CoreTypeList
	Topology() Topology
	// returns number of distinct core types
	NumCoreTypes() uint
	AvailableCStates() []string
	ValidateCStates(states CStates) error
}

// create a pre-populated Host object
func initHost(nodeName string) (Host, error) {

	host := &hostImpl{
		name:           nodeName,
		exclusivePools: PoolList{},
	}
	host.featureStates = &featureList
	// create predefined pools
	host.reservedPool = &reservedPoolType{poolImpl{
		name:  reservedPoolName,
		mutex: &sync.Mutex{},
		host:  host,
	}}
	host.sharedPool = &sharedPoolType{poolImpl{
		name:  sharedPoolName,
		cpus:  CpuList{},
		mutex: &sync.Mutex{},
		host:  host,
	}}

	topology, err := discoverTopology()
	if err != nil {
		log.Error(err, "failed to discover cpuTopology")
		return nil, fmt.Errorf("failed to init host: %w", err)
	}
	for _, cpu := range *topology.CPUs() {
		cpu._setPoolProperty(host.reservedPool)
	}
	// not very pretty but finds the lowest/highest core ranges
	var highest uint
	var highIndex uint
	var lowIndex uint
	for i, frequencies := range coreTypes {
		if frequencies.GetMax() > highest {
			highest = frequencies.GetMax()
			lowIndex = highIndex
			highIndex = uint(i)
		}
		if frequencies.GetMax() < highest {
			lowIndex = uint(i)
		}
	}
	CpuTypeReferences.pcore = highIndex
	CpuTypeReferences.ecore = lowIndex
	log.Info("discovered cpus", "cpus", len(*topology.CPUs()))
	// coretypes are populated after default profile is generated so we need to update here
	if featureList.isFeatureIdSupported(FrequencyScalingFeature) && host.NumCoreTypes() == 2 {
		defaultPowerProfile.max = coreTypes[CpuTypeReferences.Pcore()].GetMax()
		defaultPowerProfile.min = coreTypes[CpuTypeReferences.Pcore()].GetMax()
		defaultPowerProfile.efficientMax = coreTypes[CpuTypeReferences.Ecore()].GetMax()
		defaultPowerProfile.efficientMin = coreTypes[CpuTypeReferences.Ecore()].GetMax()
	}
	if host.NumCoreTypes() > numOfSupportedCoreTypes {
		log.Error(fmt.Errorf("more than %d core types detected. This may result in undefined behavior: %v", numOfSupportedCoreTypes, coreTypes), "topology issues detected")
	}
	host.topology = topology

	// create a shallow copy of pointers, changes to underlying cpu object will reflect in both lists,
	// changes to each list will not affect the other
	host.reservedPool.(*reservedPoolType).cpus = make(CpuList, len(*topology.CPUs()))
	copy(host.reservedPool.(*reservedPoolType).cpus, *topology.CPUs())
	return host, nil
}

func (host *hostImpl) SetName(name string) {
	host.name = name
}

func (host *hostImpl) GetName() string {
	return host.name
}

func (host *hostImpl) GetReservedPool() Pool {
	return host.reservedPool
}

// returns default min/max frequency range
func (host *hostImpl) GetFreqRanges() CoreTypeList {
	return coreTypes
}

// AddExclusivePool creates new empty pool
func (host *hostImpl) AddExclusivePool(poolName string) (Pool, error) {
	if i := host.exclusivePools.IndexOfName(poolName); i >= 0 {
		return host.exclusivePools[i], fmt.Errorf("pool with name %s already exists", poolName)
	}
	var pool Pool = &exclusivePoolType{poolImpl{
		name:  poolName,
		mutex: &sync.Mutex{},
		cpus:  make([]Cpu, 0),
		host:  host,
	}}

	host.exclusivePools.add(pool)
	return pool, nil
}

// GetExclusivePool Returns a Pool object of the exclusive pool with matching name supplied
// returns nil if not found
func (host *hostImpl) GetExclusivePool(name string) Pool {
	return host.exclusivePools.ByName(name)
}

// GetSharedPool returns shared pool
func (host *hostImpl) GetSharedPool() Pool {
	return host.sharedPool
}

func (host *hostImpl) GetFeaturesInfo() FeatureSet {
	return *host.featureStates
}

func (host *hostImpl) GetAllCpus() *CpuList {
	return host.topology.CPUs()
}

func (host *hostImpl) GetAllExclusivePools() *PoolList {
	return &host.exclusivePools
}

func (host *hostImpl) NumCoreTypes() uint {
	return uint(len(coreTypes))
}

func (host *hostImpl) Topology() Topology {
	return host.topology
}
