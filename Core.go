package power

import (
	"fmt"
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
}

type Core interface {
	updateValues(epp string, minFreq int, maxFreq int) error
	GetID() int
	setReserved(reserved bool)
	getReserved() bool
	restoreValues() error
}

func newCore(coreID int) (Core, error) {
	minFreq, err := readCoreIntProperty(coreID, cpuMinFreqFile)
	if err != nil {
		return nil, errors.Wrapf(err, "newCore id: %d", coreID)
	}
	maxFreq, err := readCoreIntProperty(coreID, cpuMaxFreqFile)
	if err != nil {
		return nil, errors.Wrapf(err, "newCore id: %d", coreID)
	}

	return &coreImpl{
		ID:                  coreID,
		MinimumFreq:         minFreq,
		MaximumFreq:         maxFreq,
		IsReservedSystemCPU: true,
	}, nil
}
func (core *coreImpl) setReserved(reserved bool) {
	core.IsReservedSystemCPU = reserved
}
func (core *coreImpl) getReserved() bool {
	return core.IsReservedSystemCPU
}
func (core *coreImpl) GetID() int {
	return core.ID
}

func (core *coreImpl) updateValues(epp string, minFreq int, maxFreq int) error {
	if minFreq > maxFreq {
		return errors.New("minFreq cant be higher than maxFreq")
	}
	if core.IsReservedSystemCPU {
		return nil
	}
	err := core.writeEppValue(epp)
	if err != nil {
		return errors.Wrapf(err, "failed to set EPP value for core %d", core.ID)
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
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			// log
		}
	}(f)

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

func (core *coreImpl) restoreValues() error {
	return errors.Wrap(core.updateValues(defaultEpp, core.MinimumFreq, core.MaximumFreq), "restoreValues")
}

// Get the CPU max frequency from sysfs
//todo function closure to store result and read only form core 0 maybe?
func readCoreIntProperty(coreId int, file string) (int, error) {
	path := filepath.Join(basePath, fmt.Sprint("cpu", coreId), file)
	return readIntFromFile(path)
}

// reads content of a file and returns it as a string
func readCoreStringProperty(coreId int, file string) (string, error) {
	path := filepath.Join(basePath, fmt.Sprint("cpu", coreId), file)
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
		if err != nil {
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
