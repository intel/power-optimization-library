package power

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsScalingDriverSupported(t *testing.T) {
	assert.False(t, isScalingDriverSupported("something"))
	assert.True(t, isScalingDriverSupported("intel_pstate"))
	assert.True(t, isScalingDriverSupported("intel_cpufreq"))
	assert.True(t, isScalingDriverSupported("acpi-cpufreq"))
}
func TestPreChecksScalingDriver(t *testing.T) {
	var pStates featureStatus
	origpath := basePath
	basePath = ""
	pStates = initScalingDriver()

	assert.Equal(t, pStates.name, "Frequency-Scaling")
	assert.ErrorContains(t, pStates.err, "failed to determine driver")
	epp := initEpp()
	assert.Equal(t, epp.name, "Energy-Performance-Preference")
	assert.ErrorContains(t, epp.err, "EPP file cpufreq/energy_performance_preference does not exist")
	basePath = origpath
	teardown := setupCpuScalingTests(map[string]map[string]string{
		"cpu0": {
			"min":                 "111",
			"max":                 "999",
			"driver":              "intel_pstate",
			"available_governors": "performance",
			"epp":                 "performance",
		},
	})

	pStates = initScalingDriver()
	assert.Equal(t, "intel_pstate", pStates.driver)
	assert.NoError(t, pStates.err)
	epp = initEpp()
	assert.NoError(t, epp.err)

	teardown()
	defer setupCpuScalingTests(map[string]map[string]string{
		"cpu0": {
			"driver": "some_unsupported_driver",
		},
	})()

	pStates = initScalingDriver()
	assert.ErrorContains(t, pStates.err, "unsupported")
	assert.Equal(t, pStates.driver, "some_unsupported_driver")
	teardown()
	defer setupCpuScalingTests(map[string]map[string]string{
		"cpu0": {
			"driver":              "acpi-cpufreq",
			"available_governors": "powersave",
			"max":                 "3700",
			"min":                 "3200",
		},
	})()
	acpi := initScalingDriver()
	assert.Equal(t, "acpi-cpufreq", acpi.driver)
	assert.NoError(t, acpi.err)
}

func TestCoreImpl_updateFreqValues(t *testing.T) {
	var core *cpuImpl
	const (
		maxDefault   = 9990
		maxFreqToSet = 8888
		minFreqToSet = 1000
	)
	typeCopy := coreTypes
	coreTypes = CoreTypeList{&CpuFrequencySet{min: minFreqToSet, max: maxDefault}}
	defer func() { coreTypes = typeCopy }()

	core = &cpuImpl{}
	// p-states not supported
	assert.NoError(t, core.updateFrequencies())

	teardown := setupCpuScalingTests(map[string]map[string]string{
		"cpu0": {
			"max": fmt.Sprint(maxDefault),
			"min": fmt.Sprint(minFreqToSet),
		},
	})

	defer teardown()

	// set desired power profile
	host := new(hostMock)
	pool := new(poolMock)
	core = &cpuImpl{
		id:   0,
		pool: pool,
		core: &cpuCore{coreType: 0},
	}
	pool.On("GetPowerProfile").Return(&profileImpl{max: maxFreqToSet, min: minFreqToSet})
	pool.On("getHost").Return(host)
	host.On("NumCoreTypes").Return(uint(1))

	assert.NoError(t, core.updateFrequencies())
	maxFreqContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", scalingMaxFile))
	maxFreqInt, _ := strconv.Atoi(string(maxFreqContent))
	assert.Equal(t, maxFreqToSet, maxFreqInt)
	pool.AssertNumberOfCalls(t, "GetPowerProfile", 2)

	// set default power profile
	pool = new(poolMock)
	core.pool = pool
	pool.On("GetPowerProfile").Return(nil)
	pool.On("getHost").Return(host)
	assert.NoError(t, core.updateFrequencies())
	maxFreqContent, _ = os.ReadFile(filepath.Join(basePath, "cpu0", scalingMaxFile))
	maxFreqInt, _ = strconv.Atoi(string(maxFreqContent))
	assert.Equal(t, maxDefault, maxFreqInt)
	pool.AssertNumberOfCalls(t, "GetPowerProfile", 1)

}

func TestCoreImpl_setPstatsValues(t *testing.T) {
	const (
		maxFreqToSet  = 8888
		minFreqToSet  = 1111
		governorToSet = "powersave"
		eppToSet      = "testEpp"
	)
	featureList[FrequencyScalingFeature].err = nil
	featureList[EPPFeature].err = nil
	typeCopy := coreTypes
	coreTypes = CoreTypeList{&CpuFrequencySet{min: 1000, max: 9000}}
	defer func() { coreTypes = typeCopy }()
	defer func() { featureList[EPPFeature].err = uninitialisedErr }()
	defer func() { featureList[FrequencyScalingFeature].err = uninitialisedErr }()

	poolmk := new(poolMock)
	host := new(hostMock)
	poolmk.On("getHost").Return(host)
	host.On("NumCoreTypes").Return(uint(1))
	core := &cpuImpl{
		id:   0,
		core: &cpuCore{id: 0, coreType: 0},
		pool: poolmk,
	}

	teardown := setupCpuScalingTests(map[string]map[string]string{
		"cpu0": {
			"governor": "performance",
			"max":      "9999",
			"min":      "999",
			"epp":      "balance-performance",
		},
	})
	defer teardown()

	profile := &profileImpl{
		name:     "default",
		max:      maxFreqToSet,
		min:      minFreqToSet,
		epp:      eppToSet,
		governor: governorToSet,
	}
	assert.NoError(t, core.setDriverValues(profile))

	governorFileContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", scalingGovFile))
	assert.Equal(t, governorToSet, string(governorFileContent))

	eppFileContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", eppFile))
	assert.Equal(t, eppToSet, string(eppFileContent))

	maxFreqContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", scalingMaxFile))
	maxFreqInt, _ := strconv.Atoi(string(maxFreqContent))
	assert.Equal(t, maxFreqToSet, maxFreqInt)

	minFreqContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", scalingMaxFile))
	minFreqInt, _ := strconv.Atoi(string(minFreqContent))
	assert.Equal(t, maxFreqToSet, minFreqInt)

	// check for empty epp unset
	profile.epp = ""
	assert.NoError(t, core.setDriverValues(profile))
	eppFileContent, _ = os.ReadFile(filepath.Join(basePath, "cpu0", eppFile))
	assert.Equal(t, eppToSet, string(eppFileContent))
}
