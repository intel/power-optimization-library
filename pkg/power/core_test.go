package power

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type coreMock struct {
	mock.Mock
}

func (m *coreMock) updateFreqValues(governor string, epp string, maxFreq int, minFreq int) error {
	args := m.Called(governor, epp, maxFreq, minFreq)
	return args.Error(0)
}

func (m *coreMock) GetID() int {
	args := m.Called()
	return args.Int(0)
}

func (m *coreMock) setReserved(reserved bool) {
	m.Called(reserved)
}

func (m *coreMock) getReserved() bool {
	return m.Called().Bool(0)
}

func (m *coreMock) getPool() Pool {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	} else {
		return args.(Pool)
	}
}
func (m *coreMock) setPool(pool Pool) {
	m.Called(pool)
}

func (m *coreMock) applyCStates(states CStates) error {
	return m.Called(states).Error(0)
}
func (m *coreMock) exclusiveCStates() bool {
	return m.Called().Bool(0)
}
func (m *coreMock) ApplyExclusiveCStates(cStates CStates) error {
	return m.Called(cStates).Error(0)
}

func (m *coreMock) restoreFrequencies() error {
	return m.Called().Error(0)
}

func (m *coreMock) restoreDefaultCStates() error {
	return m.Called().Error(0)
}

func setupCoreTests(cpufiles map[string]map[string]string) func() {
	origBasePath := basePath
	basePath = "testing/cores"
	var nilError error
	supportedFeatureErrors[PStatesFeature] = &nilError

	origGetNumOfCpusFunc := getNumberOfCpus
	getNumberOfCpus = func() int { return len(cpufiles) }

	for cpuName, cpuDetails := range cpufiles {
		cpudir := filepath.Join(basePath, cpuName)
		os.MkdirAll(filepath.Join(cpudir, "cpufreq"), os.ModePerm)
		os.WriteFile(filepath.Join(cpudir, scalingDrvFile), []byte("intel_pstate\n"), 0664)
		for prop, value := range cpuDetails {
			switch prop {
			case "max":
				os.WriteFile(filepath.Join(cpudir, scalingMaxFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(cpudir, cpuMaxFreqFile), []byte(value+"\n"), 0644)
			case "min":
				os.WriteFile(filepath.Join(cpudir, scalingMinFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(cpudir, cpuMinFreqFile), []byte(value+"\n"), 0644)
			case "epp":
				os.WriteFile(filepath.Join(cpudir, eppFile), []byte(value+"\n"), 0644)
			case "governor":
				os.WriteFile(filepath.Join(cpudir, scalingGovFile), []byte(value+"\n"), 0644)
			}
		}
	}
	return func() {
		os.RemoveAll(strings.Split(basePath, "/")[0])
		basePath = origBasePath
		getNumberOfCpus = origGetNumOfCpusFunc
		supportedFeatureErrors[PStatesFeature] = &uninitialisedErr
	}
}

func TestNewCore(t *testing.T) {
	cpufiles := map[string]map[string]string{
		"cpu0": {
			"max": "123",
			"min": "100",
			"epp": "some",
		},
	}
	defer setupCoreTests(cpufiles)()

	core, err := newCore(0)
	assert.NoError(t, err)

	// happy path - ensure values from files are read correctly
	assert.Equal(t, &coreImpl{
		ID:                  0,
		MinimumFreq:         100,
		MaximumFreq:         123,
		IsReservedSystemCPU: true,
	}, core)

	uninit := errors.New("sstbf not workin")
	supportedFeatureErrors[PStatesFeature] = &uninit
	core, err = newCore(1)

	assert.Equal(t, core, &coreImpl{ID: 1, IsReservedSystemCPU: true})
	assert.ErrorIs(t, err, *supportedFeatureErrors[PStatesFeature])
}

func TestCoreImpl_setGet(t *testing.T) {
	core := &coreImpl{
		ID:                  0,
		IsReservedSystemCPU: false,
	}
	assert.False(t, core.getReserved())

	core.setReserved(true)
	assert.True(t, core.IsReservedSystemCPU)

	assert.Zero(t, core.GetID())

}
func TestCoreImpl_updateValues(t *testing.T) {
	teardown := setupCoreTests(map[string]map[string]string{
		"cpu0": {
			"max": "9999",
			"min": "999",
			"epp": "some",
		},
	})
	defer teardown()
	const (
		minFreqInit   = 999
		maxFreqInit   = 9999
		maxFreqToSet  = 8888
		minFreqToSet  = 1111
		governorToSet = "powersave"
		eppToSet      = "testEpp"
	)

	core := &coreImpl{
		ID:                  0,
		MinimumFreq:         minFreqInit,
		MaximumFreq:         maxFreqInit,
		IsReservedSystemCPU: false,
	}
	assert.Nil(t, core.updateFreqValues(governorToSet, eppToSet, minFreqToSet, maxFreqToSet))

	eppFileContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", eppFile))
	assert.Equal(t, eppToSet, string(eppFileContent))

	maxFreqContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", scalingMaxFile))
	maxFreqInt, _ := strconv.Atoi(string(maxFreqContent))
	assert.Equal(t, maxFreqToSet, maxFreqInt)

	core.updateFreqValues(governorToSet, "", minFreqToSet, maxFreqToSet)
	eppFileContent, _ = os.ReadFile(filepath.Join(basePath, "cpu0", eppFile))
	assert.Equal(t, eppToSet, string(eppFileContent))

	unsupp := errors.New("not supported")
	supportedFeatureErrors[PStatesFeature] = &unsupp
	assert.ErrorIs(t, core.updateFreqValues(governorToSet, "", 0, 0), *supportedFeatureErrors[PStatesFeature])
}

func TestCoreImpl_restoreValues(t *testing.T) {
	teardown := setupCoreTests(map[string]map[string]string{
		"cpu0": {
			"max": "123",
			"min": "100",
			"epp": "some",
		},
	})
	defer teardown()

	core := coreImpl{
		ID:                  0,
		MinimumFreq:         1000,
		MaximumFreq:         9999,
		IsReservedSystemCPU: false,
	}

	assert.NoError(t, core.restoreFrequencies())

	eppFileContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", eppFile))
	assert.Equal(t, defaultEpp, string(eppFileContent))

	maxFreqContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", scalingMaxFile))
	maxFreqInt, _ := strconv.Atoi(string(maxFreqContent))
	assert.Equal(t, 9999, maxFreqInt)
}

func TestCoreImpl_getAllCores(t *testing.T) {
	teardown := setupCoreTests(map[string]map[string]string{
		"cpu0": {
			"max": "123",
			"min": "100",
		}, "cpu1": {
			"max": "123",
			"min": "100",
		},
	})
	defer teardown()

	cores, err := getAllCores()
	assert.NoError(t, err)

	assert.Len(t, cores, 2)
	assert.Equal(t, cores[0], &coreImpl{
		ID:                  0,
		MinimumFreq:         100,
		MaximumFreq:         123,
		IsReservedSystemCPU: true,
	})

	error := errors.New("")
	supportedFeatureErrors[PStatesFeature] = &error
	cores, err = getAllCores()
	assert.NoError(t, err)

	assert.Len(t, cores, 2)
	assert.Equal(t, cores[0], &coreImpl{
		ID:                  0,
		MinimumFreq:         0,
		MaximumFreq:         0,
		IsReservedSystemCPU: true,
	})

}

func TestCoreImpl_applyCStates(t *testing.T) {
	states := map[string]map[string]string{
		"state0": {"name": "C0", "disable": "0"},
		"state2": {"name": "C2", "disable": "0"},
	}
	cpufiles := map[string]map[string]map[string]string{
		"cpu0": states,
	}
	defer setupCoreCStatesTests(cpufiles)()
	mapAvailableCStates()
	err := (&coreImpl{ID: 0}).applyCStates(CStates{
		"C0": false,
		"C2": true})

	assert.NoError(t, err)

	stateFilePath := filepath.Join(
		basePath,
		fmt.Sprint("cpu", 0),
		fmt.Sprintf(cStateDisableFileFmt, 0),
	)
	disabled, _ := readStringFromFile(stateFilePath)
	assert.Equal(t, "1", disabled)

	stateFilePath = filepath.Join(
		basePath,
		fmt.Sprint("cpu", 0),
		fmt.Sprintf(cStateDisableFileFmt, 2),
	)
	disabled, _ = readStringFromFile(stateFilePath)
	assert.Equal(t, "0", disabled)
}

func TestCoreImpl_ApplyExclusiveCStates(t *testing.T) {
	states := map[string]map[string]string{
		"state0": {"name": "C0", "disable": "0"},
		"state2": {"name": "C2", "disable": "0"},
	}
	cpufiles := map[string]map[string]map[string]string{
		"cpu0": states,
	}
	defer setupCoreCStatesTests(cpufiles)()
	mapAvailableCStates()

	stateFilePath := filepath.Join(
		basePath,
		fmt.Sprint("cpu", 0),
		fmt.Sprintf(cStateDisableFileFmt, 2),
	)

	// setting core specific c states
	core := &coreImpl{ID: 0}
	err := core.ApplyExclusiveCStates(CStates{
		"C2": false})

	assert.NoError(t, err)

	assert.True(t, core.hasExclusiveCStates)

	disabled, _ := readStringFromFile(stateFilePath)
	assert.Equal(t, "1", disabled)

	// unset core specific states with no c-states set on pool
	// should restore defaults - all states enabled
	core.pool = &poolImpl{}
	err = core.ApplyExclusiveCStates(nil)

	assert.NoError(t, err)
	assert.False(t, core.hasExclusiveCStates)

	disabled, _ = readStringFromFile(stateFilePath)
	assert.Equal(t, "0", disabled)

	// unset core states with existing states in the pool
	// should set pool values
	core.pool.(*poolImpl).CStatesProfile = CStates{"C2": false}
	err = core.ApplyExclusiveCStates(nil)

	assert.NoError(t, err)
	assert.False(t, core.hasExclusiveCStates)

	disabled, _ = readStringFromFile(stateFilePath)
	assert.Equal(t, "1", disabled)
}
