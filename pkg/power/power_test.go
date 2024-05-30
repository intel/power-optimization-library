package power

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeatureSet_init(t *testing.T) {

	assert.Error(t, (&FeatureSet{}).init())

	set := FeatureSet{}
	set[0] = &featureStatus{}

	// non-existing initFunc
	assert.Panics(t, func() { set.init() })

	// no error
	called := false
	set[0] = &featureStatus{
		initFunc: func() featureStatus {
			called = true
			return featureStatus{}
		},
	}
	assert.Empty(t, set.init())
	assert.True(t, called)

	// error
	called = false

	expectedFeatureError := fmt.Errorf("error")
	set[0] = &featureStatus{
		initFunc: func() featureStatus {
			called = true
			return featureStatus{err: expectedFeatureError}
		},
	}

	featureErr := set.init()
	assert.ErrorIs(t, featureErr, expectedFeatureError)
	assert.Len(t, featureErr.(interface{ Unwrap() []error }).Unwrap(), 1)
	assert.True(t, called)
}

func TestFeatureSet_anySupported(t *testing.T) {
	// empty set - nothing supported
	set := FeatureSet{}
	assert.False(t, set.anySupported())

	// something supported
	set[0] = &featureStatus{err: nil}
	assert.True(t, set.anySupported())

	//nothing supported
	set[0] = &featureStatus{err: fmt.Errorf("")}
	set[4] = &featureStatus{err: fmt.Errorf("")}
	set[2] = &featureStatus{err: fmt.Errorf("")}
	assert.False(t, set.anySupported())
}

func TestFeatureSet_isFeatureIdSupported(t *testing.T) {
	// non existing
	set := FeatureSet{}
	assert.False(t, set.isFeatureIdSupported(0))

	// error
	set[0] = &featureStatus{err: fmt.Errorf("")}
	assert.False(t, set.isFeatureIdSupported(0))

	// no error
	set[0] = &featureStatus{err: nil}
	assert.True(t, set.isFeatureIdSupported(0))
}

func TestFeatureSet_getFeatureIdError(t *testing.T) {
	// non existing
	set := FeatureSet{}
	assert.ErrorIs(t, undefinederr, set.getFeatureIdError(0))

	// error
	set[0] = &featureStatus{err: fmt.Errorf("")}
	assert.Error(t, set.getFeatureIdError(0))

	// no error
	set[0] = &featureStatus{err: nil}
	assert.NoError(t, set.getFeatureIdError(0))
}

func TestInitialFeatureList(t *testing.T) {
	assert.False(t, featureList.anySupported())

	for id, _ := range featureList {
		assert.ErrorIs(t, featureList.getFeatureIdError(id), uninitialisedErr)
	}
}

func TestCreateInstance(t *testing.T) {
	origFeatureList := featureList
	featureList = FeatureSet{}

	defer func() { featureList = origFeatureList }()

	const machineName = "host1"
	host, err := CreateInstance(machineName)
	assert.Nil(t, host)
	assert.Error(t, err)

	featureList[4] = &featureStatus{initFunc: func() featureStatus { return featureStatus{} }}
	host, err = CreateInstance(machineName)
	assert.NoError(t, err)
	assert.NotNil(t, host)

	hostObj := host.(*hostImpl)
	assert.Equal(t, machineName, hostObj.name)
}

func Fuzz_library(f *testing.F) {
	states := map[string]map[string]string{
		"state0":   {"name": "C0"},
		"state1":   {"name": "C1"},
		"state2":   {"name": "C2"},
		"state3":   {"name": "POLL"},
		"notState": nil,
	}
	cstatesFiles := map[string]map[string]map[string]string{
		"cpu0":   states,
		"cpu1":   states,
		"cpu2":   states,
		"cpu3":   states,
		"cpu4":   states,
		"cpu5":   states,
		"cpu6":   states,
		"cpu7":   states,
		"Driver": {"intel_idle\n": nil},
	}
	uncoreFreqs := map[string]string{
		"initMax": "200",
		"initMin": "100",
		"max":     "123",
		"min":     "100",
	}
	uncoreFiles := map[string]map[string]string{
		"package_00_die_00": uncoreFreqs,
		"package_01_die_00": uncoreFreqs,
	}
	cpuFreqs := map[string]string{
		"max":                 "123",
		"min":                 "100",
		"epp":                 "some",
		"driver":              "intel_pstate",
		"available_governors": "conservative ondemand userspace powersave",
		"package":             "0",
		"die":                 "0",
	}
	cpuFreqsFiles := map[string]map[string]string{
		"cpu0": cpuFreqs,
		"cpu1": cpuFreqs,
		"cpu2": cpuFreqs,
		"cpu3": cpuFreqs,
		"cpu4": cpuFreqs,
		"cpu5": cpuFreqs,
		"cpu6": cpuFreqs,
		"cpu7": cpuFreqs,
	}
	teardownCpu := setupCpuScalingTests(cpuFreqsFiles)
	teardownCstates := setupCpuCStatesTests(cstatesFiles)
	teardownUncore := setupUncoreTests(uncoreFiles, "intel_uncore_frequency 16384 0 - Live 0xffffffffc09c8000")
	defer teardownCpu()
	defer teardownCstates()
	defer teardownUncore()
	governorList := []string{"powersave", "performance"}
	eppList := []string{"power", "performance", "balance-power", "balance-performance"}
	f.Add("node1", "performance", uint(120000), uint(250000), uint(120000), uint(160000), uint(5), uint(10))
	fuzzTarget := func(t *testing.T, nodeName string, poolName string, min uint, max uint, emin uint, emax uint, governorSeed uint, eppSeed uint) {
		basePath = "testing/cpus"
		getNumberOfCpus = func() uint { return 8 }
		nodeName = strings.ReplaceAll(nodeName, " ", "")
		nodeName = strings.ReplaceAll(nodeName, "\t", "")
		nodeName = strings.ReplaceAll(nodeName, "\000", "")
		poolName = strings.ReplaceAll(poolName, " ", "")
		poolName = strings.ReplaceAll(poolName, "\t", "")
		poolName = strings.ReplaceAll(poolName, "\000", "")
		if nodeName == "" || poolName == "" {
			return
		}
		node, _ := CreateInstance(nodeName)

		if node == nil {
			return
		}
		node.GetReservedPool().MoveCpuIDs([]uint{0})
		governor := governorList[int(governorSeed)%len(governorList)]
		epp := eppList[int(eppSeed)%len(eppList)]
		pool, _ := node.AddExclusivePool(poolName)
		profile, _ := NewEcorePowerProfile(poolName, min, max, emin, emax, governor, epp)
		pool.SetPowerProfile(profile)
		pool.SetCStates(CStates{"C0": true, "C1": false})
		states := pool.getCStates()
		if states != nil {
			node.ValidateCStates(*states)
		}
		node.GetSharedPool().MoveCpuIDs([]uint{1, 3, 5})
		node.GetExclusivePool(poolName).MoveCpuIDs([]uint{1, 3, 5})
		node.GetSharedPool().MoveCpuIDs([]uint{3})
		node.GetExclusivePool(poolName).SetPowerProfile(nil)
		node.Topology().SetUncore(&uncoreFreq{max: 24000, min: 13000})
		node.Topology().Package(0).SetUncore(&uncoreFreq{max: 24000, min: 12000})
		node.Topology().Package(0).Die(0).SetUncore(&uncoreFreq{max: 23000, min: 11000})

	}
	f.Fuzz(fuzzTarget)

}
