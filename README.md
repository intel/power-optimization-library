# libraries.orchestrators.resourcemanagement.power.powerlibrary

The Power Library is intended to monitor the systems CPUs

# Overview

Power Library is an engine for the [Kubernetes Power Manager](https://github.com/intel/kubernetes-power-manager)
replacing App QoS. It takes the desired configuration for the cores associated with exclusive Pods and tune them based
on the requested Power Profile. It will also facilitate the use of the SST Technology Suite (SST-BF, SST-CP, and SST-TF)
. (in progress)

# Definitions

Node - top level object representing a physical machine, stores pools information

Pool - container for cores and profile, if core is in a pool it means that the profile of that pool is applied to a
core (except for when profile in ``nil`` in which case core is set to its default values)

Default pool - cores that are system reserved and should never be touched by the library are in this pool. by definition
no profile can be configured for that pool. This pool cannot be deleted or created

Shared pool - cores that are not system reserved but don't belong to any exclusive pool. This pool can (but doesn't have
to) have a profile attached

Profile - stores desired properties to be set for cores in the pool

# Usage

Core values are managed by moving cores to the pools, with their attached power profiles. user of the library can create
any number of exclusive pools and profiles attached to them

## Example

```go
import "powerlib"
node := powerlib.CreateInstance(nodeName)
```

This will create a node object with the supplied name, default pool containing all cores marked as system reserved with
no changes are made to the cores' configuration, and an empty list of exclusive pools. at this point no changes are made
to cores

````go
node.AddSharedPool([]reservedCpuIds, powerlib.NewProfile(name, minFreq, maxFreq, epp))
````

Create the shared pool, this call takes all cores that we want to keep as reserved. All cores that are **not** passed to
the function call are moved to the shared pool. ``NewProfile()`` can be replaced with ``nil`` to keep the default values

````go
node.AddExclusivePool(name, profile)
````

Create a new exclusive pool, with supplied profile. No cores are added to the pool. profile can be ``nil`` to keep
default values

````go
node.AddCoresToExclusivePool(name, []coreIds)
````

This will move cores with specified ids for the shared pool to the exclusive pool. power profile of the exclusive pool
will be applied to the core

````
node.RemoveCoresFromExclusivePool(name, []coreIds)
````

Move cores with specified ids form the Exclusive cores back to the shared pool and apply profile of the shared pool

````go
node.RemoveExclusivePool(name)
````

Remove exclusive core and its attached power profile. anny cores in the pool are moved back to the shared pool

# Objects

## Node

````
    Name                  string
    ExclusivePools        []Pool
    SharedPool            Pool
    PowerProfiles         Profiles
````

The Name value is simply the name of the node, allowing a Pod that is scheduled on a different Node to be skipped easily
and to differentiate between Nodes more easily.

The Exclusive Pools value holds the information for each of the Power Profiles that are available on the cluster. The
options available for a Power Profile are Performance, Balance Performance, and Balance Power. A Pool is added to this
list when the associated Power Profile is created in the cluster. This operation is undertaken by the Power Profile
controller, which will add the Pool when it detects the creation or change of a Power Profile in the cluster, and will
delete the Pool when a Power Profile is deleted.

Each Exclusive Pool will hold the cores associated with the associated Power Workload. When a Guaranteed Pod is created
and the cores are added to the correct Power Workload, the Power Workload controller will move the Core objects for that
Pod from the Shared Pool into the correct Pool in this list. When a Core is added to a Pool, the maximum and minimum
frequency values for the core are changed in the object, and on the actual Node.

The Shared Pool holds all the cores that are in the Kubernetes’ shared pool. When the Power Library instance is created,
this Shared Pool is populated automatically, taking in all the cores on the Node, getting their absolute maximum and
minimum frequencies, and creates the Shared Pool’s Core list. The IsReservedSystemCPU value will be explained in the
Pool section. Initially - without the presence of a Shared Power Workload - every core belongs to the Default Pool, or
the Pool that does not have any Power Profile associated with it and does not tune the cores’ frequencies. When a Shared
Workload is created, the Cores that are specified to be part the the ReservedSystemCPUs subset are not tuned, while
every other Core in the list is. When a Core for a Guaranteed Pod comes along, it is taken from the Shared Pool, and
when the Pod is terminated, it is placed back in the Shared Pool.

## Pool

````
    Name            string
    Cores           []Core
    PowerProfile    Profile
````

The Name value is simply the name of the Pool, which will either be performance, balance-performance, balance-power, or
shared.

