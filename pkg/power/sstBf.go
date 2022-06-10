package power

const (
	cpuMaxFreqFile = "cpufreq/cpuinfo_max_freq"
	cpuMinFreqFile = "cpufreq/cpuinfo_min_freq"
	scalingMaxFile = "cpufreq/scaling_max_freq"
	scalingMinFile = "cpufreq/scaling_min_freq"
	eppFile        = "cpufreq/energy_performance_preference"
	scalingDrvFile = "cpufreq/scaling_driver"

	sharedPoolName = "shared"
	defaultEpp     = "balance_performance"
)

type SSTBFSupportError struct {
	msg string
}

func (s *SSTBFSupportError) Error() string {
	return "SST-BF unsupported: " + s.msg
}

func preChecksSSTBF() error {
	driver, err := readCoreStringProperty(0, scalingDrvFile)
	if err != nil {
		return &SSTBFSupportError{"failed to determine driver"}
	}
	if driver != "intel_pstate" {
		return &SSTBFSupportError{"unsupported driver"}
	}
	return nil

}
