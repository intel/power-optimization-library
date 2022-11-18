package power

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"strings"
)

type coreImpl struct {
	ID                  int
	MinimumFreq         int
	MaximumFreq         int
	IsReservedSystemCPU bool
	pool                Pool
	hasExclusiveCStates bool
}

type Core interface {
	updateFreqValues(governor string, epp string, minFreq int, maxFreq int) error
	GetID() int
	setReserved(reserved bool)
	getReserved() bool
	restoreFrequencies() error
	applyCStates(desiredCStates CStates) error
	setPool(pool Pool)
	getPool() Pool
	exclusiveCStates() bool
	ApplyExclusiveCStates(cStates CStates) error
	restoreDefaultCStates() error
}

func newCore(coreID int) (Core, error) {
	core := &coreImpl{ID: coreID, IsReservedSystemCPU: true}

	if IsFeatureSupported(PStatesFeature) {
		// read sst-bf properties only if sst-bf is supported
		minFreq, err := readCoreIntProperty(coreID, cpuMinFreqFile)
		if err != nil {
			return nil, errors.Wrapf(err, "newCore id: %d", coreID)
		}
		core.MinimumFreq = minFreq

		maxFreq, err := readCoreIntProperty(coreID, cpuMaxFreqFile)
		if err != nil {
			return nil, errors.Wrapf(err, "newCore id: %d", coreID)
		}
		core.MaximumFreq = maxFreq
	}

	return core, *supportedFeatureErrors[PStatesFeature]
}
func (core *coreImpl) setReserved(reserved bool) {
	core.IsReservedSystemCPU = reserved
}
func (core *coreImpl) getReserved() bool {
	return core.IsReservedSystemCPU
}
func (core *coreImpl) setPool(pool Pool) {
	core.pool = pool
}
func (core *coreImpl) getPool() Pool {
	return core.pool
}

func (core *coreImpl) GetID() int {
	return core.ID
}

func (core *coreImpl) exclusiveCStates() bool {
	return core.hasExclusiveCStates
}

func (core *coreImpl) restoreDefaultCStates() error {
	defaultCStates := CStates{}
	// enable all the c
	for stateName := range cStatesNamesMap {
		defaultCStates[stateName] = true
	}
	return core.applyCStates(defaultCStates)
}
func (core *coreImpl) updateFreqValues(governor string, epp string, minFreq int, maxFreq int) error {
	if !IsFeatureSupported(PStatesFeature) {
		return *supportedFeatureErrors[PStatesFeature]
	}
	var err error
	if minFreq > maxFreq {
		return errors.New("minFreq cant be higher than maxFreq")
	}
	if core.IsReservedSystemCPU {
		return nil
	}
	err = core.writeGovernorValue(governor)
	if err != nil {
		return err
	}
	if epp != "" {
		err = core.writeEppValue(epp)
		if err != nil {
			return errors.Wrapf(err, "failed to set EPP value for core %d", core.ID)
		}
	}
	err = core.writeScalingMaxFreq(maxFreq)
	if err != nil {
		return errors.Wrapf(err, "failed to set MaxFreq value for core %d", core.ID)
	}
	err = core.writeScalingMinFreq(minFreq)
	if err != nil {
		return errors.Wrapf(err, "failed to set MinFreq value for core %d", core.ID)
	}
	return nil
}

func (core *coreImpl) writeEppValue(eppValue string) error {
	return os.WriteFile(filepath.Join(basePath, fmt.Sprint("cpu", core.ID), eppFile), []byte(eppValue), 0644)
}

func (core *coreImpl) writeGovernorValue(eppValue string) error {
	return os.WriteFile(filepath.Join(basePath, fmt.Sprint("cpu", core.ID), scalingGovFile), []byte(eppValue), 0644)
}

