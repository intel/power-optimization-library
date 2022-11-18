package power

import "github.com/pkg/errors"

type profileImpl struct {
	Name     string
	Max      int
	Min      int
	Epp      string
	Governor string
}

type Profile interface {
	GetName() string
	GetEpp() string
	GetMaxFreq() int
	GetMinFreq() int
	GetGovernor() string
}

func NewProfile(name string, minFreq int, maxFreq int, governor string, epp string) (Profile, error) {
	if minFreq > maxFreq {
		return nil, errors.New("Max Freq can't be lower than Min")
	}

	if governor != cpuPolicyPerformance && governor != cpuPolicyPowersave { //todo determine by reading available governors, its different for acpi Driver
		return nil, errors.Errorf("Governor can only be set to '%s' or '%s'", cpuPolicyPerformance, cpuPolicyPowersave)
	}
	// todo check if this is p-state specific
	if governor == cpuPolicyPerformance && epp != cpuPolicyPerformance {
		return nil, errors.Errorf("Only '%s' epp can be used with '%s' governor", cpuPolicyPerformance, cpuPolicyPerformance)
	}

	return &profileImpl{
		Name:     name,
		Max:      maxFreq * 1000,
		Min:      minFreq * 1000,
		Epp:      epp,
		Governor: governor,
	}, nil
}

func (p *profileImpl) GetEpp() string {
	return p.Epp
}

func (p *profileImpl) GetMaxFreq() int {
	return p.Max
}

func (p *profileImpl) GetMinFreq() int {
	return p.Min
}

func (p *profileImpl) SetProfileName(name string) {
	p.Name = name
}

func (p *profileImpl) GetName() string {
	return p.Name
}

func (p *profileImpl) GetGovernor() string {
	return p.Governor
}
