package power

import (
	"fmt"
)

type poolImpl struct {
	name  string
	cores CoreList
	host  Host
	// P-States
	PowerProfile Profile
	// C-States
	CStatesProfile *CStates
}

type Pool interface {
	Name() string
	Cores() *CoreList

	SetCoreIDs(coreIDs []uint) error
	SetCores(requestedCores CoreList) error

	Remove() error

	Clear() error
	MoveCores(cores CoreList) error
	MoveCoresIDs(coreIDs []uint) error

	SetPowerProfile(profile Profile) error
	GetPowerProfile() Profile

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

func (pool *poolImpl) Cores() *CoreList {
	return &pool.cores
}

func (pool *poolImpl) SetCoreIDs([]uint) error {
	panic("virtual")
} // virtual

func (pool *poolImpl) SetCores(CoreList) error {
	// virtual function to be overwritten by exclusivePoolType, sharedPoolType and ReservedPoolType
	panic("scuffed")
} //virtual

func (pool *poolImpl) MoveCores(cores CoreList) error {
	for _, core := range cores {
		if err := core.SetPool(pool); err != nil {
			return err
		}
	}
	return nil
}

func (pool *poolImpl) MoveCoresIDs(coreIDs []uint) error {
	cores, err := pool.host.GetAllCores().ManyByIDs(coreIDs)
	if err != nil {
		return err
	}
	return pool.MoveCores(cores)
}

func (pool *poolImpl) Remove() error {
	panic("'virtual' function")
} // virtual

func (pool *poolImpl) Clear() error {
	panic("scuffed")
} // virtual

func (pool *poolImpl) SetPowerProfile(profile Profile) error {
	pool.PowerProfile = profile
	for _, core := range pool.cores {
		err := core.consolidate()
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

func (sharedPool *sharedPoolType) SetCoreIDs(coreIDs []uint) error {
	cores, err := sharedPool.host.GetAllCores().ManyByIDs(coreIDs)
	if err != nil {
		return fmt.Errorf("core out of range: %w", err)
	}
	return sharedPool.SetCores(cores)
}

// SetCores on shared pool with place all desired cores in shared pool
// undesired cores that were in the shared pool will be placed in the reserved pool
func (sharedPool *sharedPoolType) SetCores(requestedCores CoreList) error {
	for _, core := range *sharedPool.host.GetAllCores() {
		if requestedCores.Contains(core) {
			err := core.SetPool(sharedPool)
			if err != nil {
				return err
			}
		} else {
			if core.getPool() == sharedPool { // move cores we don't want if the shared pool to reserved, don't touch any exclusive
				err := core.SetPool(sharedPool.host.GetReservedPool())
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (sharedPool *sharedPoolType) Clear() error {
	return sharedPool.SetCores(CoreList{})
}
func (sharedPool *sharedPoolType) Remove() error {
	return fmt.Errorf("shared pool canot be removed")
}

type reservedPoolType struct {
	poolImpl
}

func (reservedPool *reservedPoolType) SetCoreIDs(coreIDs []uint) error {
	cores, err := reservedPool.host.GetAllCores().ManyByIDs(coreIDs)
	if err != nil {
		return fmt.Errorf("core out of range: %w", err)
	}
	return reservedPool.SetCores(cores)
}
func (reservedPool *reservedPoolType) SetPowerProfile(Profile) error {
	return fmt.Errorf("cannot set power profile for reserved pool")
}

func (reservedPool *reservedPoolType) SetCores(cores CoreList) error {
	/*
		case 1: core in any exclusive pool, not passed matching IDs -> untouched
		case 2: core in any exclusive pool, matching passed IDs -> error

		case 3: core in shared pool, not matching IDs passed -> untouched
		case 4: core in shared pool, IDs match passed -> move to reserved

		case 5: core in reserved pool, not matching IDs passed -> move to shared
		case 6: core in reserved pool, IDs match passed -> untouched
	*/

	sharedPool := reservedPool.host.GetSharedPool()

	for _, core := range *reservedPool.host.GetAllCores() {
		if cores.Contains(core) { // case 2,4, 6
			if core.getPool().isExclusive() { // case 2
				return fmt.Errorf("cores cannot be moved directly from exclusive to reserved pool")
			}
			err := core.SetPool(reservedPool) // case 4
			if err != nil {
				return err
			}
		} else { // case 1,3,5
			if core.getPool() == reservedPool { // case 5
				err := core.SetPool(sharedPool)
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
	return reservedPool.SetCores(CoreList{})
}

type exclusivePoolType struct {
	poolImpl
}

func (pool *exclusivePoolType) SetCoreIDs(coreIDs []uint) error {
	cores, err := pool.host.GetAllCores().ManyByIDs(coreIDs)
	if err != nil {
		return fmt.Errorf("core out of range: %w", err)
	}
	return pool.SetCores(cores)
}

func (pool *exclusivePoolType) SetCores(requestedCores CoreList) error {
	for _, core := range *pool.host.GetAllCores() {
		if requestedCores.Contains(core) {
			err := core.SetPool(pool)
			if err != nil {
				return err
			}
		} else {
			if core.getPool() != pool {
				continue
			}
			err := core.SetPool(pool.host.GetSharedPool())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (pool *exclusivePoolType) Clear() error {
	return pool.SetCores(CoreList{})
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
