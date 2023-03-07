package power

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// todo package (socket) awareness

type Core interface {
	GetID() uint
	SetPool(pool Pool) error

	getPool() Pool
	doSetPool(pool Pool) error
	consolidate() error

	// C-States stuff
	SetCStates(cStates CStates) error

	// used only to set initial pool when creating core instance
	_setPoolProperty(pool Pool)
}

type coreImpl struct {
	id    uint
	mutex *sync.Mutex
	pool  Pool
	// C-States properties
	cStates *CStates
}

func newCore(coreID uint) (Core, error) {
	core := &coreImpl{
		id:    coreID,
		mutex: &sync.Mutex{},
	}

	return core, nil
}

func (core *coreImpl) consolidate() error {
	if err := core.updateFrequencies(); err != nil {
		return err
	}
	if err := core.updateCStates(); err != nil {
		return err
	}
	return nil
}

// SetPool moves current core to a specified target pool
// allowed movements are reservedPoolType <-> sharedPoolType and sharedPoolType <-> any exclusive pool
func (core *coreImpl) SetPool(targetPool Pool) error {
	/*
		case 0: current and target pool are the same -> do nothing

		case 1: target = reserved, current = reserved  -> case 0
		case 2: target = reserved, current = shared -> do it
		case 3: target = reserved, current = exclusive -> error

		case 4: target = shared, current = exclusive -> do it
		case 5: target = shared, current = shared -> case 0
		case 6: target = shared, current = reserved -> do it

		case 7: target = exclusive, current = other exclusive -> error
		case 8: target = exclusive, current = shared -> do it
		case 9: target = exclusive, current = reserved -> error

	*/
	if targetPool == nil {
		return fmt.Errorf("target pool cannot be nil")
	}

	log.Info("Set pool", "core", core.id, "source pool", core.pool.Name(), "target pool", targetPool.Name())
	if core.pool == targetPool { // case 0,1,5
		return nil
	}
	reservedPool := core.pool.getHost().GetReservedPool()
	sharedPool := core.pool.getHost().GetSharedPool()
	if core.pool == reservedPool && targetPool.isExclusive() { // case 3
		return fmt.Errorf("cannot move from reserved to exclusive pool")
	}

	if core.pool.isExclusive() && targetPool.isExclusive() { // case 7
		return fmt.Errorf("cannot move exclusive to different exclusive pool")
	}

	if core.pool.isExclusive() && targetPool == reservedPool { // case 9
		return fmt.Errorf("cannot move from exclusive to reserved")
	}

	// cases 2,4,5,6,8
	if targetPool == sharedPool || core.pool == sharedPool {
		return core.doSetPool(targetPool)
	}
	panic("we should never get here")
}

func (core *coreImpl) doSetPool(pool Pool) error {
	log.V(4).Info("mutex locking core", "coreID", core.id)
	core.mutex.Lock()

	defer func() {
		log.V(4).Info("mutex unlocking core", "coreID", core.id)
		core.mutex.Unlock()
	}()

	origPool := core.pool
	core.pool = pool

	origPoolCores := origPool.Cores()
	log.V(4).Info("removing core from pool", "pool", origPool.Name(), "coreID", core.id)
	if err := origPoolCores.remove(core); err != nil {
		core.pool = origPool
		return err
	}

	log.V(4).Info("starting consolidation of core", "coreID", core.id)
	if err := core.consolidate(); err != nil {
		core.pool = origPool
		origPoolCores.add(core)
		return err
	}

	newPoolCores := core.pool.Cores()
	newPoolCores.add(core)
	return nil
}

func (core *coreImpl) getPool() Pool {
	return core.pool
}

func (core *coreImpl) GetID() uint {
	return core.id
}

func (core *coreImpl) _setPoolProperty(pool Pool) {
	core.pool = pool
}

// read property of specific CPU as an int, takes CPUid and path to specific file within cpu subdirectory in sysfs
func readCoreIntProperty(coreID uint, file string) (int, error) {
	path := filepath.Join(basePath, fmt.Sprint("cpu", coreID), file)
	return readIntFromFile(path)
}

// reads content of a file and returns it as a string
func readCoreStringProperty(coreID uint, file string) (string, error) {
	path := filepath.Join(basePath, fmt.Sprint("cpu", coreID), file)
	value, err := readStringFromFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read core %d string property: %w", coreID, err)
	}
	value = strings.TrimSuffix(value, "\n")
	return value, nil
}

type CoreList []Core

func (cores *CoreList) IndexOf(core Core) int {
	for i, c := range *cores {
		if c == core {
			return i
		}
	}
	return -1
}

func (cores *CoreList) Contains(core Core) bool {
	if cores.IndexOf(core) < 0 {
		return false
	} else {
		return true
	}
}
func (cores *CoreList) add(core Core) {
	*cores = append(*cores, core)
}
func (cores *CoreList) remove(core Core) error {
	index := cores.IndexOf(core)
	if index < 0 {
		return fmt.Errorf("core %d is not in pool", core.GetID())
	}
	size := len(*cores) - 1
	(*cores)[index] = (*cores)[size]
	*cores = (*cores)[:size]
	return nil
}
func (cores *CoreList) IDs() []uint {
	ids := make([]uint, len(*cores))
	for i, core := range *cores {
		ids[i] = core.GetID()
	}
	return ids
}
func (cores *CoreList) ByID(id uint) Core {
	index := int(id)
	// first we try index == coreId
	if len(*cores) > index && (*cores)[index].GetID() == id {
		return (*cores)[index]
	}
	// if that doesn't work we fall back to looping
	for _, core := range *cores {
		if core.GetID() == id {
			return core
		}
	}
	return nil
}
func (cores *CoreList) ManyByIDs(ids []uint) (CoreList, error) {
	targets := make(CoreList, len(ids))

	for i, id := range ids {
		core := cores.ByID(id)
		if core == nil {
			return nil, fmt.Errorf("core with id %d, not in list", id)
		}
		targets[i] = core
	}
	return targets, nil
}

var getAllCores = func() (CoreList, error) {
	numOfCores := getNumberOfCpus()
	cores := make(CoreList, numOfCores)
	for i := uint(0); i < numOfCores; i++ {
		core, err := newCore(i)
		if err != nil {
			return nil, err
		}
		cores[i] = core
	}
	return cores, nil
}
