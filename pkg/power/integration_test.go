// this file contains integration tests pof the power library
package power

import (
	"errors"
	"fmt"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// this test checks for potential race condition where one go routine moves cpus to a pool and another changes a power
// profile of the target pool
func TestConcurrentMoveCpusSetProfile(t *testing.T) {
	typecopy := CpuTypeReferences
	const count = 5
	for i := 0; i < count; i++ {
		doConcurrentMoveCPUSetProfile(t)
	}
	// reset feature list
	for _, status := range featureList {
		status.err = uninitialisedErr
	}
	CpuTypeReferences = typecopy

}

func doConcurrentMoveCPUSetProfile(t *testing.T) {
	const numCpus = 88
	emin := "11100"
	emax := "5550000"
	cpuConfig := map[string]string{
		"min":                 "11100",
		"max":                 "9990000",
		"driver":              "intel_pstate",
		"available_governors": "performance",
		"epp":                 "performance",
	}

	ecoreConfig := map[string]string{}
	maps.Copy(ecoreConfig, cpuConfig)
	ecoreConfig["min"] = emin
	ecoreConfig["max"] = emax

	cpuConfigAll := map[string]map[string]string{}

	cpuTopologyMap := map[string]map[string]string{}
	for i := 0; i < numCpus; i++ {
		// set e cores
		if i > numCpus/2 {
			cpuConfigAll[fmt.Sprint("cpu", i)] = ecoreConfig
		} else {
			// set p cores
			cpuConfigAll[fmt.Sprint("cpu", i)] = cpuConfig
		}
		// for this test we don't care about topology, so we just emulate 1 pkg, 1 die, numCpus cores, no hyperthreading
		cpuTopologyMap[fmt.Sprint("cpu", i)] = map[string]string{
			"pkg":  "0",
			"die":  "0",
			"core": fmt.Sprint(i),
		}
	}
	defer setupCpuCStatesTests(map[string]map[string]map[string]string{})()
	defer setupUncoreTests(map[string]map[string]string{}, "")()
	defer setupCpuScalingTests(cpuConfigAll)()
	defer setupTopologyTest(cpuTopologyMap)()

	instance, err := CreateInstance("host")

	assert.ErrorContainsf(t, err, "failed to determine driver", "expecting c-states feature error")
	assert.ErrorContainsf(t, err, "intel_uncore_frequency not loaded", "expecting uncore feature error")
	assert.NotNil(t, instance)

	assert.Len(t, *instance.GetAllCpus(), numCpus)
	assert.ElementsMatch(t, *instance.GetReservedPool().Cpus(), *instance.GetAllCpus())
	assert.Empty(t, *instance.GetSharedPool().Cpus())

	profile, err := NewEcorePowerProfile("pwr", 100, 1000, 100, 500, "performance", "performance")
	assert.NoError(t, err)

	moveCoresErrChan := make(chan error)
	setPowerProfileErrChan2 := make(chan error)

	go func(instance Host, errChannel chan error) {
		errChannel <- instance.GetSharedPool().MoveCpus(*instance.GetAllCpus())
	}(instance, moveCoresErrChan)

	go func(instance Host, profile Profile, errChannel chan error) {
		time.Sleep(5 * time.Millisecond)
		errChannel <- instance.GetSharedPool().SetPowerProfile(profile)
	}(instance, profile, setPowerProfileErrChan2)

	assert.NoError(t, <-moveCoresErrChan)
	close(moveCoresErrChan)

	assert.NoError(t, <-setPowerProfileErrChan2)
	close(setPowerProfileErrChan2)

	assert.Equal(t, profile, instance.GetSharedPool().GetPowerProfile())
	assert.ElementsMatch(t, *instance.GetAllCpus(), *instance.GetSharedPool().Cpus())
	for i := uint(0); i < numCpus; i++ {
		assert.NoError(t, verifyPowerProfile(i, profile), "cpuid", i)
	}
}

// verifies that the cpu is configured correctly
// checking is done relative to basePath
func verifyPowerProfile(cpuId uint, profile Profile) error {
	var allerrs []error
	var err error

	governor, err := readCpuStringProperty(cpuId, scalingGovFile)
	allerrs = append(allerrs, err)
	if governor != profile.Governor() {
		allerrs = append(allerrs, fmt.Errorf("governor mismatch expected : %s, current %s", profile.Governor(), governor))
	}

	if profile.Epp() != "" {
		epp, err := readCpuStringProperty(cpuId, eppFile)
		allerrs = append(allerrs, err)
		if governor != profile.Epp() {
			allerrs = append(allerrs, fmt.Errorf("epp mismatch expected : %s, current %s", profile.Epp(), epp))
		}
	}

	maxFreq, err := readCpuUintProperty(cpuId, scalingMaxFile)
	allerrs = append(allerrs, err)
	if maxFreq != profile.MaxFreq() && maxFreq != profile.EfficientMaxFreq() {
		allerrs = append(allerrs, fmt.Errorf("maxFreq mismatch expected %d, current %d", profile.MaxFreq(), maxFreq))
	}
	minFreq, err := readCpuUintProperty(cpuId, scalingMinFile)
	allerrs = append(allerrs, err)
	if minFreq != profile.MinFreq() && minFreq != profile.EfficientMinFreq() {
		allerrs = append(allerrs, fmt.Errorf("minFreq mismatch expected %d, current %d", profile.MinFreq(), minFreq))
	}
	return errors.Join(allerrs...)
}
