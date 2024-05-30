package power

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProfile(t *testing.T) {
	oldgovs := availableGovs
	availableGovs = []string{cpuPolicyPowersave, cpuPolicyPerformance}

	profile, err := NewPowerProfile("name", 0, 100, cpuPolicyPowersave, "epp")
	assert.ErrorIs(t, err, uninitialisedErr)
	assert.Nil(t, profile)

	featureList[FrequencyScalingFeature].err = nil
	featureList[EPPFeature].err = nil
	defer func() { featureList[FrequencyScalingFeature].err = uninitialisedErr }()
	defer func() { featureList[EPPFeature].err = uninitialisedErr }()
	defer func() { availableGovs = oldgovs }()

	profile, err = NewPowerProfile("name", 0, 100, cpuPolicyPowersave, "epp")
	assert.NoError(t, err)
	assert.Equal(t, "name", profile.(*profileImpl).name)
	assert.Equal(t, uint(0), profile.(*profileImpl).min)
	assert.Equal(t, uint(100*1000), profile.(*profileImpl).max)
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
	assert.ErrorContains(t, err, "governor can only be set to the following")
	assert.Nil(t, profile)
}

func TestEfficientProfile(t *testing.T) {
	oldGovs := availableGovs
	availableGovs = []string{cpuPolicyPowersave, cpuPolicyPerformance}
	featureList[FrequencyScalingFeature].err = nil
	featureList[EPPFeature].err = nil
	typeCopy := coreTypes

	//reset values afterwards
	defer func() { featureList[FrequencyScalingFeature].err = uninitialisedErr }()
	defer func() { featureList[EPPFeature].err = uninitialisedErr }()
	defer func() { coreTypes = typeCopy }()
	defer func() { availableGovs = oldGovs }()

	coreTypes = CoreTypeList{&CpuFrequencySet{min: 300, max: 1000}, &CpuFrequencySet{min: 300, max: 500}}

	//default scenario
	profile, err := NewEcorePowerProfile("name", 300, 1000, 300, 450, cpuPolicyPerformance, cpuPolicyPerformance)
	assert.NoError(t, err)
	assert.NotNil(t, profile)

	// invalid frequency ranges
	profile, err = NewEcorePowerProfile("name", 300, 1000, 430, 200, cpuPolicyPerformance, cpuPolicyPerformance)
	assert.ErrorContains(t, err, "max Freq can't be lower than min")
	assert.Nil(t, profile)

}
