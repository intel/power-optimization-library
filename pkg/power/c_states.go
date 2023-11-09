package power

import (
	"fmt"
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
	cStatesDrvPath       = cStatesDir + "/current_driver"
)

type CStates map[string]bool

func isSupportedCStatesDriver(driver string) bool {
	for _, s := range []string{"intel_idle", "acpi_idle"} {
		if driver == s {
			return true
		}
	}
	return false
}

// map of c-state name to state number path in the sysfs
// populated during library initialisation
var cStatesNamesMap = map[string]int{}

// populated when mapping CStates
var defaultCStates = CStates{}

func initCStates() featureStatus {
	feature := featureStatus{
		name:     "C-States",
		initFunc: initCStates,
	}
	driver, err := readStringFromFile(filepath.Join(basePath, cStatesDrvPath))
	driver = strings.TrimSuffix(driver, "\n")
	feature.driver = driver
	if err != nil {
		feature.err = fmt.Errorf("failed to determine driver: %w", err)
		return feature
	}
	if !isSupportedCStatesDriver(driver) {
		feature.err = fmt.Errorf("unsupported driver: %s", driver)
		return feature
	}
	feature.err = mapAvailableCStates()

	return feature
}

// sets cStatesNamesMap and defaultCStates
func mapAvailableCStates() error {
	dirs, err := os.ReadDir(filepath.Join(basePath, "cpu0", cStatesDir))
	if err != nil {
		return fmt.Errorf("could not open cpu0 C-States directory: %w", err)
	}

	cStateDirNameRegex := regexp.MustCompile(`state(\d+)`)
	for _, stateDir := range dirs {
		dirName := stateDir.Name()
		if !stateDir.IsDir() || !cStateDirNameRegex.MatchString(dirName) {
			log.Info("map C-States ignoring " + dirName)
			continue
		}
		stateNumber, err := strconv.Atoi(cStateDirNameRegex.FindStringSubmatch(dirName)[1])
		if err != nil {
			return fmt.Errorf("failed to extract C-State number %s: %w", dirName, err)
		}

		stateName, err := readCpuStringProperty(0, fmt.Sprintf(cStateNameFileFmt, stateNumber))
		if err != nil {
			return fmt.Errorf("could not read C-State %d name: %w", stateNumber, err)
		}

		cStatesNamesMap[stateName] = stateNumber
		defaultCStates[stateName] = true
	}
	log.V(3).Info("mapped C-states", "map", cStatesNamesMap)
	return nil
}

func validateCStates(states CStates) error {
	for name := range states {
		if _, exists := cStatesNamesMap[name]; !exists {
			return fmt.Errorf("c-state %s does not exist on this system", name)
		}
	}
	return nil
}
func (host *hostImpl) ValidateCStates(states CStates) error {
	return validateCStates(states)
}

func (host *hostImpl) AvailableCStates() []string {
	if !featureList.isFeatureIdSupported(CStatesFeature) {
		return []string{}
	}
	cStatesList := make([]string, 0)
	for name := range cStatesNamesMap {
		cStatesList = append(cStatesList, name)
	}
	return cStatesList
}

func (pool *poolImpl) SetCStates(states CStates) error {
	if !IsFeatureSupported(CStatesFeature) {
		return featureList.getFeatureIdError(CStatesFeature)
	}
	// check if requested states are on the system
	if err := validateCStates(states); err != nil {
		return err
	}
	pool.CStatesProfile = &states
	for _, cpu := range pool.cpus {
		if err := cpu.consolidate(); err != nil {
			return fmt.Errorf("failed to apply c-states: %w", err)
		}
	}
	return nil
}

func (pool *poolImpl) getCStates() *CStates {
	return pool.CStatesProfile
}

func (cpu *cpuImpl) SetCStates(cStates CStates) error {
	if !IsFeatureSupported(CStatesFeature) {
		return featureList.getFeatureIdError(CStatesFeature)
	}
	if err := validateCStates(cStates); err != nil {
		return err
	}
	cpu.cStates = &cStates
	return cpu.updateCStates()
}
func (cpu *cpuImpl) updateCStates() error {
	if !IsFeatureSupported(CStatesFeature) {
		return nil
	}
	if cpu.cStates != nil && *cpu.cStates != nil {
		return cpu.applyCStates(cpu.cStates)
	}
	if cpu.pool.getCStates() != nil {
		return cpu.applyCStates(cpu.pool.getCStates())
	}
	return cpu.applyCStates(&defaultCStates)
}

func (cpu *cpuImpl) applyCStates(desiredCStates *CStates) error {
	for state, enabled := range *desiredCStates {
		stateFilePath := filepath.Join(
			basePath,
			fmt.Sprint("cpu", cpu.id),
			fmt.Sprintf(cStateDisableFileFmt, cStatesNamesMap[state]),
		)
		content := make([]byte, 1)
		if enabled {
			content[0] = '0' // write '0' to enable the c state
		} else {
			content[0] = '1' // write '1' to disable the c state
		}
		if err := os.WriteFile(stateFilePath, content, 0644); err != nil {
			return fmt.Errorf("could not apply cstate %s on cpu %d: %w", state, cpu.id, err)
		}
	}
	return nil
}
