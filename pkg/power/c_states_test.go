package power

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupCoreCStatesTests(cpufiles map[string]map[string]map[string]string) func() {
	origBasePath := basePath
	basePath = "testing/cores"

	origGetNumOfCpusFunc := getNumberOfCpus
	getNumberOfCpus = func() int {
		if _, ok := cpufiles["driver"]; ok {
			return len(cpufiles) - 1
		} else {
			return len(cpufiles)
		}
	}

	for cpu, states := range cpufiles {
		if cpu == "driver" {
			os.MkdirAll(filepath.Join(basePath, strings.Split(cStatesDrvPath, "/")[0]), os.ModePerm)
			for driver := range states {
				os.WriteFile(filepath.Join(basePath, cStatesDrvPath), []byte(driver), 0644)
				break
			}
			continue
		}
		cpuStatesDir := filepath.Join(basePath, cpu, cStatesDir)
		os.MkdirAll(filepath.Join(cpuStatesDir), os.ModePerm)
		for state, props := range states {
			os.Mkdir(filepath.Join(cpuStatesDir, state), os.ModePerm)
			for propFile, value := range props {
				os.WriteFile(filepath.Join(cpuStatesDir, state, propFile), []byte(value), 0644)
			}
		}
	}

	return func() {
		os.RemoveAll(strings.Split(basePath, "/")[0])
		basePath = origBasePath
		getNumberOfCpus = origGetNumOfCpusFunc
		cStatesNamesMap = map[string]int{}
	}
}

func Test_mapAvailableCStates(t *testing.T) {
	states := map[string]map[string]string{
		"state0":   {"name": "C0"},
		"state1":   {"name": "C1"},
		"state2":   {"name": "C2"},
		"state3":   {"name": "POLL"},
		"notState": nil,
	}
	cpufiles := map[string]map[string]map[string]string{
		"cpu0": states,
		"cpu1": states,
	}
	teardown := setupCoreCStatesTests(cpufiles)

	err := mapAvailableCStates()
	assert.NoError(t, err)

	assert.Equal(t, cStatesNamesMap, map[string]int{
		"C0":   0,
		"C1":   1,
		"C2":   2,
		"POLL": 3,
	})

	teardown()

	states["state0"] = nil
	teardown = setupCoreCStatesTests(cpufiles)

	err = mapAvailableCStates()

	assert.Error(t, err)

	teardown()

	states["state0"] = map[string]string{"name": "C0"}
	delete(cpufiles, "cpu0")
	teardown = setupCoreCStatesTests(cpufiles)

	assert.Error(t, mapAvailableCStates())
	teardown()
}

func TestCStatesSupportError_Error(t *testing.T) {
	err := &CStatesSupportError{"message"}
	assert.Equal(t, "C-States unsupported: message", err.Error())
}
func TestCStates_preCheckCStates(t *testing.T) {
	teardown := setupCoreCStatesTests(map[string]map[string]map[string]string{
		"driver": {"intel_idle\n": nil},
	})

	assert.Nil(t, preChecksCStates())
	teardown()

	teardown = setupCoreCStatesTests(map[string]map[string]map[string]string{
		"driver": {"unsupported something": nil},
	})
	err := preChecksCStates()
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "unsupported"))

	teardown()
}
