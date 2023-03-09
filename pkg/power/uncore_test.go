package power

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func setupUncoreTests(files map[string]map[string]string, modulesFileContent string) func() {
	origBasePath := basePath
	basePath = "testing/uncore"

	origModulesFile := kernelModulesFilePath
	kernelModulesFilePath = basePath + "/kernelModules"

	if err := os.MkdirAll(filepath.Join(basePath, uncoreDirName), os.ModePerm); err != nil {
		panic(err)
	}

	if modulesFileContent != "" {
		if err := os.WriteFile(kernelModulesFilePath, []byte(modulesFileContent), 0644); err != nil {
			panic(err)
		}
	}

	for pkgDie, freqFiles := range files {
		pkgUncoreDir := filepath.Join(basePath, uncoreDirName, pkgDie)
		if err := os.MkdirAll(filepath.Join(pkgUncoreDir), os.ModePerm); err != nil {
			panic(err)
		}
		for file, value := range freqFiles {
			switch file {
			case "initMax":
				if err := os.WriteFile(path.Join(pkgUncoreDir, uncoreInitMaxFreqFile), []byte(value), 0644); err != nil {
					panic(err)
				}
			case "initMin":
				if err := os.WriteFile(path.Join(pkgUncoreDir, uncoreInitMinFreqFile), []byte(value), 0644); err != nil {
					panic(err)
				}
			}
		}
	}
	return func() {
		if err := os.RemoveAll(strings.Split(basePath, "/")[0]); err != nil {
			panic(err)
		}
		kernelModulesFilePath = origModulesFile
		basePath = origBasePath

		uncoreInitMaxFreq = 0
		uncoreInitMinFreq = 0
	}
}
func Test_initUncore(t *testing.T) {
	var feature featureStatus
	var teardown func()
	teardown = setupUncoreTests(map[string]map[string]string{
		"package_00_die_00": {
			"initMax": "999",
			"initMin": "100",
		},
	},
		"intel_cstates 14 0 - Live 0000ffffad212d\n"+
			uncoreKmodName+" 324 0 - Live 0000ffff3ea334\n"+
			"rtscan 2342 0 -Live 0000ffff234ab4d",
	)

	// happy path
	feature = initUncore()

	assert.Equal(t, "Uncore frequency", feature.name)
	assert.Equal(t, "N/A", feature.driver)

	assert.NoError(t, feature.err)
	assert.Equal(t, uint(999), uncoreInitMaxFreq)
	assert.Equal(t, uint(100), uncoreInitMinFreq)
	teardown()

	// module not loaded
	teardown = setupUncoreTests(map[string]map[string]string{},
		"intel_cstates 14 0 - Live 0000ffffad212d\n"+
			"rtscan 2342 0 -Live 0000ffff234ab4d",
	)
	feature = initUncore()
	assert.ErrorContains(t, feature.err, "not loaded")
	teardown()

	// no dies to manage
	teardown = setupUncoreTests(map[string]map[string]string{
		//"package_00_die_00":{},
	},
		"intel_cstates 14 0 - Live 0000ffffad212d\n"+
			uncoreKmodName+" 324 0 - Live 0000ffff3ea334\n"+
			"rtscan 2342 0 -Live 0000ffff234ab4d",
	)
	feature = initUncore()
	assert.ErrorContains(t, feature.err, "empty or invalid")
	teardown()

	// cant read init freqs
	teardown = setupUncoreTests(map[string]map[string]string{
		"package_00_die_00": {},
	},
		"intel_cstates 14 0 - Live 0000ffffad212d\n"+
			uncoreKmodName+" 324 0 - Live 0000ffff3ea334\n"+
			"rtscan 2342 0 -Live 0000ffff234ab4d",
	)
	feature = initUncore()
	assert.ErrorContains(t, feature.err, "failed to determine init freq")
}
