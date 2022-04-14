package power

import (
	"github.com/pkg/errors"
	"os"
	"runtime"
	"strconv"
	"strings"
)

var basePath = "/sys/devices/system/cpu"

const (
	cpuMaxFreqFile = "cpufreq/cpuinfo_max_freq"
	cpuMinFreqFile = "cpufreq/cpuinfo_min_freq"
	// used by profile and core
	scalingMaxFile = "cpufreq/scaling_max_freq"
	scalingMinFile = "cpufreq/scaling_min_freq"
	eppFile        = "cpufreq/energy_performance_preference"
	// baseFreqFile = "cpufreq/base_frequency"
	scalingDrvFile = "cpufreq/scaling_driver"
	sharedPoolName = "shared"
	defaultEpp     = "balance_performance"
)

// CreateInstance initialises the power library
// returns node object with empty list of exclusive pools, and a default pool containing all cpus
// by default all cpus are set to system reserved
func CreateInstance(nodeName string) (Node, error) {
	if err := preChecks(); err != nil {
		return nil, errors.Wrap(err, "preChecks")
	}
	if nodeName == "" {
		return nil, errors.Errorf("node name cannot be empty")
	}
	node := &nodeImpl{
		Name:           nodeName,
		ExclusivePools: make([]Pool, 0),
	}

	if err := node.initializeDefaultPool(); err != nil {
		return nil, errors.Wrap(err, "CreateInstance")
	}
	return node, nil
}

// getNumberOfCpus defined as var so can be mocked by the unit test
var getNumberOfCpus = runtime.NumCPU

// checks for "intel_pstate" driver required by power library
func preChecks() error {
	driver, err := readCoreStringProperty(0, scalingDrvFile)
	if driver != "intel_pstate" || err != nil {
		return errors.Errorf("failed to determine or unsupported driver")
	}
	return nil
}

// reads a file from a path, parses contents as an int a returns the value
// returns error if any step fails
func readIntFromFile(filePath string) (int, error) {
	valueString, err := readStringFromFile(filePath)
	if err != nil {
		return 0, errors.Wrap(err, "readIntFromFile")
	}
	valueString = strings.TrimSuffix(valueString, "\n")
	value, err := strconv.Atoi(valueString)
	if err != nil {
		return 0, errors.Wrap(err, "readIntFromFile")
	}
	return value, nil
}

// reads value from a file and returns contents as a string
func readStringFromFile(filePath string) (string, error) {
	valueByte, err := os.ReadFile(filePath)
	if err != nil {
		return "", errors.Wrap(err, "readStringFromFile")
	}
	return string(valueByte), nil
}
