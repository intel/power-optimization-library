package power

import (
	"fmt"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"os"
	"runtime"
	"strconv"
	"strings"
)

var basePath = "/sys/devices/system/cpu"

type featureID uint

const (
	sharedPoolName   = "sharedPool"
	reservedPoolName = "reservedPool"

	PStatesFeature featureID = iota
	CStatesFeature
)

// initialized with null logger, can be set to proper logger with SetLogger
var log = logr.Discard()

// default declaration of defined features, defined to uninitialized state
var featureList FeatureSet = map[featureID]*featureStatus{
	PStatesFeature: {
		err:      uninitialisedErr,
		initFunc: initPStates,
	},
	CStatesFeature: {
		err:      uninitialisedErr,
		initFunc: initCStates,
	},
}
var uninitialisedErr = fmt.Errorf("feature uninitialized")
var undefinedErr = fmt.Errorf("feature undefined")

// featureStatus stores feature name, driver and if feature is not supported, error describing the reason
type featureStatus struct {
	name     string
	driver   string
	err      error
	initFunc func() featureStatus
}

func (f *featureStatus) Name() string {
	return f.name
}
func (f *featureStatus) Driver() string {
	return f.driver
}
func (f *featureStatus) FeatureError() error {
	return f.err
}

func (f *featureStatus) isSupported() bool {
	if f.err == nil {
		return true
	}
	return false
}

// FeatureSet stores info of about functionalities supported by the power library
// on current system
type FeatureSet map[featureID]*featureStatus

// initialise all defined features, return multierror for each failed feature
func (set *FeatureSet) init() *multierror.Error {
	var allErrors *multierror.Error
	if len(*set) == 0 {
		return multierror.Append(allErrors, fmt.Errorf("no features defined"))
	}
	for id, status := range *set {
		feature := status.initFunc()
		(*set)[id] = &feature
		// this already checks for nil
		allErrors = multierror.Append(allErrors, feature.err)
	}
	return allErrors
}

// anySupported checks if any of the defined featured is supported on current machine
func (set *FeatureSet) anySupported() bool {
	for _, status := range *set {
		if status.err == nil {
			return true
		}
	}
	return false
}

// isFeatureIdSupported takes feature if, check if feature is supported on current system
func (set *FeatureSet) isFeatureIdSupported(id featureID) bool {
	feature, exists := (*set)[id]
	if !exists {
		return false
	}
	return feature.isSupported()
}

// getFeatureIdError retrieve any error associated with a feature
func (set *FeatureSet) getFeatureIdError(id featureID) error {
	feature, exists := (*set)[id]
	if !exists {
		return undefinedErr
	}
	return feature.err
}

// CreateInstance initialises the power library
// returns Host with empty list of exclusive pools, and a default pool containing all cpus
// by default all cpus are in the system reserved pool
// if fatal errors occurred returns nil and error
// if non-fatal error occurred Host object and error are returned
func CreateInstance(hostName string) (Host, error) {
	allErrors := featureList.init()

	if !featureList.anySupported() {
		return nil, allErrors
	}
	host, err := initHost(hostName)
	if err != nil {
		return nil, multierror.Append(allErrors, err)
	}
	return host, allErrors.ErrorOrNil()
}

// getNumberOfCpus defined as var so can be mocked by the unit test
var getNumberOfCpus = func() uint {
	return uint(runtime.NumCPU())
}

// reads a file from a path, parses contents as an int a returns the value
// returns Error if any step fails
func readIntFromFile(filePath string) (int, error) {
	valueString, err := readStringFromFile(filePath)
	if err != nil {
		return 0, err
	}
	valueString = strings.TrimSuffix(valueString, "\n")
	value, err := strconv.Atoi(valueString)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// reads value from a file and returns contents as a string
func readStringFromFile(filePath string) (string, error) {
	valueByte, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(valueByte), nil
}

// IsFeatureSupported checks if any number of features is supported. if any of the checked features is not supported
// return false
func IsFeatureSupported(features ...featureID) bool {
	for _, feature := range features {
		if !featureList.isFeatureIdSupported(feature) {
			return false
		}
	}
	return true
}

// SetLogger takes fre-configured go-logr logr.Logger to be used by the library
func SetLogger(logger logr.Logger) {
	log = logger
}
