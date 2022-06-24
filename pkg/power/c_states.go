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

type CStatesSupportError struct {
	msg string
}

func (c *CStatesSupportError) Error() string {
	return "C-States unsupported: " + c.msg
}

// map of c-state name to state number path in the sysfs
// populated during library initialisation
var cStatesNamesMap = map[string]int{}

func preChecksCStates() error {
	driver, err := readStringFromFile(filepath.Join(basePath, cStatesDrvPath))
	if err != nil {
		return &CStatesSupportError{"failed to determine driver"}
	}
	driver = strings.TrimSuffix(driver, "\n")
	if driver != "intel_idle" && driver != "acpi_idle" {
		return &CStatesSupportError{"unsupported driver: " + driver}
	}
	return nil
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
			return errors.Wrapf(err, "could not read state%d name", stateNumber)
		}

		cStatesNamesMap[stateName] = stateNumber
	}
	return nil
}
