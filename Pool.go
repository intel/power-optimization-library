package power

import "github.com/pkg/errors"

type poolImpl struct {
	Name         string
	Cores        []Core
	PowerProfile Profile
}

type Pool interface {
	GetName() string
	addCore(core Core) error
	removeCore(core Core) error
	removeCoreByID(coreID int) (Core, error)
	SetPowerProfile(profile Profile) error
	GetPowerProfile() Profile
	GetCores() []Core
	GetCoreIds() []int
}

func (pool *poolImpl) GetName() string {
	return pool.Name
}

func (pool *poolImpl) GetPowerProfile() Profile {
	return pool.PowerProfile
}

func (pool *poolImpl) addCore(core Core) error {
	for _, v := range pool.Cores {
		if v == core {
			return errors.Errorf("core %d already in the pool", core.GetID())
		}
	}
	if pool.PowerProfile != nil {
		err := core.updateValues(
			pool.PowerProfile.GetEpp(),
			pool.PowerProfile.GetMinFreq(),
			pool.PowerProfile.GetMaxFreq(),
		)
		if err != nil {
			return errors.Wrap(err, "SetPowerProfile")
		}
	} else {
		err := core.restoreValues()
		if err != nil {
			return errors.Wrap(err, "SetPowerProfile")
		}
	}
	pool.Cores = append(pool.Cores, core)
	return nil
}

func (pool *poolImpl) removeCore(core Core) error {
	index := -1
	for i, v := range pool.Cores {
		if v == core {
			index = i
			break
		}
	}
	if index < 0 {
		return errors.Errorf("core id %d not found in pool %s", core.GetID(), pool.Name)
	}
	return pool.doRemoveCore(index)
}

func (pool *poolImpl) removeCoreByID(coreId int) (Core, error) {
	index := -1
	var coreObj Core
	for i, core := range pool.Cores {
		if core.GetID() == coreId {
			index = i
			coreObj = core
			break
		}
	}
	if index < 0 {
		return nil, errors.Errorf("core id %d not found in pool %s", coreId, pool.Name)
	}
	if coreObj.getReserved() {
		return coreObj, errors.Errorf("Core %d is system reserved and cannot be moved", coreId)
	}
	return coreObj, pool.doRemoveCore(index)
}

func (pool *poolImpl) GetCoreIds() []int {
	coreIds := make([]int, len(pool.Cores))
	for i, core := range pool.Cores {
		coreIds[i] = core.GetID()
	}
	return coreIds
}

func (pool *poolImpl) GetCores() []Core {
	return pool.Cores
}

// SetPowerProfile will set new power profile for the pool configuration of all cores in the pool will be updated
// nil will reset cores to their default values
func (pool *poolImpl) SetPowerProfile(profile Profile) error {
	pool.PowerProfile = profile
	if profile != nil {
		for _, core := range pool.Cores {
			if core.getReserved() {
				continue
			}
			err := core.updateValues(
				profile.GetEpp(),
				profile.GetMinFreq(),
				profile.GetMaxFreq(),
			)
			if err != nil {
				return errors.Wrap(err, "SetPowerProfile")
			}
		}
	} else {
		for _, core := range pool.Cores {
			if !core.getReserved() {
				err := core.restoreValues()
				if err != nil {
					return errors.Wrap(err, "SetPowerProfile")
				}
			}
		}
	}
	return nil
}

func (pool *poolImpl) doRemoveCore(index int) error {
	pool.Cores[index] = pool.Cores[len(pool.Cores)-1]
	pool.Cores = pool.Cores[:len(pool.Cores)-1]
	return nil
}
