# Intel® Power Optimization Library

The Intel Power Optimization Library is an open source library that takes the desired configuration of the user to tune
the frequencies and set the priority level of the cores.

# Overview

The Power Optimization Library takes allows management of CPUs power/performance on via Pool based management

This library is currently used as part of the
[Kubernetes Power Manager](https://github.com/intel/kubernetes-power-manager), but could be used with other utilities.

## Features

* Pool based Frequency Tuning
* Facilitate use of Intel SST (Speed Select Technology) Suite
    * SST-BF - Speed Select Technology - Base Frequency
    * SST-CP - Speed Select Technology - Core Power
* C-States control
* Uncore frequency
* CPU Topology discovery and awareness

# Prerequisites

- Linux based OS
- P-States
    - ``intel_pstates`` enabled - no entries in kernel cmdline disabling it
- C-States
    - ``intel_cstates`` kernel module loaded
- Uncore frequency
    - kernel 5.6+ compiled with ``CONFIG_INTEL_UNCORE_FREQ_CONTROL``
    - ``intel-uncore-frequency`` kernel module loaded

**Note:** on Ubuntu systems for Uncore frequency feature a ``linux-generic-hwe`` kernel is required

# Definitions

**Intel SST-BF (Speed Select - Base Frequency)** - allows the user to control the base frequency of certain cores.
The base frequency is a guaranteed level of performance on the CPU (a CPU will never go below its base frequency).
Priority cores can be set to a higher base frequency than a majority of the other cores on the system to which they
can apply their critical workloads for a guaranteed performance.

Frequency tuning allows the individual CPUs on the system to be sped up or slowed down by changing their frequency.

**Intel SST-CP (Speed Select Technology - Core Power)** - allows the user to group cores into levels of priority.
When there is power to spare on the system, it can be distributed among the cores based on their priority level.
While it is not guaranteed that the extra power will be applied to the highest priority cores, the system will do its
best to do so.

There are four levels of priority available:

1. Performance
2. Balance Performance
3. Balance Power
4. Power

The Priority level for a core is defined using its **EPP (Energy Performance Preference)** value, which is one of the
options in the Power Profiles. If not all the power is utilized on the CPU, the CPU can put the higher priority cores
up to Turbo Frequency (allows the cores to run faster).

**C-States** To save energy on a system, you can command the CPU to go into a low-power mode. Each CPU has several power
modes, which
are collectively called C-States. These work by cutting the clock signal and power from idle CPUs, or CPUs that are not
executing commands.While you save more energy by sending CPUs into deeper C-State modes, it does take more time for the
CPU to fully “wake up” from sleep mode, so there is a trade-off when it comes to deciding the depth of sleep.

**Uncore** equates to logic outside the CPU cores but residing on the same die. Traffic (for example, Data Reads)
generated by threads executing on CPU cores or IO devices may be operated on by logic in the Uncore. Logic responsible
for managing coherency, managing access to the DIMMs, managing power distribution and sleep states, and so forth.

**Uncore Frequency**  the frequency of the Uncore fabric.

# Usage

CPU frequency/power values are managed by assigning Cores to desired pools, associated with their attached Power
Profiles. The user of the Power Optimization Library can create any number of Exclusive Pools and Power Profiles.

C-States are similarly managed via Pools but can also be manage or per-CPU basis.

Uncore frequencies can be set system-wide, per package or per die.

### Setup

See [Object definitions](pkg/power/README.md#library-objects) for more information.

To begin using the library first create a host object with the supplied name, a reserved pool containing all CPUs marked
as system reserved and an empty list of Exclusive Pools. At this stage no changes are made to any configurations.

```go
import "github.com/intel/power-optimization-library/pkg/power"
host := power.CreateInstance("Name")
```

All CPUs start in a reserved pool, meaning that they cannot be managed, we need to first configure shared Pool that can
be managed. \
The below will leave CPUs with id 0,1 unmanaged by the library in the Reserved Pool and move all other CPUs to Shared
Pool.

````go
host.GetReservedPool().SetCpuIDs([]uint{0, 1})
````

Alternatively CPUs to be put in the Shared Pool can be provided

````go
host.GetSharedPool().SecCpuIDs([]uint{2, 3, 4, 5, 6,7})
````

Create an Exclusive pool with the name ``"performance-pool"``. No CPUs placed are in an exclusive pool upon creation.

````go
performancePool, err := node.AddExclusivePool("performance-pool")
````

Move desired CPUs to the new Pool

````go
err := performancePool.MoveCpuIDs([]uint{3, 4})
````

CPUs can only be moved to/from shared pool, cannot move pools from reserved pool or directly between exclusive pools

Exclusive pools can also be removed.

````go
err := perofmancePool.Remove()
````

All CPUs in the removed pool will be moved back to the Shared Pool.

### P-States

Power profiles can be associated with any Exclusive Pool or the Shared Pool

To set a power Profile firs create it using ``NewPowerProfile(name, minFreq, maxFreq, governor, epp)``
All frequency values are in kHz

````go
performanceProfile, err := NewPowerProfile("powerProfile", 2_600_000, 2_800_000, "performance", "performance")
````

All values and support by hardware is validated during Profile creation.

A power profile can now be associated with an Exclusive Pool or Shared Pool

````go
err := host.GetExclusivePool("performance-pool").SetPowerProfilePool(performanceProfile)
````

Power Profiles can be unset/removed by passing ``nil``. this will restore CPUs frequencies, governor and epp to default

````go
err := host.GetExclusivePool("performance-pool").SetPowerProfile(nil)
````

### C-States

C-States can be configured similarly by creating a CStates object and applying it to a pool

````go
err := host.GetExclusivePool("performance-pool").SetCstates(CStates{"C0": true})
````

It is also possible to set CStates on a per-CPU basis. This configuration will always precede per-Pool configuration

````go
err := host.GetAllCpus().ById(4).SetCstates(CStates{"C0": true})
````

Multiple CPUs

````go
cStates := Cstates{"C0": true}
for _, cpu := range host.GetAllCpus().ManyByIDs([]uint{3, 4, 5}){
    err := cpu.SetCStates(cStates)
}
````

### Uncore frequency

It is possible to set uncore frequency on a system-wide basis, per-package basis or per-die basis. Higher granularity 
objects i.e. per-die config will always precede per-package configuration

First create uncore object.
**Note:** due to driver limitations frequency will be rounded down to the nearest multiple of 100,000

````go
uncore, err := NewUncore(2_000_000, 2_500_000)
````

Uncore will be validated during creation against hardware capabilities

The uncore can now be applied system-wide, to package or die

````go
err := host.Topology().SetUncore(uncore)
err := host.Topology().Package(0).Die(0).SetUncore(uncore)
````

# References

- [Intel® Speed Select Technology - Core Power (Intel® SST-CP) Overview Technology Guide](https://networkbuilders.intel.com/solutionslibrary/intel-speed-select-technology-core-power-intel-sst-cp-overview-technology-guide)

# License

Apache 2.0 license, See [License](LICENSE)