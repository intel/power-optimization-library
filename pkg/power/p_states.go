package power

const (
	cpuMaxFreqFile = "cpufreq/cpuinfo_max_freq"
	cpuMinFreqFile = "cpufreq/cpuinfo_min_freq"
	scalingMaxFile = "cpufreq/scaling_max_freq"
	scalingMinFile = "cpufreq/scaling_min_freq"

	eppFile = "cpufreq/energy_performance_preference"

	scalingGovFile = "cpufreq/scaling_governor"

	scalingDrvFile = "cpufreq/scaling_driver"

	sharedPoolName = "shared"

	defaultEpp      = "balance_performance"
	defaultGovernor = cpuPolicyPowersave

	cpuPolicyPerformance = "performance"
	cpuPolicyPowersave   = "powersave"
)

type PStatesSupportError struct {
	msg string
}

func (s *PStatesSupportError) Error() string {
	return "P-States unsupported: " + s.msg
}

func isPStatesDriverSupported(driver string) bool {
	for _, s := range []string{"intel_pstate", "intel_cpufreq"} {
		if driver == s {
			return true
		}
	}
	return false
}

func preChecksPStates() featureStatus {
	pStates := featureStatus{
		Feature: PStatesFeature,
		Name:    "P-States",
	}
	driver, err := readCoreStringProperty(0, scalingDrvFile)
	pStates.Driver = driver
	if err != nil {
		pStates.Error = &PStatesSupportError{"failed to determine Driver"}
	}
	if !isPStatesDriverSupported(driver) {
		pStates.Error = &PStatesSupportError{"unsupported Driver"}
	}
	return pStates

}
