package power

import (
	"github.com/pkg/errors"
)

// The nodeImpl object holds all the information regarding a node in a cluster.
type nodeImpl struct {
	Name           string
	ExclusivePools []Pool
	SharedPool     Pool
	allCores       []Core
	featureStates  FeatureSet
}

type Node interface {
	SetNodeName(name string)
	GetName() string
	GetReservedCoreIds() []int
	AddProfile(name string, minFreq int, maxFreq int, governor string, epp string) (Profile, error)
	UpdateProfile(name string, minFreq int, maxFreq int, governor string, epp string) error
	DeleteProfile(name string) error
	AddCoresToExclusivePool(name string, cores []int) error
	AddExclusivePool(name string, profile Profile) (Pool, error)
	RemoveExclusivePool(name string) error
	AddSharedPool(coreIds []int, profile Profile) error
	RemoveCoresFromExclusivePool(poolName string, cores []int) error
	RemoveSharedPool() error
	GetProfile(name string) Profile
	GetExclusivePool(name string) Pool
	GetSharedPool() Pool
	AvailableCStates() ([]string, error)
<<<<<<< HEAD
	ApplyCStatesToSharedPool(cStates map[string]bool) error
	ApplyCStateToPool(poolName string, cStates map[string]bool) error
	ApplyCStatesToCore(coreID int, cStates map[string]bool) error
=======
	ApplyCStatesToSharedPool(cStates CStates) error
	ApplyCStateToPool(poolName string, cStates CStates) error
	ApplyCStatesToCore(coreID int, cStates CStates) error
>>>>>>> internal/main
	IsCStateValid(cStates ...string) bool
	GetFeaturesInfo() FeatureSet
}

func (node *nodeImpl) SetNodeName(name string) {
	node.Name = name
}

func (node *nodeImpl) GetName() string {
	return node.Name
}

// AddProfile combines profile creation and pool creation into one call
func (node *nodeImpl) AddProfile(poolName string, minFreq int, maxFreq int, governor string, epp string) (Profile, error) {
	var err error
	if !IsFeatureSupported(PStatesFeature) {
		return nil, *supportedFeatureErrors[PStatesFeature]
	}
	profile, err := NewProfile(poolName, minFreq, maxFreq, governor, epp)
	if err != nil {
		return nil, err
	}
	if epp == "power" {
		err = node.SharedPool.SetPowerProfile(profile)
	} else {
		_, err = node.AddExclusivePool(poolName, profile)
	}
	return profile, errors.Wrap(err, "AddProfile")
}

// DeleteProfile will delete all pools and profiles with te passed Name
// if exclusive pool is being deleted, cores are moved to the shared pool
// ie shared pool is being deleted, cores are moved to the default pool
func (node *nodeImpl) DeleteProfile(name string) error {
	if !IsFeatureSupported(PStatesFeature) {
		return *supportedFeatureErrors[PStatesFeature]
	}
	found := false
	for _, pool := range node.ExclusivePools {
		if pool.GetPowerProfile() == nil {
			continue
		}
		if pool.GetPowerProfile().GetName() == name {
			found = true
			err := node.RemoveExclusivePool(pool.GetName())
			if err != nil {
				return errors.Wrap(err, "DeleteProfile")
			}
		}
	}
	if node.SharedPool.GetName() == name {
		found = true
		err := node.RemoveSharedPool()
		if err != nil {
			return errors.Wrap(err, "DeleteProfile")
		}
	}
	if !found {
		return errors.Errorf("DeleteProfile: profile with Name %s not found", name)
	}
	return nil
}

// AddExclusivePool creates new empty pool with attached profile attached
func (node *nodeImpl) AddExclusivePool(poolName string, profile Profile) (Pool, error) {
	if i := node.findExclusivePoolByName(poolName); i >= 0 {
		return node.ExclusivePools[i], errors.Errorf("pool with Name %s already exists", poolName)
	}
	var pool Pool = &poolImpl{
		Name:         poolName,
		Cores:        make([]Core, 0),
		PowerProfile: profile,
	}
	node.ExclusivePools = append(node.ExclusivePools, pool)
	return pool, nil
}

// RemoveExclusivePool removes an exclusive pool, any cores in the pool are moved back to shared pool, and updates cpu values
func (node *nodeImpl) RemoveExclusivePool(name string) error {
	index := node.findExclusivePoolByName(name)
	if index < 0 {
		return errors.New("pool with the Name is not tin the list")
	}
	err := node.RemoveCoresFromExclusivePool(name, node.ExclusivePools[index].GetCoreIds())
	if err != nil {
		return errors.Wrapf(err, "Remove exclusive pool")
	}

	return node.doRemoveExclusivePool(index)
}

