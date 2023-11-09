package power

import (
	"fmt"
	"sync"
)

type poolImpl struct {
	name  string
	cpus  CpuList
	mutex sync.Locker
	host  Host
	// Scaling-Driver
	PowerProfile Profile
	// C-States
	CStatesProfile *CStates
}

type Pool interface {
	Name() string
	Cpus() *CpuList

	SetCpuIDs(cpuIDs []uint) error
	SetCpus(requestedCpus CpuList) error

	Remove() error

	Clear() error
	MoveCpus(cpus CpuList) error
	MoveCpuIDs(cpuIDs []uint) error

	SetPowerProfile(profile Profile) error
	GetPowerProfile() Profile

	poolMutex() sync.Locker

	// c-states
	SetCStates(states CStates) error
	getCStates() *CStates
	// private interface members
	getHost() Host
	isExclusive() bool
}

func (pool *poolImpl) Name() string {
	return pool.name
}

func (pool *poolImpl) Cpus() *CpuList {
	return &pool.cpus
}

func (pool *poolImpl) SetCpuIDs([]uint) error {
	panic("virtual")
} // virtual

func (pool *poolImpl) SetCpus(CpuList) error {
	// virtual function to be overwritten by exclusivePoolType, sharedPoolType and ReservedPoolType
	panic("scuffed")
} //virtual

func (pool *poolImpl) MoveCpus(cpus CpuList) error {
	panic("virtual")
}

func (pool *poolImpl) MoveCpuIDs(cpuIDs []uint) error {
	panic("virtual")
}

func (pool *poolImpl) Remove() error {
	panic("'virtual' function")
} // virtual

func (pool *poolImpl) Clear() error {
	panic("scuffed")
} // virtual

func (pool *poolImpl) poolMutex() sync.Locker {
	return pool.mutex
}

func (pool *poolImpl) SetPowerProfile(profile Profile) error {
	log.V(4).Info("SetPowerProfile mutex lock", "pool", pool.name)
	pool.mutex.Lock()
	pool.PowerProfile = profile
	defer func() {
		pool.mutex.Unlock()
		log.V(4).Info("SetPowerProfile mutex unlock", "pool", pool.name)
	}()
	for _, cpu := range pool.cpus {
		err := cpu.consolidate()
		if err != nil {
			return err
		}
	}
	return nil
}

func (pool *poolImpl) GetPowerProfile() Profile {
	return pool.PowerProfile
}

func (pool *poolImpl) getHost() Host {
	return pool.host
}

func (pool *poolImpl) isExclusive() bool {
	return false
}

type sharedPoolType struct {
	poolImpl
}

func (sharedPool *sharedPoolType) MoveCpuIDs(cpuIDs []uint) error {
	cpus, err := sharedPool.host.GetAllCpus().ManyByIDs(cpuIDs)
	if err != nil {
		return err
	}
	return sharedPool.MoveCpus(cpus)
}
func (sharedPool *sharedPoolType) MoveCpus(cpus CpuList) error {
	for _, cpu := range cpus {
		if err := cpu.SetPool(sharedPool); err != nil {
			return err
		}
	}
	return nil
}
func (sharedPool *sharedPoolType) SetCpuIDs(cpuIDs []uint) error {
	cores, err := sharedPool.host.GetAllCpus().ManyByIDs(cpuIDs)
	if err != nil {
		return fmt.Errorf("cpuCore out of range: %w", err)
	}
	return sharedPool.SetCpus(cores)
}

