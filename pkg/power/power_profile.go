package power

import (
	"fmt"
)

type profileImpl struct {
	name         string
	max          uint
	min          uint
	efficientMax uint
	efficientMin uint
	epp          string
	governor     string
	// todo classification
}

// Profile contains scaling driver information
type Profile interface {
	Name() string
	Epp() string
	MaxFreq() uint
	EfficientMaxFreq() uint
	MinFreq() uint
	EfficientMinFreq() uint
	Governor() string
}

var availableGovs []string

// todo add simple constructor that determines frequencies automagically?

// NewPowerProfile creates a power profile,
func NewPowerProfile(name string, minFreq uint, maxFreq uint, governor string, epp string) (Profile, error) {
	if !featureList.isFeatureIdSupported(FrequencyScalingFeature) {
		return nil, featureList.getFeatureIdError(FrequencyScalingFeature)
	}
	if len(coreTypes) > 1 {
		log.Error(fmt.Errorf("creating standard power profile on system with multiple core types"), "undefined behavior expected")
	}
	if minFreq > maxFreq {
		return nil, fmt.Errorf("max Freq can't be lower than min")
	}
	if governor == "" {
		governor = defaultGovernor
	}
	if !checkGov(governor) { //todo determine by reading available governors, its different for acpi Driver
		return nil, fmt.Errorf("governor can only be set to the following %v", availableGovs)

	}
	if epp != "" && governor == cpuPolicyPerformance && epp != cpuPolicyPerformance {
		return nil, fmt.Errorf("only '%s' epp can be used with '%s' governor", cpuPolicyPerformance, cpuPolicyPerformance)
	}

	log.Info("creating powerProfile object", "name", name)
	return &profileImpl{
		name:         name,
		max:          maxFreq * 1000,
		min:          minFreq * 1000,
		efficientMax: maxFreq * 1000,
		efficientMin: minFreq * 1000,
		epp:          epp,
		governor:     governor,
	}, nil
}

// creates a Power Profile for efficient and performant cores
func NewEcorePowerProfile(name string, minFreq uint, maxFreq uint, emin uint, emax uint, governor string, epp string) (Profile, error) {
	if !featureList.isFeatureIdSupported(FrequencyScalingFeature) {
		return nil, featureList.getFeatureIdError(FrequencyScalingFeature)
	}
	if minFreq > maxFreq {
		return nil, fmt.Errorf("max Freq can't be lower than min")
	}
	if emin > emax {
		return nil, fmt.Errorf("max Freq can't be lower than min")
	}
	if governor == "" {
		governor = defaultGovernor
	}
	if !checkGov(governor) { //todo determine by reading available governors, its different for acpi Driver
		return nil, fmt.Errorf("governor can only be set to the following %v", availableGovs)

	}
	if epp != "" && governor == cpuPolicyPerformance && epp != cpuPolicyPerformance {
		return nil, fmt.Errorf("only '%s' epp can be used with '%s' governor", cpuPolicyPerformance, cpuPolicyPerformance)
	}

	log.Info("creating powerProfile object", "name", name)
	return &profileImpl{
		name:         name,
		max:          maxFreq * 1000,
		min:          minFreq * 1000,
		efficientMax: emax * 1000,
		efficientMin: emin * 1000,
		epp:          epp,
		governor:     governor,
	}, nil
}

func (p *profileImpl) Epp() string {
	return p.epp
}

func (p *profileImpl) MaxFreq() uint {
	return p.max
}

func (p *profileImpl) MinFreq() uint {
	return p.min
}

func (p *profileImpl) EfficientMaxFreq() uint {
	return p.efficientMax
}

func (p *profileImpl) EfficientMinFreq() uint {
	return p.efficientMin
}

func (p *profileImpl) Name() string {
	return p.name
}

func (p *profileImpl) Governor() string {
	return p.governor
}

func checkGov(governor string) bool {
	for _, element := range availableGovs {
		if element == governor {
			return true
		}
	}
	return false
}
