package power

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func Test_diff(t *testing.T) {
	all := make([]Core, 8)
	for i, _ := range all {
		mck := new(coreMock)
		mck.On("GetID").Return(i)
		all[i] = mck
	}
	excluded := []Core{
		all[1], all[3], all[5],
	}

	difference := diffCoreList(all, excluded)
	assert.ElementsMatch(t, difference, []Core{all[0], all[2], all[4], all[6], all[7]})
}

func TestCreateInstance(t *testing.T) {
	nodeName := "node1"
	mockCpuData := map[string]string{
		"min": "100",
		"max": "123",
		"epp": "epp",
	}
	mockedCores := map[string]map[string]string{
		"cpu0": mockCpuData,
		"cpu1": mockCpuData,
		"cpu2": mockCpuData,
		"cpu3": mockCpuData,
	}
	defer setupCoreTests(mockedCores)()

	node, err := CreateInstance(nodeName)
	assert.Nil(t, err)

	assert.Equal(t, nodeName, node.GetName())
	assert.Len(t, node.(*nodeImpl).SharedPool.(*poolImpl).Cores, len(mockedCores))
	assert.Equal(t, sharedPoolName, node.(*nodeImpl).SharedPool.(*poolImpl).Name)

	assert.Empty(t, node.(*nodeImpl).ExclusivePools)
}

func TestPreChecks(t *testing.T) {
	defer setupCoreTests(map[string]map[string]string{
		"cpu0": {},
	})()

	assert.Nil(t, preChecks())

	os.WriteFile(filepath.Join(basePath, "cpu0", scalingDrvFile), []byte("not intel\n"), 0664)
	assert.Error(t, preChecks())
}
