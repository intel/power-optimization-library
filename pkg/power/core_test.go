package power

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type coreMock struct {
	mock.Mock
}

func (m *coreMock) SetCStates(cStates CStates) error {
	return m.Called(cStates).Error(0)
}

func (m *coreMock) _setPoolProperty(pool Pool) {
	m.Called(pool)
}
func (m *coreMock) consolidate() error {
	return m.Called().Error(0)
}
func (m *coreMock) doSetPool(pool Pool) error {
	return m.Called(pool).Error(0)
}
func (m *coreMock) GetID() uint {
	args := m.Called()
	return args.Get(0).(uint)
}
func (m *coreMock) getPool() Pool {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	} else {
		return args.(Pool)
	}
}
func (m *coreMock) SetPool(pool Pool) error {
	return m.Called(pool).Error(0)
}

func setupCoreTests(cpufiles map[string]map[string]string) func() {
	origBasePath := basePath
	basePath = "testing/cores"
	defaultDefaultPowerProfile := defaultPowerProfile

	// backup pointer to function that gets all CPUs
	// replace it with our controlled function
	origGetNumOfCpusFunc := getNumberOfCpus
	getNumberOfCpus = func() uint { return uint(len(cpufiles)) }

	// "initialise" P-States feature
	featureList[PStatesFeature].err = nil

	// if cpu0 is here we set its values to temporary defaultPowerProfile
	if cpu0, ok := cpufiles["cpu0"]; ok {
		defaultPowerProfile = &profileImpl{}
		if max, ok := cpu0["max"]; ok {
			max, _ := strconv.Atoi(max)
			defaultPowerProfile.max = max
		}
		if min, ok := cpu0["min"]; ok {
			min, _ := strconv.Atoi(min)
			defaultPowerProfile.min = min
		}
		if governor, ok := cpu0["governor"]; ok {
			defaultPowerProfile.governor = governor
		}
		if epp, ok := cpu0["epp"]; ok {
			defaultPowerProfile.epp = epp
		}
	}
	for cpuName, cpuDetails := range cpufiles {
		cpudir := filepath.Join(basePath, cpuName)
		os.MkdirAll(filepath.Join(cpudir, "cpufreq"), os.ModePerm)
		for prop, value := range cpuDetails {
			switch prop {
			case "driver":
				os.WriteFile(filepath.Join(cpudir, pStatesDrvFile), []byte(value+"\n"), 0664)
			case "max":
				os.WriteFile(filepath.Join(cpudir, scalingMaxFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(cpudir, cpuMaxFreqFile), []byte(value+"\n"), 0644)
			case "min":
				os.WriteFile(filepath.Join(cpudir, scalingMinFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(cpudir, cpuMinFreqFile), []byte(value+"\n"), 0644)
			case "epp":
				os.WriteFile(filepath.Join(cpudir, eppFile), []byte(value+"\n"), 0644)
			case "governor":
				os.WriteFile(filepath.Join(cpudir, scalingGovFile), []byte(value+"\n"), 0644)
			}
		}
	}
	return func() {
		// wipe created cores dir
		os.RemoveAll(strings.Split(basePath, "/")[0])
		// revert cpu /sys path
		basePath = origBasePath
		// revert get number of system cpus function
		getNumberOfCpus = origGetNumOfCpusFunc
		// revert p-states feature to un initialised state
		featureList[PStatesFeature].err = uninitialisedErr
		// revert default powerProfile
		defaultPowerProfile = defaultDefaultPowerProfile
	}
}

func TestNewCore(t *testing.T) {
	cpufiles := map[string]map[string]string{
		"cpu0": {
			"max": "123",
			"min": "100",
			"epp": "some",
		},
	}
	defer setupCoreTests(cpufiles)()

	// happy path - ensure values from files are read correctly

	core, err := newCore(0)
	assert.NoError(t, err)

	assert.NotNil(t, core.(*coreImpl).mutex)
	// we don't want to compare value of new mutex, so we set it to nil
	core.(*coreImpl).mutex = nil
	assert.Equal(t, &coreImpl{
		id: 0,
	}, core)

	// now "break" P-States by setting a feature error
	featureList[PStatesFeature].err = fmt.Errorf("some error")

	core, err = newCore(0)

	assert.NoError(t, err)

	assert.NotNil(t, core.(*coreImpl).mutex)
	// Ensure P-States stuff was never read by ensuring related properties are 0
	core.(*coreImpl).mutex = nil
	assert.Equal(t, &coreImpl{
		id: 0,
	}, core)
}

func TestCoreImpl_SetPool(t *testing.T) {
	// feature errors are set so functions inside consolidate() return without doing anything
	host := new(hostMock)

	sharedPool := new(poolMock)
	sharedPool.On("isExclusive").Return(false)
	sharedPool.On("getHost").Return(host)
	sharedPool.On("Name").Return("shared")
	sharedPoolCores := make(CoreList, 8)
	sharedPool.On("Cores").Return(&sharedPoolCores)

	reservedPool := new(poolMock)
	reservedPool.On("isExclusive").Return(false)
	reservedPool.On("getHost").Return(host)
	reservedPool.On("Name").Return("reserved")
	reservedPoolCores := make(CoreList, 8)
	reservedPool.On("Cores").Return(&reservedPoolCores)

	host.On("GetReservedPool").Return(reservedPool)
	host.On("GetSharedPool").Return(sharedPool)

	exclusivePool1 := new(poolMock)
	exclusivePool1.On("isExclusive").Return(true)
	exclusivePool1.On("getHost").Return(host)
	exclusivePool1.On("Name").Return("excl1")
	exclusivePool1Cores := make(CoreList, 8)
	exclusivePool1.On("Cores").Return(&exclusivePool1Cores)

	exclusivePool2 := new(poolMock)
	exclusivePool2.On("isExclusive").Return(true)
	exclusivePool2.On("getHost").Return(host)
	exclusivePool2.On("Name").Return("excl2")
	exclusivePool2Cores := make(CoreList, 8)
	exclusivePool2.On("Cores").Return(&exclusivePool2Cores)

	core := &coreImpl{
		id:    0,
		mutex: &sync.Mutex{},
		pool:  sharedPool,
	}
	// nil pool
	assert.ErrorContains(t, core.SetPool(nil), "cannot be nil")

	// current == target pool, case 0
	assert.NoError(t, core.SetPool(sharedPool))
	sharedPool.AssertNotCalled(t, "isExclusive")
	assert.True(t, core.pool == sharedPool)

	// shared to reserved
	sharedPoolCores[0] = core
	core.pool = sharedPool
	assert.NoError(t, core.SetPool(reservedPool))
	assert.True(t, core.pool == reservedPool)

	// shared to shared
	core.pool = sharedPool
	sharedPoolCores[0] = core
	assert.NoError(t, core.SetPool(sharedPool))
	assert.True(t, core.pool == sharedPool)

	// shared to exclusive
	core.pool = sharedPool
	sharedPoolCores[0] = core
	assert.NoError(t, core.SetPool(exclusivePool1))
	assert.True(t, core.pool == exclusivePool1)

	// reserved to reserved
	core.pool = reservedPool
	reservedPoolCores[0] = core
	assert.NoError(t, core.SetPool(reservedPool))
	assert.True(t, core.pool == reservedPool)

	// reserved to shared
	core.pool = reservedPool
	reservedPoolCores[0] = core
	assert.NoError(t, core.SetPool(sharedPool))
	assert.True(t, core.pool == sharedPool)

	// reserved to exclusive
	core.pool = reservedPool
	reservedPoolCores[0] = core
	assert.ErrorContains(t, core.SetPool(exclusivePool1), "reserved to exclusive")
	assert.True(t, core.pool == reservedPool)

	// exclusive to reserved
	core.pool = exclusivePool1
	exclusivePool1Cores[0] = core
	assert.ErrorContains(t, core.SetPool(reservedPool), "exclusive to reserved")
	assert.True(t, core.pool == exclusivePool1)

	// exclusive to shared
	core.pool = exclusivePool1
	exclusivePool1Cores[0] = core
	assert.NoError(t, core.SetPool(sharedPool))
	assert.True(t, core.pool == sharedPool)

	// exclusive to same exclusive
	core.pool = exclusivePool1
	exclusivePool1Cores[0] = core
	assert.NoError(t, core.SetPool(exclusivePool1))
	assert.True(t, core.pool == exclusivePool1)

	//exclusive to another exclusive
	core.pool = exclusivePool1
	exclusivePool1Cores[0] = core
	assert.ErrorContains(t, core.SetPool(exclusivePool2), " exclusive to different exclusive")
	assert.True(t, core.pool == exclusivePool1)
}

func TestCoreImpl_doSetPool(t *testing.T) {
	var sourcePool, targetPool *poolMock
	var core *coreImpl
	// happy path
	sourcePool = new(poolMock)
	sourcePool.On("Name").Return("sauce")

	targetPool = new(poolMock)
	targetPool.On("name").Return("target")
	core = &coreImpl{
		pool:  sourcePool,
		mutex: &sync.Mutex{},
	}
	sourcePool.On("Cores").Return(&CoreList{core})
	targetPool.On("Cores").Return(&CoreList{})

	assert.NoError(t, core.doSetPool(targetPool))
	assert.True(t, core.pool == targetPool)

	// remove failure
	sourcePool = new(poolMock)
	sourcePool.On("Name").Return("sauce")

	targetPool = new(poolMock)
	targetPool.On("name").Return("target")
	core = &coreImpl{
		pool:  sourcePool,
		mutex: &sync.Mutex{},
	}
	sourcePool.On("Cores").Return(&CoreList{})
	targetPool.On("Cores").Return(&CoreList{})

	assert.ErrorContains(t, core.doSetPool(targetPool), "not in pool")
	assert.True(t, core.pool == sourcePool)
}

func TestCoreImpl_getAllCores(t *testing.T) {
	teardown := setupCoreTests(map[string]map[string]string{
		"cpu0": {
			"max": "123",
			"min": "100",
		},
		"cpu1": {
			"max": "124",
			"min": "99",
		},
	})
	defer teardown()

	cores, err := getAllCores()
	assert.NoError(t, err)

	assert.Len(t, cores, 2)
	assert.Equal(t, cores[0], &coreImpl{
		id:    0,
		mutex: cores[0].(*coreImpl).mutex,
	})
	assert.Equal(t, cores[1], &coreImpl{
		id:    1,
		mutex: cores[1].(*coreImpl).mutex,
	})
}

func TestCoreList_IDs(t *testing.T) {
	cores := CoreList{}
	var expectedIDs []uint
	for i := uint(0); i < 5; i++ {
		mockedCore := new(coreMock)
		mockedCore.On("GetID").Return(i)
		cores = append(cores, mockedCore)
		expectedIDs = append(expectedIDs, i)
	}
	assert.ElementsMatch(t, cores.IDs(), expectedIDs)
}

func TestCoreList_ByID(t *testing.T) {
	// test for quick get to skip iteration over list when index == coreId
	cores := CoreList{}
	for i := uint(0); i < 5; i++ {
		mockedCore := new(coreMock)
		mockedCore.On("GetID").Return(i)
		cores = append(cores, mockedCore)
	}

	assert.Equal(t, cores[2], cores.ByID(2))
	cores[0].(*coreMock).AssertNotCalled(t, "GetID")
	cores[1].(*coreMock).AssertNotCalled(t, "GetID")

	// test for when index != coreID and have to iterate
	cores = CoreList{}
	for _, u := range []uint{56, 1, 6, 99, 2, 11} {
		mocked := new(coreMock)
		mocked.On("GetID").Return(u)
		cores = append(cores, mocked)
	}
	assert.Equal(t, cores[3], cores.ByID(99))
	assert.Equal(t, cores[5], cores.ByID(11))

	// not in list
	assert.Nil(t, cores.ByID(77))
}

func TestCoreList_ManyByIDs(t *testing.T) {
	cores := CoreList{}
	for i := uint(0); i < 5; i++ {
		mockedCore := new(coreMock)
		mockedCore.On("GetID").Return(i)
		cores = append(cores, mockedCore)
	}
	returnedList, err := cores.ManyByIDs([]uint{1, 3})
	assert.ElementsMatch(t, returnedList, []Core{cores[1], cores[3]})
	assert.NoError(t, err)

	// out of range#
	returnedList, err = cores.ManyByIDs([]uint{6})
	assert.Error(t, err)
}
