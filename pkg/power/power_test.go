package power

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
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
	set[0] = &featureStatus{
		initFunc: func() featureStatus {
			called = true
			return featureStatus{err: fmt.Errorf("error")}
		},
	}
	assert.Equal(t, 1, set.init().Len())
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
	assert.ErrorIs(t, undefinedErr, set.getFeatureIdError(0))

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

//
//
//func Fuzz_library(f *testing.F) {
//	states := map[string]map[string]string{
//		"state0":   {"name": "C0"},
//		"state1":   {"name": "C1"},
//		"state2":   {"name": "C2"},
//		"state3":   {"name": "POLL"},
//		"notState": nil,
//	}
//	cstatesFiles := map[string]map[string]map[string]string{
//		"cpu0":   states,
//		"cpu1":   states,
//		"cpu2":   states,
//		"cpu3":   states,
//		"cpu4":   states,
//		"cpu5":   states,
//		"cpu6":   states,
//		"cpu7":   states,
//		"Driver": {"intel_idle\n": nil},
//	}
//
//	cpuFreqs := map[string]string{
//		"max": "123",
//		"min": "100",
//		"epp": "some",
//	}
//	cpuFreqsFiles := map[string]map[string]string{
//		"cpu0": cpuFreqs,
//		"cpu1": cpuFreqs,
//		"cpu2": cpuFreqs,
//		"cpu3": cpuFreqs,
//		"cpu4": cpuFreqs,
//		"cpu5": cpuFreqs,
//		"cpu6": cpuFreqs,
//		"cpu7": cpuFreqs,
//	}
//	setupCoreTests(cpuFreqsFiles)
//	setupCoreCStatesTests(cstatesFiles)
//
//	governorList := []string{"powersave", "performance"}
//	eppList := []string{"power", "performance", "balance-power", "balance-performance"}
//
//	fuzzTarget := func(t *testing.T, nodeName string, poolName string, value1 uint, value2 uint, governorSeed uint, eppSeed uint) {
//		basePath = "testing/cores"
//		getNumberOfCpus = func() int { return 8 }
//
//		if nodeName == "" {
//			return
//		}
//		node, err := CreateInstance(nodeName)
//
//		if err != nil || node == nil {
//			t.Fatal("node failed to init", err)
//		}
//
//		err = node.SetReservedPoolCores([]int{0})
//		if err != nil {
//			t.Error(err)
//		}
//		governor := governorList[int(governorSeed)%len(governorList)]
//		epp := eppList[int(eppSeed)%len(eppList)]
//		profile, err := node.AddProfile(poolName, int(value1), int(value2), governor, epp)
//
//		if err != nil {
//			return
//		}
//		switch profile.(type) {
//		default:
//			t.Error("profile is null")
//		case Profile:
//		}
//
//		if epp == "power" {
//			return
//		}
//
//		err = node.AddCoresToExclusivePool(poolName, []int{1, 3, 5})
//		if err != nil {
//			t.Error(err)
//		}
//
//		err = node.RemoveCoresFromExclusivePool(poolName, []int{3})
//		if err != nil {
//			t.Error(err)
//		}
//
//		err = node.GetExclusivePool(poolName).SetPowerProfile(nil)
//		if err != nil {
//			t.Error(err)
//		}
//	}
//	f.Fuzz(fuzzTarget)
//}
