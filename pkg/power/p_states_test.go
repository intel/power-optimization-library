package power

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestIsPStatesDriverSupported(t *testing.T) {
	assert.False(t, isPStatesDriverSupported("something"))
	assert.True(t, isPStatesDriverSupported("intel_pstate"))
	assert.True(t, isPStatesDriverSupported("intel_cpufreq"))
}
func TestPreChecksPStates(t *testing.T) {
	var pStates featureStatus
	basePath = ""
	pStates = initPStates()

	assert.Equal(t, pStates.name, "P-States")
	assert.ErrorContains(t, pStates.err, "failed to determine driver")

	setupCoreTests(map[string]map[string]string{
		"cpu0": {
			"min":    "111",
			"max":    "999",
			"driver": "intel_pstate",
		},
	})

	pStates = initPStates()
	assert.Equal(t, "intel_pstate", pStates.driver)
	assert.NoError(t, pStates.err)

	defer setupCoreTests(map[string]map[string]string{
		"cpu0": {
			"driver": "some_unsupported_driver",
		},
	})()

	pStates = initPStates()
	assert.ErrorContains(t, pStates.err, "unsupported")
	assert.Equal(t, pStates.driver, "some_unsupported_driver")
}

func TestCoreImpl_updateFreqValues(t *testing.T) {
	var core *coreImpl
	const (
		maxDefault   = 9990
		maxFreqToSet = 8888
	)

	core = &coreImpl{}
	// p-states not supported
	assert.NoError(t, core.updateFrequencies())

	teardown := setupCoreTests(map[string]map[string]string{
		"cpu0": {
			"max": fmt.Sprint(maxDefault),
		},
	})
	defer teardown()

	// set desired power profile
	pool := new(poolMock)
	core = &coreImpl{
		id:   0,
		pool: pool,
	}
	pool.On("GetPowerProfile").Return(&profileImpl{max: maxFreqToSet})
	assert.NoError(t, core.updateFrequencies())
	maxFreqContent, _ := os.ReadFile(filepath.Join(basePath, "cpu0", scalingMaxFile))
	maxFreqInt, _ := strconv.Atoi(string(maxFreqContent))
	assert.Equal(t, maxFreqToSet, maxFreqInt)
	pool.AssertNumberOfCalls(t, "GetPowerProfile", 2)

	// set default power profile
	pool = new(poolMock)
	core.pool = pool
	pool.On("GetPowerProfile").Return(nil)
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
	core := &coreImpl{
		id: 0,
	}

	teardown := setupCoreTests(map[string]map[string]string{
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
	assert.NoError(t, core.setPStatesValues(profile))

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
	assert.NoError(t, core.setPStatesValues(profile))
	eppFileContent, _ = os.ReadFile(filepath.Join(basePath, "cpu0", eppFile))
	assert.Equal(t, eppToSet, string(eppFileContent))

	// error when cant write
	teardown()
	assert.ErrorContains(t, core.setPStatesValues(profile), "no such file")
}
