package power

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockSystemTopology struct {
	mock.Mock
}

func (m *mockSystemTopology) addCpu(u uint) (Cpu, error) {
	ret := m.Called(u)

	var r0 Cpu
	var r1 error

	if ret.Get(0) != nil {
		r0 = ret.Get(0).(Cpu)
	}
	r1 = ret.Error(1)

	return r0, r1
}

func (m *mockSystemTopology) CPUs() *CpuList {
	ret := m.Called()

	var r0 *CpuList
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*CpuList)
	}

	return r0
}

func (m *mockSystemTopology) Packages() *[]Package {
	ret := m.Called()

	var r0 *[]Package
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*[]Package)

	}
	return r0
}

func (m *mockSystemTopology) Package(id uint) Package {
	ret := m.Called(id)

	var r0 Package
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(Package)
	}

	return r0
}

func setupTopologyTest(cpufiles map[string]map[string]string) func() {
	origBasePath := basePath
	basePath = "testing/cpus"

	// backup pointer to function that gets all CPUs
	// replace it with our controlled function
	origGetNumOfCpusFunc := getNumberOfCpus
	getNumberOfCpus = func() uint { return uint(len(cpufiles)) }

	for cpuName, cpuDetails := range cpufiles {
		cpudir := filepath.Join(basePath, cpuName)
		err := os.MkdirAll(filepath.Join(cpudir, "topology"), os.ModePerm)
		if err != nil {
			panic(err)
		}
		for prop, value := range cpuDetails {
			switch prop {
			case "pkg":
				err := os.WriteFile(filepath.Join(cpudir, packageIdFile), []byte(value+"\n"), 0664)
				if err != nil {
					panic(err)
				}
			case "die":
				err := os.WriteFile(filepath.Join(cpudir, dieIdFile), []byte(value+"\n"), 0644)
				if err != nil {
					panic(err)
				}
			case "core":
				err := os.WriteFile(filepath.Join(cpudir, coreIdFile), []byte(value+"\n"), 0644)
				if err != nil {
					panic(err)
				}
			}
		}
	}
	return func() {
		// wipe created cpus dir
		err := os.RemoveAll(strings.Split(basePath, "/")[0])
		if err != nil {
			panic(err)
		}
		// revert cpu /sys path
		basePath = origBasePath
		// revert get number of system cpus function
		getNumberOfCpus = origGetNumOfCpusFunc
	}
}

type topologyTestSuite struct {
	suite.Suite
	origBasePath         string
	origGetNumCpus       func() uint
	origDiscoverTopology func() (Topology, error)
}

func TestTopologyDiscovery(t *testing.T) {
	tstSuite := &topologyTestSuite{
		origBasePath:         basePath,
		origGetNumCpus:       getNumberOfCpus,
		origDiscoverTopology: discoverTopology,
	}
	suite.Run(t, tstSuite)
}
func (s *topologyTestSuite) AfterTest(suiteName, testName string) {
	os.RemoveAll(strings.Split(basePath, "/")[0])
	basePath = s.origBasePath
	discoverTopology = s.origDiscoverTopology
	getNumberOfCpus = s.origGetNumCpus
}

func (s *topologyTestSuite) TestCpuImpl_discoverTopology() {
	t := s.T()
	// 2 packages, 1 die, 2 cores, 2 threads, cpus 0,1,4,5 belong to pkg0, 2,3,6,7 to pkg1, 4-7 are hyperthread cpus
	teardown := setupTopologyTest(map[string]map[string]string{
		"cpu0": {
			"pkg":  "0",
			"die":  "0",
			"core": "0",
		},
		"cpu1": {
			"pkg":  "0",
			"die":  "0",
			"core": "1",
		},
		"cpu2": {
			"pkg":  "1",
			"die":  "0",
			"core": "0",
		},
		"cpu3": {
			"pkg":  "1",
			"die":  "0",
			"core": "1",
		},
		"cpu4": {
			"pkg":  "0",
			"die":  "0",
			"core": "0",
		},
		"cpu5": {
			"pkg":  "0",
			"die":  "0",
			"core": "1",
		},
		"cpu6": {
			"pkg":  "1",
			"die":  "0",
			"core": "0",
		},
		"cpu7": {
			"pkg":  "1",
			"die":  "0",
			"core": "1",
		},
	})
	defer teardown()

	topology, err := discoverTopology()
	assert.NoError(t, err)
	topologyObj := topology.(*systemTopology)

	assert.Len(t, topologyObj.packages, 2)
	assert.Len(t, topologyObj.allCpus, 8)
	assert.ElementsMatch(t, topologyObj.allCpus.IDs(), []uint{0, 1, 2, 3, 4, 5, 6, 7})
	assert.Equal(t, topologyObj.packages[0].id, uint(0))
	assert.Equal(t, topologyObj.packages[1].id, uint(1))

	assert.Len(t, topologyObj.packages[0].dies, 1)
	assert.Len(t, topologyObj.packages[1].dies, 1)
	assert.NotEqual(t, topologyObj.packages[0].dies[0], topologyObj.packages[1].dies[0])
	assert.ElementsMatch(t, topologyObj.packages[0].cpus.IDs(), []uint{0, 1, 4, 5})
	assert.ElementsMatch(t, topologyObj.packages[1].cpus.IDs(), []uint{2, 3, 6, 7})
	// only one die per pkg so pkg cpus == die cpus
	assert.ElementsMatch(t, topologyObj.packages[0].dies[0].cpus, topologyObj.packages[0].cpus)
	assert.ElementsMatch(t, topologyObj.packages[1].dies[0].cpus, topologyObj.packages[1].cpus)

	// emulate hyperthreading enabled so 2 cpus/threads per physical core
	// without hyperthreading we expect one thread per core
	assert.Len(t, topologyObj.packages[0].dies[0].cores, 2)
	assert.Len(t, topologyObj.packages[1].dies[0].cores, 2)

	assert.Len(t, topologyObj.packages[0].dies[0].cpus, 4)
	assert.Len(t, topologyObj.packages[1].dies[0].cpus, 4)

	assert.ElementsMatch(t, topologyObj.packages[0].dies[0].cores[0].cpus.IDs(), []uint{0, 4})
	assert.ElementsMatch(t, topologyObj.packages[0].dies[0].cores[1].cpus.IDs(), []uint{1, 5})
	assert.ElementsMatch(t, topologyObj.packages[1].dies[0].cores[0].cpus.IDs(), []uint{2, 6})
	assert.ElementsMatch(t, topologyObj.packages[1].dies[0].cores[1].cpus.IDs(), []uint{3, 7})
}

