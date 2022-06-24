package power

import (
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

type poolImpl struct {
	Name           string
	Cores          []Core
	PowerProfile   Profile
	CStatesProfile map[string]bool
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
	SetCStates(states map[string]bool) error
	getCStates() map[string]bool
}

func (pool *poolImpl) GetName() string {
	return pool.Name
}

func (pool *poolImpl) GetPowerProfile() Profile {
	return pool.PowerProfile
}

func (pool *poolImpl) getCStates() map[string]bool {
	return pool.CStatesProfile
}

func (pool *poolImpl) addCore(core Core) error {
	for _, v := range pool.Cores {
		if v == core {
			return errors.Errorf("core %d already in the pool", core.GetID())
		}
	}
	if IsFeatureSupported(SSTBFFeature) {
		if pool.PowerProfile != nil {
			err := core.updateFreqValues(
				pool.PowerProfile.GetEpp(),
				pool.PowerProfile.GetMinFreq(),
				pool.PowerProfile.GetMaxFreq(),
			)
			if err != nil {
				return errors.Wrap(err, "SetPowerProfile")
			}
		} else {
			err := core.restoreFrequencies()
			if err != nil {
				return errors.Wrap(err, "SetPowerProfile")
			}
		}
		core.setPool(pool)
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
	if !IsFeatureSupported(SSTBFFeature) {
		return supportedFeatureErrors[SSTBFFeature]
	}
	pool.PowerProfile = profile
	if profile != nil {
		for _, core := range pool.Cores {
			if core.getReserved() {
				continue
			}
			err := core.updateFreqValues(
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
				err := core.restoreFrequencies()
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

func (pool *poolImpl) SetCStates(states map[string]bool) error {
	if !IsFeatureSupported(CStatesFeature) {
		return supportedFeatureErrors[CStatesFeature]
	}
	// check if requested states are on the system
	for name := range states {
		if _, exists := cStatesNamesMap[name]; !exists {
			return errors.Errorf("C-state %s does not exist on this system", name)
		}
	}
	pool.CStatesProfile = states
	var applyErrors = new(multierror.Error)
	for _, core := range pool.Cores {
		if core.exclusiveCStates() {
			continue
		}
		applyErrors = multierror.Append(applyErrors, core.applyCStates(states))
	}
	return applyErrors.ErrorOrNil()
}
