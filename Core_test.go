package power

import (
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

func (m *coreMock) updateValues(epp string, maxFreq int, minFreq int) error {
	args := m.Called(epp, maxFreq, minFreq)
	return args.Error(0)
}

func (m *coreMock) GetID() int {
	args := m.Called()
	return args.Int(0)
}

func (m *coreMock) setReserved(reserved bool) {
	m.Called(reserved)
	return
}

func (m *coreMock) getReserved() bool {
	return m.Called().Bool(0)
}

func (m *coreMock) restoreValues() error {
	return m.Called().Error(0)
}

func setupCoreTests(cpufiles map[string]map[string]string) func() {
	origBasePath := basePath
	basePath = "testing/cores"

	origGetNumOfCpusFunc := getNumberOfCpus
	getNumberOfCpus = func() int { return len(cpufiles) }

	for cpuName, cpuDetails := range cpufiles {
		cpudir := filepath.Join(basePath, cpuName)
		os.MkdirAll(filepath.Join(cpudir, "cpufreq"), 644)
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
			}
		}
	}
	return func() {
		os.RemoveAll(strings.Split(basePath, "/")[0])
		basePath = origBasePath
		getNumberOfCpus = origGetNumOfCpusFunc
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
	//setupCoreTests(cpufiles)
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
	setupCoreTests(map[string]map[string]string{
		"cpu0": {
			"max": "9999",
			"min": "999",
			"epp": "some",
		},
	})
	//defer teardown()
	const (
		minFreqInit  = 999
		maxFreqInit  = 9999
		maxFreqToSet = 8888
		minFreqToSet = 1111
		eppToSet     = "testEpp"
	)

	core := &coreImpl{
		ID:                  0,
		MinimumFreq:         minFreqInit,
		MaximumFreq:         maxFreqInit,
		IsReservedSystemCPU: false,
	}
	assert.Nil(t, core.updateValues(eppToSet, minFreqToSet, maxFreqToSet))

	eppFileContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", eppFile))
	assert.Equal(t, eppToSet, string(eppFileContent))

	maxFreqContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", scalingMaxFile))
	maxFreqInt, _ := strconv.Atoi(string(maxFreqContent))
	assert.Equal(t, maxFreqToSet, maxFreqInt)
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

	assert.NoError(t, core.restoreValues())

	eppFileContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", eppFile))
	assert.Equal(t, defaultEpp, string(eppFileContent))

	maxFreqContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", scalingMaxFile))
	maxFreqInt, _ := strconv.Atoi(string(maxFreqContent))
	assert.Equal(t, 9999, maxFreqInt)
}
