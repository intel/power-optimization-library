package power

type profileImpl struct {
	Name string
	Max  int
	Min  int
	Epp  string
}

type Profile interface {
	GetName() string
	GetEpp() string
	GetMaxFreq() int
	GetMinFreq() int
}

func NewProfile(name string, minFreq int, maxFreq int, epp string) Profile {
	if minFreq > maxFreq {
		return nil
	}
	return &profileImpl{
		Name: name,
		Max:  maxFreq * 1000,
		Min:  minFreq * 1000,
		Epp:  epp,
	}
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
