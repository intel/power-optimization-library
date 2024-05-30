package power

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

const (
	numOfSupportedCoreTypes uint = 2
)

// uints are references to an array index of frequency sets
type supportedCores struct {
	pcore uint
	ecore uint
}

func (c *supportedCores) Pcore() uint {
	return c.pcore
}
func (c *supportedCores) Ecore() uint {
	return c.ecore
}

// public instance with read only access to supported core types
var CpuTypeReferences = supportedCores{}

// Cpu represents a compute unit/thread as seen by the OS
// it is either a physical core ot virtual thread if hyperthreading/SMT is enabled
type Cpu interface {
	GetID() uint
	GetAbsMinMax() (uint, uint)
	SetPool(pool Pool) error

	getPool() Pool
	doSetPool(pool Pool) error
	consolidate() error
	consolidate_unsafe() error
	GetCore() Core
	// C-States stuff
	SetCStates(cStates CStates) error

	// used only to set initial pool when creating core instance
	_setPoolProperty(pool Pool)
}

type cpuImpl struct {
	id    uint
	mutex sync.Locker
	pool  Pool
	core  Core
	// C-States properties
	cStates *CStates
}

func newCpu(coreID uint, core Core) (Cpu, error) {
	if featureList.isFeatureIdSupported(FrequencyScalingFeature) {
		min, max, err := readCpuFreqLimits(coreID)
		if err != nil {
			return &cpuImpl{}, err
		}
		cType := coreTypes.appendIfUnique(min, max)
		core.setType(cType)
	}
	cpu := &cpuImpl{
		id:    coreID,
		mutex: &sync.Mutex{},
		core:  core,
	}

	return cpu, nil
}

func (cpu *cpuImpl) consolidate() error {
	cpu.mutex.Lock()
	defer cpu.mutex.Unlock()
	return cpu.consolidate_unsafe()
}
func (cpu *cpuImpl) consolidate_unsafe() error {
	if err := cpu.updateFrequencies(); err != nil {
		return err
	}
	if err := cpu.updateCStates(); err != nil {
		return err
	}
	return nil
}

// SetPool moves current core to a specified target pool
// allowed movements are reservedPoolType <-> sharedPoolType and sharedPoolType <-> any exclusive pool
func (cpu *cpuImpl) SetPool(targetPool Pool) error {
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

	log.Info("Set pool", "cpu", cpu.id, "source pool", cpu.pool.Name(), "target pool", targetPool.Name())
	cpu.mutex.Lock()
	defer cpu.mutex.Unlock()

	if cpu.pool == targetPool { // case 0,1,5
		return nil
	}
	reservedPool := cpu.pool.getHost().GetReservedPool()
	sharedPool := cpu.pool.getHost().GetSharedPool()
	if cpu.pool == reservedPool && targetPool.isExclusive() { // case 3
		return fmt.Errorf("cannot move from reserved to exclusive pool")
	}

	if cpu.pool.isExclusive() && targetPool.isExclusive() { // case 7
		return fmt.Errorf("cannot move exclusive to different exclusive pool")
	}

	if cpu.pool.isExclusive() && targetPool == reservedPool { // case 9
		return fmt.Errorf("cannot move from exclusive to reserved")
	}

	// cases 2,4,5,6,8
	if targetPool == sharedPool || cpu.pool == sharedPool {
		return cpu.doSetPool(targetPool)
	}
	panic("we should never get here")
}

func (cpu *cpuImpl) doSetPool(pool Pool) error {
	cpu.pool.poolMutex().Lock()
	pool.poolMutex().Lock()
	log.V(4).Info("acquired mutexes", "source", cpu.pool.Name(), "target", pool.Name(), "cpu", cpu.id)

	origPool := cpu.pool
	cpu.pool = pool

	defer func() {
		log.V(4).Info("releasing mutexes", "source", origPool.Name(), "target", pool.Name())
		origPool.poolMutex().Unlock()
		pool.poolMutex().Unlock()
	}()

	origPoolCpus := origPool.Cpus()
	log.V(4).Info("removing cpu from pool", "pool", origPool.Name(), "coreID", cpu.id)
	if err := origPoolCpus.remove(cpu); err != nil {
		cpu.pool = origPool
		return err
	}

	log.V(4).Info("starting consolidation of cpu", "coreID", cpu.id)
	if err := cpu.consolidate_unsafe(); err != nil {
		cpu.pool = origPool
		origPoolCpus.add(cpu)
		return err
	}

	newPoolCpus := cpu.pool.Cpus()
	newPoolCpus.add(cpu)
	return nil
}