func (s *topologyTestSuite) TestSystemTopology_Getters() {
	cpus := make(CpuList, 2)
	cpus[0] = new(cpuMock)
	cpus[1] = new(cpuMock)

	pkgs := packageList{
		0: &cpuPackage{},
		1: &cpuPackage{},
	}

	topo := &systemTopology{
		packages: pkgs,
		allCpus:  cpus,
	}

	assert.ElementsMatch(s.T(), *topo.CPUs(), cpus)
	assert.ElementsMatch(s.T(), *topo.Packages(), []Package{pkgs[0], pkgs[1]})
	assert.Equal(s.T(), topo.Package(1), pkgs[1])
	assert.Nil(s.T(), topo.Package(6))
}
func (s *topologyTestSuite) TestSystemTopology_addCpu() {
	defer setupTopologyTest(map[string]map[string]string{})()
	// fail to read fs
	topo := &systemTopology{
		packages: packageList{},
		allCpus:  make(CpuList, 1),
	}
	cpu, err := topo.addCpu(0)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), cpu)
}

func (s *topologyTestSuite) TestCpuPackage_Getters() {
	cpus := make(CpuList, 2)
	cpus[0] = new(cpuMock)
	cpus[1] = new(cpuMock)

	dice := dieList{
		0: &cpuDie{},
		1: &cpuDie{},
	}

	pkg := &cpuPackage{
		dies: dice,
		cpus: cpus,
	}

	assert.ElementsMatch(s.T(), *pkg.CPUs(), cpus)
	assert.ElementsMatch(s.T(), *pkg.Dies(), []Die{dice[0], dice[1]})
	assert.Equal(s.T(), pkg.Die(1), dice[1])
	assert.Nil(s.T(), pkg.Die(6))
}
func (s *topologyTestSuite) TestCpuPackage_addCpu() {
	defer setupTopologyTest(map[string]map[string]string{})()
	// fail to read fs
	pkg := &cpuPackage{
		dies: dieList{},
		cpus: make(CpuList, 1),
	}
	cpu, err := pkg.addCpu(0)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), cpu)
}

func (s *topologyTestSuite) TestCpuDie_Getters() {
	cpus := make(CpuList, 2)
	cpus[0] = new(cpuMock)
	cpus[1] = new(cpuMock)

	cores := coreList{
		0: &cpuCore{},
		1: &cpuCore{},
	}

	die := &cpuDie{
		cores: cores,
		cpus:  cpus,
	}

	assert.ElementsMatch(s.T(), *die.CPUs(), cpus)
	assert.ElementsMatch(s.T(), *die.Cores(), []Core{cores[0], cores[1]})
	assert.Equal(s.T(), die.Core(1), cores[1])
	assert.Nil(s.T(), die.Core(6))
}
func (s *topologyTestSuite) TestCpuDie_addCpu() {
	defer setupTopologyTest(map[string]map[string]string{})()
	// fail to read fs
	pkg := &cpuPackage{
		dies: dieList{},
		cpus: make(CpuList, 1),
	}
	cpu, err := pkg.addCpu(0)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), cpu)
}

func (s *topologyTestSuite) TestCpuCore_Getters() {
	cpus := make(CpuList, 2)
	cpus[0] = new(cpuMock)
	cpus[1] = new(cpuMock)

	core := &cpuCore{
		cpus: cpus,
	}

	assert.ElementsMatch(s.T(), *core.CPUs(), cpus)
}