The Cores value is the list of cores that are associated with that Pool. If it is an Exclusive Pool, cores will be taken
out of the Shared Pool when a Guaranteed Pod is created and its cores are placed in the associated Power Workload. The
operation of moving the cores from the Shared Pool to the Exclusive Pool is done in the Power Workload controller when a
Power Workload is created, updated, or deleted.

The Shared Pool, while a singular Pool object, technically consists of two separate pools of cores. The first pool is
the Default pool, where no frequency tuning takes place on the cores. The second is the Shared pool, which is associated
with the cores on the Node that will be tuned down to the lower frequencies of the Shared Power Profile. The Default
pool is the initial pool created for the Shared Pool when the Power Library instance is created. Every core on the
system is a part of the Default pool at the beginning, with the MaximumFrequency value and the MinimumFrequency value of
the Core object being set to the absolute maximum and absolute minimum values of the core on the system respectively.
The IsReservedSystemCPU is set to True for each Core in the Default pool.

When a Shared Power Workload is created by the user, the reservedCPUs flag in the Workload spec is used to determine
which cores are to be left alone and which are to be tuned to the Shared Power Profile’s lower frequency values. This is
done by changing the IsReservedSystemCPU value in the Core object to False if the core is not part of the Power
Workload’s reservedCPUs list. When the Power Library runs through a Pool to change the frequencies of all Cores in its
Core list, it skips over any Cores that have a True IsSystemReservedCPU value.

The PowerProfile value is simply the name of the Power Profile that is associated with the Pool. It is only the string
value of the name and not the actual Power Profile, that can be retrieved through the Node’s PowerProfiles list.

## Profile

````
    Name    string
    Max     int
    Min     int
    Epp     string
````

The Profile object is a replica of the Power Profile CRD used by the Power Manager. It’s just a way that the Power
Library can get the information about a Power Profile without having to constantly query the Kubernetes API.

## Core

````
    ID                  int
    MinimumFreq         int
    MaximumFreq         int
    IsReservedSystemCPU bool
````

The ID value is simply the core’s ID on the system.

The MaximumFrequency value is the frequency you want placed in the core’s
/sys/devices/system/cpu/cpuN/cpufreq/scaling_max_freq file, which determines the maximum frequency the core can run at.
Initially, when the Power Library is initialized and each Core object is placed into the Shared Pool’s Core list, this
value will take on the number in the core’s cpuinfo_max_freq, which is the absolute maximum frequency the core can run
at. This value is taken, as when the core’s scaling values are not changed, this is the value that will be in the
scaling_max_freq file.

The MinimumFrequency value is the frequency you want placed in the core’s
/sys/devices/system/cpu/cpuN/cpufreq/scaling_min_freq file, which determines the minimum frequency the core can run at.
Initially, when the Power Library is initialized and each Core object is placed into the Shared Pool’s Core list, this
value will take on the number in the core’s cpuinfo_min_freq, which is the absolute minimum frequency the core can run
at. This value is taken, as when the core’s scaling values are not changed, this is the value that will be in the
scaling_min_freq file.

The MaximumFrequency and MinimumFrequency values are updated when a Core is placed into a new Pool. For example, when a
Core goes from the Shared Pool to an Exclusive Pool, the values will be changed from - for example - 1500/1000 to
3500/3300. Then when the Cores are returned to the Shared Pool, they will revert from 3500/3300 to 1500/1000.

The IsReservedSystemCPU is the value which is used to determine whether the core’s frequency values should be changed on
the system. If the value is True, when the Power Library is updated the frequency values on the Node, the Core will be
passed over and no changes will occur. The reason for this is to determine which cores have been delegated as the
Reserved System CPUs for Kubelet, which we don’t want to update the frequency to as those cores will always be doing
work for Kubernetes. If there is no Shared Power Workload and a Core is taken out of the Shared Pool and given to an
Exclusive Pool, when the Core is given back to the Shared Pool, it’s scaling frequencies will still be updated to the
absolute maximum and minimum. There can never be an instance where a Core is taken out of the Shared Pool before a
Shared Power Workload is created, and then returned after the Shared Power Workload is created, accidentally setting the
Core’s maximum and minimum frequencies to the absolute values instead of those of the Shared Power Profile, as an
Exclusive Pool will never be given a core from the system that is a part of the Reserved System CPU list. So when
returned to the Shared Pool, if there is a Shared Power Workload available, it will take on the values in that, if not
it is given the absolute values.