func (cpu *cpuImpl) getPool() Pool {
	return cpu.pool
}

func (cpu *cpuImpl) GetID() uint {
	return cpu.id
}

func (cpu *cpuImpl) GetAbsMinMax() (uint, uint) {
	// return 0,0 to prevent indexing error on coretype
	if !featureList.isFeatureIdSupported(FrequencyScalingFeature) {
		return 0, 0
	}
	typeNum := cpu.core.GetType()
	return coreTypes[typeNum].GetMin(), coreTypes[typeNum].GetMax()
}

func (cpu *cpuImpl) GetCore() Core {
	return cpu.core
}

func (cpu *cpuImpl) _setPoolProperty(pool Pool) {
	cpu.pool = pool
}

// read property of specific CPU as an int, takes CPUid and path to specific file within cpu subdirectory in sysfs
func readCpuUintProperty(cpuID uint, file string) (uint, error) {
	path := filepath.Join(basePath, fmt.Sprint("cpu", cpuID), file)
	return readUintFromFile(path)
}

// reads content of a file and returns it as a string
func readCpuStringProperty(cpuID uint, file string) (string, error) {
	path := filepath.Join(basePath, fmt.Sprint("cpu", cpuID), file)
	value, err := readStringFromFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read cpuCore %d string property: %w", cpuID, err)
	}
	value = strings.TrimSuffix(value, "\n")
	return value, nil
}

// reads the min and max frequency of a CPU
func readCpuFreqLimits(id uint) (uint, uint, error) {
	maxFreq, err := readCpuUintProperty(id, cpuMaxFreqFile)
	if err != nil {
		return 0, 0, err
	}
	minFreq, err := readCpuUintProperty(id, cpuMinFreqFile)
	if err != nil {
		return 0, 0, err
	}
	return minFreq, maxFreq, nil
}

type CpuList []Cpu

func (cpus *CpuList) IndexOf(cpu Cpu) int {
	for i, c := range *cpus {
		if c == cpu {
			return i
		}
	}
	return -1
}

func (cpus *CpuList) Contains(cpu Cpu) bool {
	if cpus.IndexOf(cpu) < 0 {
		return false
	} else {
		return true
	}
}
func (cpus *CpuList) add(cpu Cpu) {
	*cpus = append(*cpus, cpu)
}
func (cpus *CpuList) remove(cpu Cpu) error {
	index := cpus.IndexOf(cpu)
	if index < 0 {
		return fmt.Errorf("cpu %d is not in pool", cpu.GetID())
	}
	size := len(*cpus) - 1
	(*cpus)[index] = (*cpus)[size]
	*cpus = (*cpus)[:size]
	return nil
}
func (cpus *CpuList) IDs() []uint {
	ids := make([]uint, len(*cpus))
	for i, cpu := range *cpus {
		ids[i] = cpu.GetID()
	}
	return ids
}
func (cpus *CpuList) ByID(id uint) Cpu {
	index := int(id)
	// first we try index == cpuId
	if len(*cpus) > index && (*cpus)[index].GetID() == id {
		return (*cpus)[index]
	}
	// if that doesn't work we fall back to looping
	for _, cpu := range *cpus {
		if cpu.GetID() == id {
			return cpu
		}
	}
	return nil
}
func (cpus *CpuList) ManyByIDs(ids []uint) (CpuList, error) {
	targets := make(CpuList, len(ids))

	for i, id := range ids {
		cpu := cpus.ByID(id)
		if cpu == nil {
			return nil, fmt.Errorf("cpu with id %d, not in list", id)
		}
		targets[i] = cpu
	}
	return targets, nil
}
