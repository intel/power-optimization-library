package power

import (
	"fmt"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	cStatesDir           = "cpuidle"
	cStateDisableFileFmt = cStatesDir + "/state%d/disable"
	cStateNameFileFmt    = cStatesDir + "/state%d/name"
	cStatesDrvPath       = "cpuidle/current_driver"
)

type CStates map[string]bool

type CStatesSupportError struct {
	msg string
}

func (c *CStatesSupportError) Error() string {
	return "C-States unsupported: " + c.msg
}

func isSupportedCStatesDriver(driver string) bool {
	for _, s := range []string{"intel_idle", "acpi_idle"} {
		if driver == s {
			return true
		}
	}
	return false
}

// map of c-state Name to state number path in the sysfs
// populated during library initialisation
var cStatesNamesMap = map[string]int{}

func preChecksCStates() featureStatus {
	feature := featureStatus{
		Feature: CStatesFeature,
		Name:    "C-States",
	}
	driver, err := readStringFromFile(filepath.Join(basePath, cStatesDrvPath))
	driver = strings.TrimSuffix(driver, "\n")
	feature.Driver = driver
	if err != nil {
		feature.Error = &CStatesSupportError{"failed to determine Driver"}
	}
	if !isSupportedCStatesDriver(driver) {
		feature.Error = &CStatesSupportError{"unsupported Driver"}
	}
	return feature
}

func mapAvailableCStates() error {
	dirs, err := os.ReadDir(filepath.Join(basePath, "cpu0", cStatesDir))
	if err != nil {
		return errors.New("C-States: could not open cpu0 states dir")
	}

	cStateDirNameRegex := regexp.MustCompile(`state\d+`)
	for _, stateDir := range dirs {
		dirName := stateDir.Name()
		if !stateDir.IsDir() || !cStateDirNameRegex.MatchString(dirName) {
			continue
		}
		stateNumber, err := strconv.Atoi(dirName[5:])
		if err != nil {
			return errors.Wrapf(err, "getting state number %s", dirName)
		}

		stateName, err := readCoreStringProperty(0, fmt.Sprintf(cStateNameFileFmt, stateNumber))
		if err != nil {
			return errors.Wrapf(err, "could not read state%d Name", stateNumber)
		}

		cStatesNamesMap[stateName] = stateNumber
	}
	return nil
}
