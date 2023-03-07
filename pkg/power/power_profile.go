package power

import (
	"fmt"
)

type profileImpl struct {
	name     string
	max      int
	min      int
	epp      string
	governor string
	// todo classification
}

// Profile is a P-states power profile
// requires P-states feature
type Profile interface {
	Name() string
	Epp() string
	MaxFreq() int
	MinFreq() int
	Governor() string
}

// todo add simple constructor that determines frequencies automagically?

// NewPowerProfile creates a power P-States power profile,
func NewPowerProfile(name string, minFreq int, maxFreq int, governor string, epp string) (Profile, error) {
	if !featureList.isFeatureIdSupported(PStatesFeature) {
		return nil, featureList.getFeatureIdError(PStatesFeature)
	}

	if minFreq > maxFreq {
		return nil, fmt.Errorf("max Freq can't be lower than min")
	}

	if governor != cpuPolicyPerformance && governor != cpuPolicyPowersave { //todo determine by reading available governors, its different for acpi Driver
		return nil, fmt.Errorf("governor can only be set to '%s' or '%s'", cpuPolicyPerformance, cpuPolicyPowersave)
	}

	if governor == cpuPolicyPerformance && epp != cpuPolicyPerformance {
		return nil, fmt.Errorf("only '%s' epp can be used with '%s' governor", cpuPolicyPerformance, cpuPolicyPerformance)
	}

	log.Info("creating powerProfile object", "name", name)
	return &profileImpl{
		name:     name,
		max:      maxFreq * 1000,
		min:      minFreq * 1000,
		epp:      epp,
		governor: governor,
	}, nil
}

func (p *profileImpl) Epp() string {
	return p.epp
}

func (p *profileImpl) MaxFreq() int {
	return p.max
}

func (p *profileImpl) MinFreq() int {
	return p.min
}

func (p *profileImpl) Name() string {
	return p.name
}

func (p *profileImpl) Governor() string {
	return p.governor
}
