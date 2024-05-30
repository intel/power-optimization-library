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
	getID() uint
}

// this stores the frequencies of core types
// cores can refer to this object using an array index
var coreTypes CoreTypeList

// parent struct to store system topology
type (
	cpuTopology struct {
		packages packageList
		allCpus  CpuList
		uncore   Uncore
	}

	Topology interface {
		topologyTypeObj
		hasUncore
		Packages() *[]Package
		Package(id uint) Package
	}
)

func (s *cpuTopology) addCpu(cpuId uint) (Cpu, error) {
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

func (s *cpuTopology) CPUs() *CpuList {
	return &s.allCpus
}

func (s *cpuTopology) CoreTypes() CoreTypeList {
	return coreTypes
}

func (s *cpuTopology) Packages() *[]Package {
	pkgs := make([]Package, len(s.packages))

	i := 0
	for _, pkg := range s.packages {
		pkgs[i] = pkg
		i++
	}
	return &pkgs
}

func (s *cpuTopology) Package(id uint) Package {
	pkg := s.packages[id]
	return pkg
}

func (s *cpuTopology) getID() uint {
	return 0
}

// cpu socket represents a physical cpu package
type (
	cpuPackage struct {
		topology Topology
		id       uint
		uncore   Uncore
		cpus     CpuList
		dies     dieList
	}
	Package interface {
		hasUncore
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
	die := c.dies[id]
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
			id:           dieId,
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

func (c *cpuPackage) getID() uint {
	return c.id
}

type (
	cpuDie struct {
		parentSocket Package
		id           uint
		uncore       Uncore
		cores        coreList
		cpus         CpuList
	}
	Die interface {
		topologyTypeObj
		hasUncore
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
	core := d.cores[id]
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

func (d *cpuDie) getID() uint {
	return d.id
}

type (
	cpuCore struct {
		parentDie Die
		id        uint
		cpus      CpuList
		// an array index pointing to a frequency set
		coreType uint
	}
	Core interface {
		topologyTypeObj
		typeSetter
	}
)

func (c *cpuCore) GetType() uint {
	return c.coreType
}

func (c *cpuCore) setType(t uint) {
	c.coreType = t
}

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

func (c *cpuCore) getID() uint {
	return c.id
}

type packageList map[uint]Package

type dieList map[uint]Die

type coreList map[uint]Core

var discoverTopology = func() (Topology, error) {
	numOfCores := getNumberOfCpus()
	topology := &cpuTopology{
		allCpus:  make(CpuList, numOfCores),
		packages: packageList{},
		uncore:   defaultUncore,
	}
	for i := uint(0); i < numOfCores; i++ {
		if _, err := topology.addCpu(i); err != nil {
			return nil, err
		}
	}
	return topology, nil
}
