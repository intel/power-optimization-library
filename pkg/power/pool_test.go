package power

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"testing"
)

type mockPool struct {
	mock.Mock
}

func (m *mockPool) GetName() string {
	return ""
}

func (m *mockPool) addCore(core Core) error {
	args := m.Called(core)
	return args.Error(0)
}

func (m *mockPool) removeCore(core Core) error {
	args := m.Called(core)
	return args.Error(0)
}

func (m *mockPool) removeCoreByID(coreID int) (Core, error) {
	args := m.Called(coreID)
	return args.Get(0).(Core), args.Error(1)
}

func (m *mockPool) SetPowerProfile(profile Profile) error {
	args := m.Called(profile)
	return args.Error(0)
}

func (m *mockPool) GetPowerProfile() Profile {
	args := m.Called()
	return args.Get(0).(Profile)
}

func (m *mockPool) GetCores() []Core {
	args := m.Called()
	return args.Get(0).([]Core)
}

func (m *mockPool) GetCoreIds() []int {
	args := m.Called()
	return args.Get(0).([]int)
}

<<<<<<< HEAD
func (m *mockPool) SetCStates(states map[string]bool) error {
=======
func (m *mockPool) SetCStates(states CStates) error {
>>>>>>> internal/main
	args := m.Called(states)
	return args.Error(0)
}

<<<<<<< HEAD
func (m *mockPool) getCStates() map[string]bool {
	args := m.Called()
	return args.Get(0).(map[string]bool)
=======
func (m *mockPool) getCStates() CStates {
	args := m.Called()
	return args.Get(0).(CStates)
>>>>>>> internal/main
}

type poolTestSuite struct {
	suite.Suite
}

func TestPool(t *testing.T) {
	suite.Run(t, new(poolTestSuite))
}
func (s *poolTestSuite) BeforeTest(suiteName, testName string) {
	supportedFeatureErrors[PStatesFeature] = nil
	supportedFeatureErrors[CStatesFeature] = nil
	cStatesNamesMap = map[string]int{"C1": 1, "C2": 2}
}

func (s *poolTestSuite) AfterTest(suiteName, testName string) {
	supportedFeatureErrors[PStatesFeature] = &uninitialisedErr
	supportedFeatureErrors[CStatesFeature] = &uninitialisedErr
	cStatesNamesMap = map[string]int{}
}

func (s *poolTestSuite) TestAddCore() {
	mockCore := new(coreMock)
	p := &poolImpl{
		PowerProfile: &profileImpl{
			Max:      123,
			Min:      100,
			Epp:      "epp",
			Governor: cpuPolicyPowersave,
		},
	}

	// happy path
	mockCore.On("setPool", mock.Anything).Return()
	mockCore.On("updateFreqValues", cpuPolicyPowersave, "epp", 100, 123).Return(nil)
	mockCore.On("exclusiveCStates").Return(true)
	s.NoError(p.addCore(mockCore))
	mockCore.AssertExpectations(s.T())
	s.Contains(p.Cores, mockCore)

	// attempting to add existing core - expecting Error
	mockCore.On("GetID").Return(0)
	s.Error(p.addCore(mockCore))

	// Simulate failure to update cpu files
	mockCore = new(coreMock)
	mockCore.On("exclusiveCStates").Return(true)
	mockCore.On("updateFreqValues", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("test err"))
	s.Error(p.addCore(mockCore))
	mockCore.AssertCalled(s.T(), "updateFreqValues", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	// ensure no attempt to make changes is made if SSTBF is not supported on the system
	mockCore = new(coreMock)
	mockCore.On("exclusiveCStates").Return(true)
	mockCore.On("setPool", p)
	e := errors.New("Error")
	supportedFeatureErrors[PStatesFeature] = &e
	s.NoError(p.addCore(mockCore))
	mockCore.AssertNotCalled(s.T(), "updateFreqValues")
}

func (s *poolTestSuite) TestRemoveCore() {
	mockCore0 := new(coreMock)
	mockCore0.On("GetID").Return(0)
	mockCore0.On("getReserved").Return(false)

	mockCore1 := new(coreMock)
	mockCore1.On("GetID").Return(1)
	mockCore1.On("getReserved").Return(false)

	pool := &poolImpl{Cores: []Core{mockCore0, mockCore1}}

	// Happy path test
	s.Nil(pool.removeCore(mockCore1))
	s.NotContains(pool.Cores, mockCore1)
	s.Contains(pool.Cores, mockCore0)

	// attempt to remove core not in a pool
	s.Error(pool.removeCore(mockCore1))
}

func (s *poolTestSuite) TestRemoveCoreById() {
	mockCore0 := new(coreMock)
	mockCore0.On("GetID").Return(0)
	mockCore0.On("getReserved").Return(false)

	mockCore1 := new(coreMock)
	mockCore1.On("GetID").Return(1)
	mockCore1.On("getReserved").Return(false)

	pool := &poolImpl{Cores: []Core{mockCore0, mockCore1}}

	// Happy path test
	_, err := pool.removeCoreByID(0)
	s.NoError(err)
	s.NotContains(pool.Cores, mockCore0)
	s.Contains(pool.Cores, mockCore1)

	// attempt to remove core not in a pool
	_, err = pool.removeCoreByID(0)
	s.Error(err)
}

func (s *poolTestSuite) TestSetCStates() {
	states := CStates{"C2": true, "C1": false}
	cores := make([]Core, 2)
	for i := range cores {
		core := new(coreMock)
		core.On("applyCStates", states).Return(nil)
		core.On("exclusiveCStates").Return(false)
		cores[i] = core
	}
	supportedFeatureErrors[CStatesFeature] = nil

	assert.NoError(s.T(), (&poolImpl{Cores: cores}).SetCStates(states))

	for _, core := range cores {
		core.(*coreMock).AssertExpectations(s.T())
	}

	// fail to apply c states on one core
	cores = make([]Core, 2)
	for i := range cores {
		core := new(coreMock)
		core.On("exclusiveCStates").Return(false)
		cores[i] = core
	}
	cores[0].(*coreMock).On("applyCStates", states).Return(errors.New("apply fail"))
	cores[1].(*coreMock).On("applyCStates", states).Return(nil)

	assert.Error(s.T(), (&poolImpl{Cores: cores}).SetCStates(states))

	for _, core := range cores {
		core.(*coreMock).AssertExpectations(s.T())
	}

	cores = make([]Core, 2)
	for i := range cores {
		core := new(coreMock)
		cores[i] = core
	}

	delete(cStatesNamesMap, "C2")
	assert.Error(s.T(), (&poolImpl{Cores: cores}).SetCStates(states))
	for _, core := range cores {
		core.(*coreMock).AssertNotCalled(s.T(), "applyCStates")
	}

	e := errors.New("")
	supportedFeatureErrors[CStatesFeature] = &e
	assert.ErrorIs(s.T(), (&poolImpl{}).SetCStates(states), *supportedFeatureErrors[CStatesFeature])
}
