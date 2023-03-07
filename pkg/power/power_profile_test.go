package power

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewProfile(t *testing.T) {
	profile, err := NewPowerProfile("name", 0, 100, cpuPolicyPowersave, "epp")

	assert.ErrorIs(t, err, uninitialisedErr)

	featureList[PStatesFeature].err = nil
	defer func() { featureList[PStatesFeature].err = uninitialisedErr }()

	profile, err = NewPowerProfile("name", 0, 100, cpuPolicyPowersave, "epp")
	assert.Equal(t, "name", profile.(*profileImpl).name)
	assert.Equal(t, 0, profile.(*profileImpl).min)
	assert.Equal(t, 100*1000, profile.(*profileImpl).max)
	assert.Equal(t, "powersave", profile.(*profileImpl).governor)
	assert.Equal(t, "epp", profile.(*profileImpl).epp)

	profile, err = NewPowerProfile("name", 0, 10, cpuPolicyPerformance, cpuPolicyPerformance)
	assert.NoError(t, err)
	assert.NotNil(t, profile)

	profile, err = NewPowerProfile("name", 0, 100, cpuPolicyPerformance, "epp")
	assert.ErrorContains(t, err, fmt.Sprintf("'%s' epp can be used with '%s' governor", cpuPolicyPerformance, cpuPolicyPerformance))
	assert.Nil(t, profile)

	profile, err = NewPowerProfile("name", 100, 0, cpuPolicyPowersave, "epp")
	assert.ErrorContains(t, err, "max Freq can't be lower than min")
	assert.Nil(t, profile)

	profile, err = NewPowerProfile("name", 0, 100, "something random", "epp")
	assert.ErrorContains(t, err, fmt.Sprintf("governor can only be set to '%s' or '%s'", cpuPolicyPerformance, cpuPolicyPowersave))
	assert.Nil(t, profile)
}
