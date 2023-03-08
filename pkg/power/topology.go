package power

const (
	cpuTopologyDir = "topology/"
	packageIdFile  = cpuTopologyDir + "physical_package_id"
	dieIdFile      = cpuTopologyDir + "die_id"
	coreIdFile     = cpuTopologyDir + "core_id"
)

type topologyTypeObj interface {
	addCpu(uint) (Cpu, error)
	CPUs() *CpuList
}

// parent struct to store system topology
type (
	systemTopology struct {
		packages packageList
		allCpus  CpuList
	}

	Topology interface {
		topologyTypeObj
		Packages() *[]Package
		Package(id uint) Package
	}
)

func (s *systemTopology) addCpu(cpuId uint) (Cpu, error) {
	var socketId uint
	var err error
	var cpu Cpu

	if socketId, err = readCpuUintProperty(cpuId, packageIdFile); err != nil {
		return nil, err
	}

	if socket, exists := s.packages[socketId]; exists {
		cpu, err = socket.addCpu(cpuId)
	} else {
		s.packages[socketId] = &cpuPackage{
			topology: s,
			id:       socketId,
			cpus:     CpuList{},
			dies:     dieList{},
		}
		cpu, err = s.packages[socketId].addCpu(cpuId)
	}
	if err != nil {
		return nil, err
	}
	s.allCpus[cpuId] = cpu
	return cpu, err
}

func (s *systemTopology) CPUs() *CpuList {
	return &s.allCpus
}

func (s *systemTopology) Packages() *[]Package {
	pkgs := make([]Package, len(s.packages))

	i := 0
	for _, pkg := range s.packages {
		pkgs[i] = pkg
		i++
	}
	return &pkgs
}

func (s *systemTopology) Package(id uint) Package {
	pkg, _ := s.packages[id]
	return pkg
}

// cpu socket represents a physical cpu package
type (
	cpuPackage struct {
		topology *systemTopology
		id       uint
		cpus     CpuList
		dies     dieList
	}
	Package interface {
		topologyTypeObj
		Dies() *[]Die
		Die(id uint) Die
	}
)

func (c *cpuPackage) Dies() *[]Die {
	dice := make([]Die, len(c.dies))
	i := 0
	for _, die := range c.dies {
		dice[i] = die
		i++
	}
	return &dice
}

func (c *cpuPackage) Die(id uint) Die {
	die, _ := c.dies[id]
	return die
}

func (c *cpuPackage) addCpu(cpuId uint) (Cpu, error) {
	var err error
	var dieId uint
	var cpu Cpu

	if dieId, err = readCpuUintProperty(cpuId, dieIdFile); err != nil {
		return nil, err
	}

	if die, exists := c.dies[dieId]; exists {
		cpu, err = die.addCpu(cpuId)
	} else {
		c.dies[dieId] = &cpuDie{
			parentSocket: c,
			id:           cpuId,
			cores:        coreList{},
			cpus:         CpuList{},
		}
		cpu, err = c.dies[dieId].addCpu(cpuId)
	}
	if err != nil {
		return nil, err
	}
	c.cpus.add(cpu)
	return cpu, nil
}

func (c *cpuPackage) CPUs() *CpuList {
	return &c.cpus
}

type (
	cpuDie struct {
		parentSocket *cpuPackage
		id           uint
		cores        coreList
		cpus         CpuList
	}
	Die interface {
		topologyTypeObj
		Cores() *[]Core
		Core(id uint) Core
	}
)

func (d *cpuDie) Cores() *[]Core {
	cores := make([]Core, len(d.cores))
	i := 0
	for _, core := range d.cores {
		cores[i] = core
		i++
	}
	return &cores
}

func (d *cpuDie) Core(id uint) Core {
	core, _ := d.cores[id]
	return core
}

func (d *cpuDie) CPUs() *CpuList {
	return &d.cpus
}

func (d *cpuDie) addCpu(cpuId uint) (Cpu, error) {
	var err error
	var coreId uint
	var cpu Cpu

	if coreId, err = readCpuUintProperty(cpuId, coreIdFile); err != nil {
		return nil, err
	}

	if core, exists := d.cores[coreId]; exists {
		cpu, err = core.addCpu(cpuId)
	} else {
		d.cores[coreId] = &cpuCore{
			parentDie: d,
			id:        coreId,
			cpus:      CpuList{},
		}
		cpu, err = d.cores[coreId].addCpu(cpuId)
	}
	if err != nil {
		return nil, err
	}
	d.cpus.add(cpu)
	return cpu, nil
}

type (
	cpuCore struct {
		parentDie *cpuDie
		id        uint
		cpus      CpuList
	}
	Core interface {
		topologyTypeObj
	}
)

func (c *cpuCore) addCpu(cpuId uint) (Cpu, error) {
	cpu, err := newCpu(cpuId, c)
	if err != nil {
		return nil, err
	}
	c.cpus.add(cpu)
	return cpu, nil
}

func (c *cpuCore) CPUs() *CpuList {
	return &c.cpus
}

type packageList map[uint]*cpuPackage

type dieList map[uint]*cpuDie

type coreList map[uint]*cpuCore

var discoverTopology = func() (Topology, error) {
	numOfCores := getNumberOfCpus()
	topology := &systemTopology{
		allCpus:  make(CpuList, numOfCores),
		packages: packageList{},
	}
	for i := uint(0); i < numOfCores; i++ {
		if _, err := topology.addCpu(i); err != nil {
			return nil, err
		}
	}
	return topology, nil
}