func (node *nodeImpl) findExclusivePoolByName(name string) int {
	index := -1
	for i, v := range node.ExclusivePools {
		if v.GetName() == name {
			index = i
		}
	}
	return index
}

func (node *nodeImpl) doRemoveExclusivePool(index int) error {
	node.ExclusivePools[index] = node.ExclusivePools[len(node.ExclusivePools)-1]
	node.ExclusivePools = node.ExclusivePools[:len(node.ExclusivePools)-1]
	return nil
}

func (node *nodeImpl) initializeDefaultPool() error {
	cores, err := getAllCores()
	if err != nil {
		return errors.Wrap(err, "initDefaultPool")
	}
	pool := &poolImpl{
		Name:  sharedPoolName,
		Cores: cores,
	}
	for _, core := range cores {
		core.setPool(pool)
	}
	node.SharedPool = pool
	return nil
}

// AddSharedPool takes in list of cores which will be marked as reserved
// all remaining cores on the system will be placed in the reserved pool and their power profile will be applied
func (node *nodeImpl) AddSharedPool(coreIds []int, profile Profile) error {
	cores := make([]Core, len(coreIds))
	for i, coreID := range coreIds {
		for _, core := range node.SharedPool.GetCores() {
			if core.GetID() == coreID {
				cores[i] = core
				err := core.restoreFrequencies()
				if err != nil {
					return err
				}
				core.setReserved(true)
			}
		}
	}

	notReserved := diffCoreList(node.SharedPool.GetCores(), cores)
	for _, core := range notReserved {
		core.setReserved(false)
	}
	err := node.SharedPool.SetPowerProfile(profile)
	return errors.Wrap(err, "AddSharedPool")
}

// AddCoresToExclusivePool takes cores from the shared pool and moves them to the exclusive pool
// core values of the pool are applied
func (node *nodeImpl) AddCoresToExclusivePool(poolName string, coreIds []int) error {
	var exclPool Pool
	index := node.findExclusivePoolByName(poolName)
	if index < 0 {
		return errors.Errorf("pool with Name %s does not exist", poolName)
	}
	exclPool = node.ExclusivePools[index]
	for _, coreID := range coreIds {
		core, err := node.SharedPool.removeCoreByID(coreID)
		if err != nil {
			return errors.Wrap(err, "AddCoresToExclusivePool")
		}
		core.setReserved(false)
		err = exclPool.addCore(core)
		if err != nil {
			return errors.Wrap(err, "AddCoresToExclusivePool")
		}
	}
	return nil
}

// RemoveCoresFromExclusivePool removes cores from specified exclusive core and
// places them back in the shared pool, cpu values are reset to their default values
func (node *nodeImpl) RemoveCoresFromExclusivePool(poolName string, coreIds []int) error {
	index := node.findExclusivePoolByName(poolName)
	if index < 0 {
		return errors.Errorf("pool with Name %s doesnt exsit", poolName)
	}
	for _, coreID := range coreIds {
		core, err := node.ExclusivePools[index].removeCoreByID(coreID)
		if err != nil {
			return errors.Wrap(err, "RemoveCoresFromExclusivePool")
		}
		sharedProfile := node.GetSharedPool().GetPowerProfile()
		if sharedProfile == nil {
			core.setReserved(true)
		}
		err = node.SharedPool.addCore(core)
		if err != nil {
			return errors.Wrap(err, "RemoveCoresFromExclusivePool")
		}
	}
	return nil
}

// UpdateProfile updates all values set for cores attached to that profile
func (node *nodeImpl) UpdateProfile(name string, minFreq int, maxFreq int, governor string, epp string) error {
	if !IsFeatureSupported(PStatesFeature) {
		return *supportedFeatureErrors[PStatesFeature]
	}
	newProfile, err := NewProfile(name, minFreq, maxFreq, governor, epp)
	if err != nil {
		return err
	}

	for _, pool := range node.ExclusivePools {
		if pool.GetPowerProfile() == nil {
			continue
		}
		if pool.GetPowerProfile().GetName() == name {
			return pool.SetPowerProfile(newProfile)
		}
	}
	return errors.Errorf("pool with Name %s not found", name)
}

