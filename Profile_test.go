package power

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProfileImpl_Profile(t *testing.T) {
	assert := assert.New(t)
	profileNameInit := "pool name"
	profileNameNew := "unit tests :("

	profile := &profileImpl{Name: profileNameInit}
	assert.Equal(profileNameInit, profile.GetName())

	profile.SetProfileName(profileNameNew)
	assert.Equal(profileNameNew, profile.Name)
}

func TestNewProfile(t *testing.T) {
	profile := NewProfile("name", 0, 100, "epp")

	assert.Equal(t, "name", profile.(*profileImpl).Name)
	assert.Equal(t, 0, profile.(*profileImpl).Min)
	assert.Equal(t, 100, profile.(*profileImpl).Max)
	assert.Equal(t, "epp", profile.(*profileImpl).Epp)
}
