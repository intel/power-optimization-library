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
	uncoreMaxFreqFile     = "max_freq_khz"
	uncoreMinFreqFile     = "min_freq_khz"
)

type (
	uncoreFreq struct {
		min uint
		max uint
	}
	Uncore interface {
		write(pkgID, dieID uint) error
	}
)

func NewUncore(minFreq uint, maxFreq uint) (Uncore, error) {
	if !featureList.isFeatureIdSupported(UncoreFeature) {
		return nil, featureList.getFeatureIdError(UncoreFeature)
	}
	if minFreq < defaultUncore.min {
		return nil, fmt.Errorf("specified Min frequency is lower than %d kHZ allowed by the hardware", defaultUncore.min)
	}
	if maxFreq > defaultUncore.max {
		return nil, fmt.Errorf("specified Max frequency is higher than %d kHz allowed by the hardware", defaultUncore.max)
	}
	if maxFreq < minFreq {
		return nil, fmt.Errorf("max freq cannot be lower than min")
	}

	normalizedMin := normalizeUncoreFreq(minFreq)
	normalizedMax := normalizeUncoreFreq(maxFreq)
	if normalizedMin != minFreq {
		log.Info("Uncore Min Frequency was normalized due to driver requirements", "requested", minFreq, "normalized", normalizedMin)
	}
	if normalizedMax != maxFreq {
		log.Info("Uncore Max Frequency was normalized due to driver requirements", "requested", maxFreq, "normalized", normalizedMax)
	}
	return &uncoreFreq{min: normalizedMin, max: normalizedMax}, nil
}

func (u *uncoreFreq) write(pkgId, dieId uint) error {
	if err := os.WriteFile(
		path.Join(basePath, fmt.Sprintf(uncorePathFmt, pkgId, dieId), uncoreMaxFreqFile),
		[]byte(fmt.Sprint(u.max)),
		0644,
	); err != nil {
		return err
	}
	if err := os.WriteFile(
		path.Join(basePath, fmt.Sprintf(uncorePathFmt, pkgId, dieId), uncoreMinFreqFile),
		[]byte(fmt.Sprint(u.min)),
		0644,
	); err != nil {
		return err
	}
	return nil
}

var (
	defaultUncore         = &uncoreFreq{}
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
		defaultUncore.max = value
	}
	if value, err := readUncoreProperty(0, 0, uncoreInitMinFreqFile); err != nil {
		feature.err = fmt.Errorf("uncore feature error %w", fmt.Errorf("failed to determine init freq: %w", err))
		return feature
	} else {
		defaultUncore.min = value
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

type hasUncore interface {
	SetUncore(uncore Uncore) error
	applyUncore() error
	getEffectiveUncore() Uncore
}

func (s *cpuTopology) SetUncore(uncore Uncore) error {
	s.uncore = uncore
	return s.applyUncore()
}

func (s *cpuTopology) getEffectiveUncore() Uncore {
	if s.uncore == nil {
		return defaultUncore
	}
	return s.uncore
}
func (s *cpuTopology) applyUncore() error {
	for _, pkg := range s.packages {
		if err := pkg.applyUncore(); err != nil {
			return err
		}
	}
	return nil
}
func (c *cpuPackage) SetUncore(uncore Uncore) error {
	c.uncore = uncore
	return c.applyUncore()
}

func (c *cpuPackage) applyUncore() error {
	for _, die := range c.dies {
		if err := die.applyUncore(); err != nil {
			return err
		}
	}
	return nil
}

func (c *cpuPackage) getEffectiveUncore() Uncore {
	if c.uncore != nil {
		return c.uncore
	}
	return c.topology.getEffectiveUncore()
}

func (d *cpuDie) SetUncore(uncore Uncore) error {
	d.uncore = uncore
	return d.applyUncore()
}

func (d *cpuDie) applyUncore() error {
	return d.getEffectiveUncore().write(d.parentSocket.getID(), d.id)
}

func (d *cpuDie) getEffectiveUncore() Uncore {
	if d.uncore != nil {
		return d.uncore
	}
	return d.parentSocket.getEffectiveUncore()
}

func readUncoreProperty(pkgID, dieID uint, property string) (uint, error) {
	fullPath := path.Join(basePath, fmt.Sprintf(uncorePathFmt, pkgID, dieID), property)
	return readUintFromFile(fullPath)
}

func normalizeUncoreFreq(freq uint) uint {
	return freq - (freq % uint(100_000))
}