// GetReservedCoreIds returns a list of all core id that are reserved (in default Pool)
func (node *nodeImpl) GetReservedCoreIds() []int {
	reservedCoresIds := make([]int, 0)
	for _, core := range node.SharedPool.GetCores() {
		if core.getReserved() {
			reservedCoresIds = append(reservedCoresIds, core.GetID())
		}
	}
	return reservedCoresIds
}

// RemoveSharedPool marks all cpus that are in shared pool (i.e. are not in any exclusive pool) as system reserved
// profile attached to the shared pool is removed and all cpus are reverted to their default frequency values
// will not affect any exclusive pools
func (node *nodeImpl) RemoveSharedPool() error {
	err := node.SharedPool.SetPowerProfile(nil)
	for _, core := range node.SharedPool.GetCores() {
		core.setReserved(true)
	}
	return errors.Wrap(err, "RemoveSharedPool")
}

// GetProfile returns PowerProfile object attached to exclusive or shared pool with supplied Name
// returns nil if not found
func (node *nodeImpl) GetProfile(name string) Profile {
	if sharedProfile := node.SharedPool.GetPowerProfile(); sharedProfile != nil {
		if sharedProfile.GetName() == name {
			return sharedProfile
		}
	}
	for _, pool := range node.ExclusivePools {
		if pool.GetPowerProfile() == nil {
			continue
		}
		if pool.GetPowerProfile().GetName() == name {
			return pool.GetPowerProfile()
		}
	}
	return nil
}

// GetExclusivePool Returns a Pool object of the exclusive pool with matching Name supplied
// returns nil if not found
func (node *nodeImpl) GetExclusivePool(name string) Pool {
	for _, pool := range node.ExclusivePools {
		if pool.GetName() == name {
			return pool
		}
	}
	return nil
}

// GetSharedPool returns a Pool object containing PowerProfile, Name and Cores
// that are not system reserved
func (node *nodeImpl) GetSharedPool() Pool {
	var notReserved []Core
	for _, core := range node.SharedPool.GetCores() {
		if !core.getReserved() {
			notReserved = append(notReserved, core)
		}
	}
	return &poolImpl{
		Name:         sharedPoolName,
		Cores:        notReserved,
		PowerProfile: node.SharedPool.GetPowerProfile(),
	}
}

func (node *nodeImpl) AvailableCStates() ([]string, error) {
	if !IsFeatureSupported(CStatesFeature) {
		return nil, *supportedFeatureErrors[CStatesFeature]
	}
	cStatesList := make([]string, 0)
	for name := range cStatesNamesMap {
		cStatesList = append(cStatesList, name)
	}
	return cStatesList, nil
}
<<<<<<< HEAD
func (node *nodeImpl) ApplyCStatesToSharedPool(cStates map[string]bool) error {
	return node.SharedPool.SetCStates(cStates)
}

func (node *nodeImpl) ApplyCStateToPool(poolName string, cStates map[string]bool) error {
=======
func (node *nodeImpl) ApplyCStatesToSharedPool(cStates CStates) error {
	return node.SharedPool.SetCStates(cStates)
}

func (node *nodeImpl) ApplyCStateToPool(poolName string, cStates CStates) error {
>>>>>>> internal/main
	index := node.findExclusivePoolByName(poolName)
	if index < 0 {
		return errors.Errorf("pool with the Name %s does not exist", poolName)
	}
	return node.ExclusivePools[index].SetCStates(cStates)
}

func (node *nodeImpl) ApplyCStatesToCore(coreID int, cStates CStates) error {
	// we can expect this list to be ordered,
	// node.allCores[coreID] should be core object for the correct core
<<<<<<< HEAD
	core := node.allCores[coreID]
	if cStates == nil {
		core.setReserved(false)
		if core.getPool().getCStates() != nil {
			return core.applyCStates(core.getPool().getCStates())
		} else {
			return core.restoreDefaultCStates()
		}
	}
	core.setReserved(true)
	return node.allCores[coreID].applyCStates(cStates)
=======
	return node.allCores[coreID].ApplyExclusiveCStates(cStates)
>>>>>>> internal/main
}

func (node *nodeImpl) IsCStateValid(cStates ...string) bool {
	for _, cState := range cStates {
		if _, ok := cStatesNamesMap[cState]; !ok {
			return false
		}
	}
	return true
}

func (node *nodeImpl) GetFeaturesInfo() FeatureSet {
	return node.featureStates
}
