package power

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func Test_diff(t *testing.T) {
	all := make([]Core, 8)
	for i := range all {
		mck := new(coreMock)
		mck.On("GetID").Return(i)
		all[i] = mck
	}
	excluded := []Core{
		all[1], all[3], all[5],
	}

	difference := diffCoreList(all, excluded)
	assert.ElementsMatch(t, difference, []Core{all[0], all[2], all[4], all[6], all[7]})
}

func TestCreateInstance(t *testing.T) {
	nodeName := "node1"
	mockCpuData := map[string]string{
		"min": "100",
		"max": "123",
		"epp": "epp",
	}
	mockedCores := map[string]map[string]string{
		"cpu0": mockCpuData,
		"cpu1": mockCpuData,
		"cpu2": mockCpuData,
		"cpu3": mockCpuData,
	}
	defer setupCoreTests(mockedCores)()

	node, err := CreateInstance(nodeName)
	var cStatesSupportError *CStatesSupportError
	assert.ErrorAs(t, err, &cStatesSupportError)

	assert.Equal(t, nodeName, node.GetName())
	assert.Len(t, node.(*nodeImpl).SharedPool.(*poolImpl).Cores, len(mockedCores))
	assert.Equal(t, sharedPoolName, node.(*nodeImpl).SharedPool.(*poolImpl).Name)

	assert.Empty(t, node.(*nodeImpl).ExclusivePools)
}

func TestPreChecks(t *testing.T) {
	defer setupCoreTests(map[string]map[string]string{
		"cpu0": {},
	})()
	_, err := preChecks()
	var cStateErr *CStatesSupportError
	assert.ErrorAs(t, err, &cStateErr)

	var sstBfErr *PStatesSupportError
	if errors.As(err, &sstBfErr) {
		assert.False(t, true, "unexpected Error type", err)
	}

	os.WriteFile(filepath.Join(basePath, "cpu0", scalingDrvFile), []byte("not intel\n"), 0664)

	_, err = preChecks()
	assert.Error(t, err)
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

	cpuFreqs := map[string]string{
		"max": "123",
		"min": "100",
		"epp": "some",
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
	setupCoreTests(cpuFreqsFiles)
	setupCoreCStatesTests(cstatesFiles)

	governorList := []string{"powersave", "performance"}
	eppList := []string{"power", "performance", "balance-power", "balance-performance"}

	fuzzTarget := func(t *testing.T, nodeName string, poolName string, value1 uint, value2 uint, governorSeed uint, eppSeed uint) {
		basePath = "testing/cores"
		getNumberOfCpus = func() int { return 8 }

		if nodeName == "" {
			return
		}
		node, err := CreateInstance(nodeName)

		if err != nil || node == nil {
			t.Fatal("node failed to init", err)
		}

		err = node.AddSharedPool([]int{0}, nil)
		if err != nil {
			t.Error(err)
		}
		governor := governorList[int(governorSeed)%len(governorList)]
		epp := eppList[int(eppSeed)%len(eppList)]
		profile, err := node.AddProfile(poolName, int(value1), int(value2), governor, epp)

		if err != nil {
			return
		}
		switch profile.(type) {
		default:
			t.Error("profile is null")
		case Profile:
		}

		if epp == "power" {
			return
		}

		err = node.AddCoresToExclusivePool(poolName, []int{1, 3, 5})
		if err != nil {
			t.Error(err)
		}

		err = node.RemoveCoresFromExclusivePool(poolName, []int{3})
		if err != nil {
			t.Error(err)
		}

		err = node.GetExclusivePool(poolName).SetPowerProfile(nil)
		if err != nil {
			t.Error(err)
		}
	}
	f.Fuzz(fuzzTarget)
}
