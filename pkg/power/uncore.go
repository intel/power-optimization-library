package power

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"
)

const (
	uncoreKmodName = "intel_uncore_frequency"
	uncoreDirName  = "intel_uncore_frequency"

	uncorePathFmt         = uncoreDirName + "/package_%02d_die_%02d"
	uncoreInitMaxFreqFile = "initial_max_freq_khz"
	uncoreInitMinFreqFile = "initial_min_freq_khz"
)

var (
	uncoreInitMaxFreq     uint
	uncoreInitMinFreq     uint
	kernelModulesFilePath = "/proc/modules"
)

func initUncore() featureStatus {
	feature := featureStatus{
		name:     "Uncore frequency",
		driver:   "N/A",
		initFunc: initUncore,
	}

	if !checkKernelModuleLoaded(uncoreKmodName) {
		feature.err = fmt.Errorf("uncore feature error: %w", fmt.Errorf("kernel module %s not loaded", uncoreKmodName))
		return feature
	}
	uncoreDirPath := path.Join(basePath, uncoreDirName)
	uncoreDir, err := os.OpenFile(uncoreDirPath, os.O_RDONLY, 0)
	if err != nil {
		feature.err = fmt.Errorf("uncore feature error: %w", err)
		return feature
	}
	if _, err := uncoreDir.Readdirnames(1); err != nil {
		feature.err = fmt.Errorf("uncore feature error: %w", fmt.Errorf("uncore interace dir empty or invalid: %w", err))
		return feature
	}
	if value, err := readUncoreProperty(0, 0, uncoreInitMaxFreqFile); err != nil {
		feature.err = fmt.Errorf("uncore feature error %w", fmt.Errorf("failed to determine init freq: %w", err))
		return feature
	} else {
		uncoreInitMaxFreq = value
	}
	if value, err := readUncoreProperty(0, 0, uncoreInitMinFreqFile); err != nil {
		feature.err = fmt.Errorf("uncore feature error %w", fmt.Errorf("failed to determine init freq: %w", err))
		return feature
	} else {
		uncoreInitMinFreq = value
	}

	return feature
}

func checkKernelModuleLoaded(module string) bool {
	modulesFile, err := os.Open(kernelModulesFilePath)
	if err != nil {
		return false
	}
	defer modulesFile.Close()

	reader := bufio.NewScanner(modulesFile)
	for reader.Scan() {
		if strings.Contains(reader.Text(), module) {
			return true
		}
	}
	return false
}

func readUncoreProperty(pkgID, dieID uint, property string) (uint, error) {
	fullPath := path.Join(basePath, fmt.Sprintf(uncorePathFmt, pkgID, dieID), property)
	return readUintFromFile(fullPath)
}
