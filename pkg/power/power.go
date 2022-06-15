package power

import (
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const (
	SSTBFFeature = iota
	CStatesFeature
)

var basePath = "/sys/devices/system/cpu"

var uninitialisedErr = errors.New("uninitialised")

var supportedFeatureErrors = map[int]error{
	SSTBFFeature:   uninitialisedErr,
	CStatesFeature: uninitialisedErr,
}

// CreateInstance initialises the power library
// returns node object with empty list of exclusive pools, and a default pool containing all cpus
// by default all cpus are set to system reserved
func CreateInstance(nodeName string) (Node, error) {
	// TODO this will need to change in manager
	var allErrors *multierror.Error
	checks := preChecks()
	// if more or equal errors than supported features has occurred
	if checks.Len() >= len(supportedFeatureErrors) {
		return nil, errors.Wrap(checks, "preChecks")
	}
	allErrors = multierror.Append(allErrors, errors.Wrap(checks.ErrorOrNil(), "preChecks"))

	if nodeName == "" {
		return nil, multierror.Append(errors.Errorf("node name cannot be empty"))
	}

	node := &nodeImpl{
		Name:           nodeName,
		ExclusivePools: make([]Pool, 0),
	}

	if err := node.initializeDefaultPool(); err != nil {
		return nil, multierror.Append(allErrors, errors.Wrap(checks, "CreateInstance"))
	}
	// store list of all cores
	// at this point all cores are in the share pool, so we can copy it
	// this is a list of pointers, so we don't need to worry about keeping another set of objects up to date
	node.allCores = make([]Core, len(node.SharedPool.GetCores()))
	copy(node.allCores, node.SharedPool.GetCores())

	if IsFeatureSupported(CStatesFeature) {
		if err := mapAvailableCStates(); err != nil {
			return nil, multierror.Append(allErrors, checks)
		}
	}
	return node, allErrors.ErrorOrNil()
}

// getNumberOfCpus defined as var so can be mocked by the unit test
var getNumberOfCpus = runtime.NumCPU

// performs all pre-checks (driver etc.)
// sets supportedFeatureErrors map
func preChecks() *multierror.Error {
	var results *multierror.Error

	sstBfErr := preChecksSSTBF()
	supportedFeatureErrors[SSTBFFeature] = sstBfErr
	results = multierror.Append(results, sstBfErr)

	cStatesErr := preChecksCStates()
	supportedFeatureErrors[CStatesFeature] = cStatesErr
	results = multierror.Append(results, cStatesErr)

	return results
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

func IsFeatureSupported(features ...int) bool {
	for _, feature := range features {
		if supportedFeatureErrors[feature] != nil {
			return false
		}
	}
	return true
}