// SetCpus on shared pool with place all desired cpus in shared pool
// undesired cpus that were in the shared pool will be placed in the reserved pool
func (sharedPool *sharedPoolType) SetCpus(requestedCores CpuList) error {
	for _, cpu := range *sharedPool.host.GetAllCpus() {
		if requestedCores.Contains(cpu) {
			err := cpu.SetPool(sharedPool)
			if err != nil {
				return err
			}
		} else {
			if cpu.getPool() == sharedPool { // move cpus we don't want if the shared pool to reserved, don't touch any exclusive
				err := cpu.SetPool(sharedPool.host.GetReservedPool())
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (sharedPool *sharedPoolType) Clear() error {
	return sharedPool.SetCpus(CpuList{})
}
func (sharedPool *sharedPoolType) Remove() error {
	return fmt.Errorf("shared pool canot be removed")
}

type reservedPoolType struct {
	poolImpl
}

func (reservedPool *reservedPoolType) MoveCpuIDs(cpuIDs []uint) error {
	cpus, err := reservedPool.host.GetAllCpus().ManyByIDs(cpuIDs)
	if err != nil {
		return err
	}
	return reservedPool.MoveCpus(cpus)
}
func (reservedPool *reservedPoolType) MoveCpus(cpus CpuList) error {
	for _, cpu := range cpus {
		if err := cpu.SetPool(reservedPool); err != nil {
			return err
		}
	}
	return nil
}
func (reservedPool *reservedPoolType) SetCpuIDs(cpuIDs []uint) error {
	cpus, err := reservedPool.host.GetAllCpus().ManyByIDs(cpuIDs)
	if err != nil {
		return fmt.Errorf("cpuCore out of range: %w", err)
	}
	return reservedPool.SetCpus(cpus)
}
func (reservedPool *reservedPoolType) SetPowerProfile(Profile) error {
	return fmt.Errorf("cannot set power profile for reserved pool")
}

func (reservedPool *reservedPoolType) SetCpus(cores CpuList) error {
	/*
		case 1: cpu in any exclusive pool, not passed matching IDs -> untouched
		case 2: cpu in any exclusive pool, matching passed IDs -> error

		case 3: cpu in shared pool, not matching IDs passed -> untouched
		case 4: cpu in shared pool, IDs match passed -> move to reserved

		case 5: cpu in reserved pool, not matching IDs passed -> move to shared
		case 6: cpu in reserved pool, IDs match passed -> untouched
	*/

	sharedPool := reservedPool.host.GetSharedPool()

	for _, cpu := range *reservedPool.host.GetAllCpus() {
		if cores.Contains(cpu) { // case 2,4, 6
			if cpu.getPool().isExclusive() { // case 2
				return fmt.Errorf("cpus cannot be moved directly from exclusive to reserved pool")
			}
			err := cpu.SetPool(reservedPool) // case 4
			if err != nil {
				return err
			}
		} else { // case 1,3,5
			if cpu.getPool() == reservedPool { // case 5
				err := cpu.SetPool(sharedPool)
				if err != nil {
					return err
				}
			}
			continue // 1,3 do nothing
		}
	}
	return nil
}

func (reservedPool *reservedPoolType) Remove() error {
	return fmt.Errorf("reserved Pool cannot be removed")
}

func (reservedPool *reservedPoolType) Clear() error {
	return reservedPool.SetCpus(CpuList{})
}

type exclusivePoolType struct {
	poolImpl
}

func (pool *exclusivePoolType) MoveCpuIDs(cpuIDs []uint) error {
	cpus, err := pool.host.GetAllCpus().ManyByIDs(cpuIDs)
	if err != nil {
		return err
	}
	return pool.MoveCpus(cpus)
}
func (pool *exclusivePoolType) MoveCpus(cpus CpuList) error {
	for _, cpu := range cpus {
		if err := cpu.SetPool(pool); err != nil {
			return err
		}
	}
	return nil
}
func (pool *exclusivePoolType) SetCpuIDs(cpuIDs []uint) error {
	cpus, err := pool.host.GetAllCpus().ManyByIDs(cpuIDs)
	if err != nil {
		return fmt.Errorf("cpuCore out of range: %w", err)
	}
	return pool.SetCpus(cpus)
}

func (pool *exclusivePoolType) SetCpus(requestedCores CpuList) error {
	for _, cpu := range *pool.host.GetAllCpus() {
		if requestedCores.Contains(cpu) {
			err := cpu.SetPool(pool)
			if err != nil {
				return err
			}
		} else {
			if cpu.getPool() != pool {
				continue
			}
			err := cpu.SetPool(pool.host.GetSharedPool())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (pool *exclusivePoolType) Clear() error {
	return pool.SetCpus(CpuList{})
}

func (pool *exclusivePoolType) Remove() error {
	if err := pool.Clear(); err != nil {
		return err
	}
	if err := pool.host.GetAllExclusivePools().remove(pool); err != nil {
		return err
	}
	// improvement: mark current pool as invalid
	// *pool = nil
	return nil
}

func (pool *exclusivePoolType) isExclusive() bool {
	return true
}

type PoolList []Pool

func (pools *PoolList) IndexOf(pool Pool) int {
	for i, p := range *pools {
		if p == pool {
			return i
		}
	}
	return -1
}

func (pools *PoolList) IndexOfName(name string) int {
	for i, p := range *pools {
		if p.Name() == name {
			return i
		}
	}
	return -1
}

func (pools *PoolList) Contains(pool Pool) bool {
	if pools.IndexOf(pool) < 0 {
		return false
	} else {
		return true
	}
}

func (pools *PoolList) remove(pool Pool) error {
	index := pools.IndexOf(pool)
	if index < 0 {
		return fmt.Errorf("pool %s not in on host", pool.Name())
	}
	size := len(*pools) - 1
	(*pools)[index] = (*pools)[size]
	*pools = (*pools)[:size]
	return nil
}

func (pools *PoolList) add(pool Pool) {
	*pools = append(*pools, pool)
}

func (pools *PoolList) ByName(name string) Pool {
	index := pools.IndexOfName(name)
	if index < 0 {
		return nil
	}
	return (*pools)[index]
}
