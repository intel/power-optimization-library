package power

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProfileImpl_Profile(t *testing.T) {
	assert := assert.New(t)
	profileNameInit := "pool Name"
	profileNameNew := "unit tests :("

	profile := &profileImpl{Name: profileNameInit}
	assert.Equal(profileNameInit, profile.GetName())

	profile.SetProfileName(profileNameNew)
	assert.Equal(profileNameNew, profile.Name)
}

func TestNewProfile(t *testing.T) {
	profile, err := NewProfile("Name", 0, 100, cpuPolicyPowersave, "epp")

	assert.NoError(t, err)

	assert.Equal(t, "Name", profile.(*profileImpl).Name)
	assert.Equal(t, 0, profile.(*profileImpl).Min)
	assert.Equal(t, 100*1000, profile.(*profileImpl).Max)
	assert.Equal(t, "powersave", profile.(*profileImpl).Governor)
	assert.Equal(t, "epp", profile.(*profileImpl).Epp)

	profile, err = NewProfile("Name", 0, 10, cpuPolicyPerformance, cpuPolicyPerformance)
	assert.NoError(t, err)
	assert.NotNil(t, profile)

	profile, err = NewProfile("Name", 0, 100, cpuPolicyPerformance, "epp")
	assert.Error(t, err)
	assert.Nil(t, profile)

	profile, err = NewProfile("Name", 100, 0, cpuPolicyPowersave, "epp")
	assert.Error(t, err)
	assert.Nil(t, profile)

	profile, err = NewProfile("Name", 0, 100, "something random", "epp")
	assert.Error(t, err)
	assert.Nil(t, profile)
}
