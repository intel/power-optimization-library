package power

// collection of Scaling Driver specific functions and methods

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	pStatesDrvFile = "cpufreq/scaling_driver"

	cpuMaxFreqFile = "cpufreq/cpuinfo_max_freq"
	cpuMinFreqFile = "cpufreq/cpuinfo_min_freq"
	scalingMaxFile = "cpufreq/scaling_max_freq"
	scalingMinFile = "cpufreq/scaling_min_freq"

	scalingGovFile = "cpufreq/scaling_governor"
	availGovFile   = "cpufreq/scaling_available_governors"
	eppFile        = "cpufreq/energy_performance_preference"

	defaultEpp      = "default"
	defaultGovernor = cpuPolicyPowersave

	cpuPolicyPerformance  = "performance"
	cpuPolicyPowersave    = "powersave"
	cpuPolicyUserspace    = "userspace"
	cpuPolicyOndemand     = "ondemand"
	cpuPolicySchedutil    = "schedutil"
	cpuPolicyConservative = "conservative"
)

var defaultPowerProfile *profileImpl

func isScalingDriverSupported(driver string) bool {
	for _, s := range []string{"intel_pstate", "intel_cpufreq", "acpi-cpufreq"} {
		if driver == s {
			return true
		}
	}
	return false
}

func initScalingDriver() featureStatus {
	pStates := featureStatus{
		name:     "Frequency-Scaling",
		initFunc: initScalingDriver,
	}
	var err error
	availableGovs, err = initAvailableGovernors()
	if err != nil {
		pStates.err = fmt.Errorf("failed to read available governors: %w", err)
	}
	driver, err := readCpuStringProperty(0, pStatesDrvFile)
	if err != nil {
		pStates.err = fmt.Errorf("%s - failed to read driver name: %w", pStates.name, err)
	}
	pStates.driver = driver
	if !isScalingDriverSupported(driver) {
		pStates.err = fmt.Errorf("%s - unsupported driver: %s", pStates.name, driver)
	}
	if err != nil {
		pStates.err = fmt.Errorf("%s - failed to determine driver: %w", pStates.name, err)
	}
	if pStates.err == nil {
		if err := generateDefaultProfile(); err != nil {
			pStates.err = fmt.Errorf("failed to read default frequenices: %w", err)
		}
	}
	return pStates
}
func initEpp() featureStatus {
	epp := featureStatus{
		name:     "Energy-Performance-Preference",
		initFunc: initEpp,
	}
	_, err := readCpuStringProperty(0, eppFile)
	if os.IsNotExist(errors.Unwrap(err)) {
		epp.err = fmt.Errorf("EPP file %s does not exist", eppFile)
	}
	return epp
}

func initAvailableGovernors() ([]string, error) {
	govs, err := readCpuStringProperty(0, availGovFile)
	if err != nil {
		return []string{}, err
	}
	return strings.Split(govs, " "), nil
}
func GetAvailableGovernors() []string{
	return availableGovs
}
func generateDefaultProfile() error {
	maxFreq, err := readCpuUintProperty(0, cpuMaxFreqFile)
	if err != nil {
		return err
	}
	minFreq, err := readCpuUintProperty(0, cpuMinFreqFile)
	if err != nil {
		return err
	}

	_, err = readCpuStringProperty(0, eppFile)
	epp := defaultEpp
	if os.IsNotExist(errors.Unwrap(err)) {
		epp = ""
	}
	defaultPowerProfile = &profileImpl{
		name:     "default",
		max:      maxFreq,
		min:      minFreq,
		epp:      epp,
		governor: defaultGovernor,
	}
	return nil
}

func (cpu *cpuImpl) updateFrequencies() error {
	if !IsFeatureSupported(FreqencyScalingFeature) {
		return nil
	}
	if cpu.pool.GetPowerProfile() != nil {
		return cpu.setDriverValues(cpu.pool.GetPowerProfile())
	}
	return cpu.setDriverValues(defaultPowerProfile)
}

// setDriverValues is an entrypoint to power governor feature consolidation
func (cpu *cpuImpl) setDriverValues(powerProfile Profile) error {
	if err := cpu.writeGovernorValue(powerProfile.Governor()); err != nil {
		return fmt.Errorf("failed to set governor for cpu %d: %w", cpu.id, err)
	}
	if powerProfile.Epp() != "" {
		if err := cpu.writeEppValue(powerProfile.Epp()); err != nil {
			return fmt.Errorf("failed to set EPP value for cpu %d: %w", cpu.id, err)
		}
	}
	if err := cpu.writeScalingMaxFreq(powerProfile.MaxFreq()); err != nil {
		return fmt.Errorf("failed to set MaxFreq value for cpu %d: %w", cpu.id, err)
	}
	if err := cpu.writeScalingMinFreq(powerProfile.MinFreq()); err != nil {
		return fmt.Errorf("failed to set MaxFreq value for cpu %d: %w", cpu.id, err)
	}

	return nil
}
func (cpu *cpuImpl) writeGovernorValue(governor string) error {
	return os.WriteFile(filepath.Join(basePath, fmt.Sprint("cpu", cpu.id), scalingGovFile), []byte(governor), 0644)
}
func (cpu *cpuImpl) writeEppValue(eppValue string) error {
	return os.WriteFile(filepath.Join(basePath, fmt.Sprint("cpu", cpu.id), eppFile), []byte(eppValue), 0644)
}
func (cpu *cpuImpl) writeScalingMaxFreq(freq uint) error {
	scalingFile := filepath.Join(basePath, fmt.Sprint("cpu", cpu.id), scalingMaxFile)
	f, err := os.OpenFile(
		scalingFile,
		os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
		0644,
	)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprint(freq))
	if err != nil {
		return err
	}
	return nil
}
func (cpu *cpuImpl) writeScalingMinFreq(freq uint) error {
	scalingFile := filepath.Join(basePath, fmt.Sprint("cpu", cpu.id), scalingMinFile)
	f, err := os.OpenFile(
		scalingFile,
		os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
		0644,
	)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprint(freq))
	if err != nil {
		return err
	}
	return nil
}
