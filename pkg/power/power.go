package power

import (
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
)

var basePath = "/sys/devices/system/cpu"

type featureID uint

const (
	sharedPoolName                    = "sharedPool"
	reservedPoolName                  = "reservedPool"
	FrequencyScalingFeature featureID = iota
	EPPFeature
	CStatesFeature
	UncoreFeature
)

type LibConfig struct {
	CpuPath    string
	ModulePath string
	Cores      uint
}

// initialized with null logger, can be set to proper logger with SetLogger
var log = logr.Discard()

// default declaration of defined features, defined to uninitialized state
var featureList FeatureSet = map[featureID]*featureStatus{
	EPPFeature: {
		err:      uninitialisedErr,
		initFunc: initEpp,
	},
	FrequencyScalingFeature: {
		err:      uninitialisedErr,
		initFunc: initScalingDriver,
	},
	CStatesFeature: {
		err:      uninitialisedErr,
		initFunc: initCStates,
	},
	UncoreFeature: {
		err:      uninitialisedErr,
		initFunc: initUncore,
	},
}
var uninitialisedErr = fmt.Errorf("feature uninitialized")
var undefinederr = fmt.Errorf("feature undefined")

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
	return f.err == nil
}

// FeatureSet stores info of about functionalities supported by the power library
// on current system
type FeatureSet map[featureID]*featureStatus

// initialise all defined features, return multiple errors for each failed feature
func (set *FeatureSet) init() error {
	if len(*set) == 0 {
		return fmt.Errorf("no features defined")
	}
	allErrors := make([]error, 0, len(*set))
	for id, status := range *set {
		feature := status.initFunc()
		(*set)[id] = &feature
		allErrors = append(allErrors, feature.err)
	}
	return errors.Join(allErrors...)
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
		return undefinederr
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
		return nil, errors.Join(allErrors, err)
	}
	return host, allErrors
}
func CreateInstanceWithConf(hostname string, conf LibConfig) (Host, error) {
	if conf.CpuPath != "" {
		basePath = conf.CpuPath
	}
	if conf.ModulePath != "" {
		kernelModulesFilePath = conf.ModulePath
	}
	getNumberOfCpus = func() uint { return conf.Cores }
	return CreateInstance(hostname)
}

// getNumberOfCpus defined as var so can be mocked by the unit test
var getNumberOfCpus = func() uint {
	// First, try to get CPUs from sysfs. If the sysfs isn't available
	// return Number of CPUs from runtime
	cpusAvailable, err := readStringFromFile(path.Join(basePath, "online"))
	if err != nil {
		return uint(runtime.NumCPU())
	}
	// Delete \n character and split the string to get
	// first and last element
	cpusAvailable = strings.Replace(cpusAvailable, "\n", "", -1)
	cpuSlice := strings.Split(cpusAvailable, "-")
	if len(cpuSlice) < 2 {
		return uint(runtime.NumCPU())
	}
	// Calculate number of CPUs, if an error occurs
	// return the number of CPUs from runtime
	firstElement, err := strconv.Atoi(cpuSlice[0])
	if err != nil {
		return uint(runtime.NumCPU())
	}
	secondElement, err := strconv.Atoi(cpuSlice[1])
	if err != nil {
		return uint(runtime.NumCPU())
	}
	return uint((secondElement - firstElement) + 1)
}

// reads a file from a path, parses contents as an int a returns the value
// returns Error if any step fails
func readUintFromFile(filePath string) (uint, error) {
	valueString, err := readStringFromFile(filePath)
	if err != nil {
		return 0, err
	}
	valueString = strings.TrimSuffix(valueString, "\n")
	value, err := strconv.Atoi(valueString)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("unexpected negative value when expecting uint")
	}
	return uint(value), nil
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
