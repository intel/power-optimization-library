package power

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

func TestPoolImpl_AddCore(t *testing.T) {
	assert := assert.New(t)

	mockCore := new(coreMock)
	p := &poolImpl{
		PowerProfile: &profileImpl{
			Max: 123,
			Min: 100,
			Epp: "epp",
		},
	}

	// happy path
	mockCore.On("updateValues", "epp", 100, 123).Return(nil)
	assert.NoError(p.addCore(mockCore))
	mockCore.AssertExpectations(t)
	assert.Contains(p.Cores, mockCore)

	// attempting to add existing core - expecting error
	mockCore.On("GetID").Return(0)
	assert.Error(p.addCore(mockCore))

	// Simulate failure to update cpu files
	mockCore = new(coreMock)
	mockCore.On("updateValues", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("test err"))
	assert.Error(p.addCore(mockCore))
	mockCore.AssertCalled(t, "updateValues", mock.Anything, mock.Anything, mock.Anything)
}

func TestPoolImpl_RemoveCore(t *testing.T) {
	assert := assert.New(t)
	mockCore0 := new(coreMock)
	mockCore0.On("GetID").Return(0)
	mockCore0.On("getReserved").Return(false)

	mockCore1 := new(coreMock)
	mockCore1.On("GetID").Return(1)
	mockCore1.On("getReserved").Return(false)

	pool := &poolImpl{Cores: []Core{mockCore0, mockCore1}}

	// Happy path test
	assert.Nil(pool.removeCore(mockCore1))
	assert.NotContains(pool.Cores, mockCore1)
	assert.Contains(pool.Cores, mockCore0)

	// attempt to remove core not in a pool
	assert.Error(pool.removeCore(mockCore1))
}

func TestPoolImpl_RemoveCoreById(t *testing.T) {
	assert := assert.New(t)
	mockCore0 := new(coreMock)
	mockCore0.On("GetID").Return(0)
	mockCore0.On("getReserved").Return(false)

	mockCore1 := new(coreMock)
	mockCore1.On("GetID").Return(1)
	mockCore1.On("getReserved").Return(false)

	pool := &poolImpl{Cores: []Core{mockCore0, mockCore1}}

	// Happy path test
	_, err := pool.removeCoreByID(0)
	assert.NoError(err)
	assert.NotContains(pool.Cores, mockCore0)
	assert.Contains(pool.Cores, mockCore1)

	// attempt to remove core not in a pool
	_, err = pool.removeCoreByID(0)
	assert.Error(err)
}
