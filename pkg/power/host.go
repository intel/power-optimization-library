package power

import (
	"fmt"
)

// The hostImpl is the backing object of Host interface
type hostImpl struct {
	name           string
	exclusivePools PoolList
	reservedPool   Pool
	sharedPool     Pool
	allCores       CoreList
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

	GetAllCores() *CoreList

	AvailableCStates() []string
	ValidateCStates(states CStates) error
	//IsCStateValid(s string) bool
}

// create a pre-populated Host object
func initHost(nodeName string) (Host, error) {

	host := &hostImpl{
		name:           nodeName,
		exclusivePools: PoolList{},
		featureStates:  &featureList,
	}

	// create predefined pools
	host.reservedPool = &reservedPoolType{poolImpl{
		name: reservedPoolName,
		host: host,
	}}
	host.sharedPool = &sharedPoolType{poolImpl{
		name:  sharedPoolName,
		cores: CoreList{},
		host:  host,
	}}

	allCores, err := getAllCores()
	if err != nil {
		log.Error(err, "failed to discover cpus")
		return nil, fmt.Errorf("failed to init host: %w", err)
	}
	for _, core := range allCores {
		core._setPoolProperty(host.reservedPool)
	}

	log.Info("discovered cores", "cores", len(allCores))

	host.allCores = allCores

	// create a shallow copy of pointers, changes to underlying core object will reflect in both lists,
	// changes to each list will not affect the other
	host.reservedPool.(*reservedPoolType).cores = make(CoreList, len(allCores))
	copy(host.reservedPool.(*reservedPoolType).cores, allCores)
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

// AddExclusivePool creates new empty pool
func (host *hostImpl) AddExclusivePool(poolName string) (Pool, error) {
	if i := host.exclusivePools.IndexOfName(poolName); i >= 0 {
		return host.exclusivePools[i], fmt.Errorf("pool with name %s already exists", poolName)
	}
	var pool Pool = &exclusivePoolType{poolImpl{
		name:  poolName,
		cores: make([]Core, 0),
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

func (host *hostImpl) GetAllCores() *CoreList {
	return &host.allCores
}

func (host *hostImpl) GetAllExclusivePools() *PoolList {
	return &host.exclusivePools
}