func (core *coreImpl) writeScalingMaxFreq(freq int) error {
	if freq > core.MaximumFreq {
		freq = core.MaximumFreq
	}
	scalingFile := filepath.Join(basePath, fmt.Sprint("cpu", core.ID), scalingMaxFile)
	f, err := os.OpenFile(
		scalingFile,
		os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
		0644,
	)
	if err != nil {
		return errors.Wrap(err, "writeScalingMaxFreq")
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprint(freq))
	if err != nil {
		return errors.Wrap(err, "writeScalingMaxFreq")
	}
	return nil
}

func (core *coreImpl) writeScalingMinFreq(freq int) error {
	if freq < core.MinimumFreq {
		freq = core.MinimumFreq
	}
	scalingFile := filepath.Join(basePath, fmt.Sprint("cpu", core.ID), scalingMinFile)
	f, err := os.OpenFile(
		scalingFile,
		os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
		0644,
	)
	if err != nil {
		return errors.Wrap(err, "writeScalingMinFreq")
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprint(freq))
	if err != nil {
		return errors.Wrap(err, "writeScalingMinFreq")
	}
	return nil
}

func (core *coreImpl) restoreFrequencies() error {
	return errors.Wrap(core.updateFreqValues(defaultGovernor, defaultEpp, core.MinimumFreq, core.MaximumFreq), "restoreFrequencies")
}

func (core *coreImpl) applyCStates(desiredCStates CStates) error {
	if desiredCStates == nil {
		return core.restoreDefaultCStates()
	}
	var applyingErrors *multierror.Error
	for state, enabled := range desiredCStates {
		stateFilePath := filepath.Join(
			basePath,
			fmt.Sprint("cpu", core.ID),
			fmt.Sprintf(cStateDisableFileFmt, cStatesNamesMap[state]),
		)
		content := make([]byte, 1)
		if enabled {
			content[0] = '0' // write '0' to enable the c state
		} else {
			content[0] = '1' // write '1' to disable the c state
		}
		applyingErrors = multierror.Append(applyingErrors,
			errors.Wrapf(os.WriteFile(stateFilePath, content, 0644), "CState %s, core %d", state, core.ID))
	}
	return applyingErrors.ErrorOrNil()
}

func (core *coreImpl) ApplyExclusiveCStates(cStates CStates) error {
	// wipe core specific configuration and apply pool one or default one
	if cStates == nil {
		if err := core.applyCStates(core.pool.getCStates()); err != nil {
			return err
		}
		core.hasExclusiveCStates = false
	} else {
		if err := core.applyCStates(cStates); err != nil {
			return err
		}
		core.hasExclusiveCStates = true
	}
	return nil
}

// Get the CPU max frequency from sysfs
func readCoreIntProperty(coreID int, file string) (int, error) {
	path := filepath.Join(basePath, fmt.Sprint("cpu", coreID), file)
	return readIntFromFile(path)
}

// reads content of a file and returns it as a string
func readCoreStringProperty(coreID int, file string) (string, error) {
	path := filepath.Join(basePath, fmt.Sprint("cpu", coreID), file)
	value, err := readStringFromFile(path)
	if err != nil {
		//log
		return "", err
	}
	value = strings.TrimSuffix(value, "\n")
	return value, nil
}

func getAllCores() ([]Core, error) {
	num := getNumberOfCpus()
	cores := make([]Core, num)
	for i := 0; i < num; i++ {
		core, err := newCore(i)
		if err != nil && !errors.Is(err, *supportedFeatureErrors[PStatesFeature]) {
			return nil, errors.Wrap(err, "getAllCores")
		}
		cores[i] = core
	}
	return cores, nil
}

// TODO this probably badly needs optimizing
// returns a list of cores that are in the first list but are not present in the excluded list
func diffCoreList(all []Core, excluded []Core) []Core {
	var diffCores = make([]Core, 0)
	for _, s1 := range all {
		found := false
		for _, s2 := range excluded {
			if s1.GetID() == s2.GetID() {
				found = true
				break
			}
		}
		if !found {
			diffCores = append(diffCores, s1)
		}
	}
	return diffCores
}
