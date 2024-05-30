# Under the hood

## Library objects

``Host`` - top level object representing the physical machine that is being managed

``Pool`` - object container for CPUs and ``Profile``, if CPU is in a Pool it means that the profile of that pool is
applied to it (except for when associated Profile is ``nil`` in which case the CPU config is set to its default values)

* ``Reserved Pool`` - CPUs that are in reserved pool and will never be touched by the library. By definition no profile
  can be configured for that pool. This pool cannot be removed or created

* ``Shared pool`` - CPUs that are not system reserved but don't belong to any Exclusive Pool. This pool can (but doesn't
  have to) have a profile associated. It cannot be removed or created

``Topology`` - an overarching object representing entire system topology. Calls on core object are system-wide eg.
``topology.SetUncore()`` would apply to all dies on the system

* ``Package`` - a physical Processor package as inserted into a socket, housing CPU Die(s)

* ``Die`` - a piece of integrated circuit, contains any number of cores

* ``Core`` a physical CPU core

* ``CPU`` - an object representing a logical CPU/compute unit/thread as seen by the operating system. If hyperthreading
  is enabled there are 2 ``CPUs`` per ``Core``, otherwise there is a 1-1 correspondence.

``Profile`` or ``Power Profile`` - stores desired P-State properties (Governor, EPP, Max/Min frequency) to be set for
Cores in the pool. A ``Power Profile`` has to be associated with ``Pool`` in order to be applied.

``C-States`` a Map storing association of C-State to its enablement state. ``Cstate`` can be associated with a ``Pool``
or with a ``CPU``. ``CPU`` associations will precede ``Pool`` associations.

``Uncore`` - Object storing desired uncore max and min frequency, can be applied to ``Topology`` for system-wide,
``Package``, or ``Die``. ``Die`` uncore will precede ``Package`` Uncore, which will in turn precede ``Topology``
system-wide uncore.

## Objects description

## Host

````
    Name                  string
    ExclusivePools        []Pool
    SharedPool            Pool
    PowerProfiles         Profiles
````

The Name value is simply the name of the Host.

The Exclusive Pools value holds the information for each of the Power Profiles that are available on the cluster. The
options available for a Power Profile are Performance, Balance Performance, and Balance Power. A Pool is added to this
list when the associated Power Profile is created on the host. This operation is undertaken by the Power Profile
controller, which will add the Pool when it detects the creation or change of a Power Profile in the cluster, and will
delete the Pool when a Power Profile is deleted.

Each Exclusive Pool will hold the Cores associated with the associated Power Workload. When a Guaranteed Pod is created
and the Cores are added to the correct Power Workload, the Power Workload controller will move the Core objects for that
Pod from the Shared Pool into the correct Pool in this list. When a Core is added to a Pool, the maximum and minimum
frequency values for the Core are changed in the object, and on the actual Node.

The Shared Pool holds all the Cores that are in the Kubernetes’ shared pool. When the Power Library instance is created,
this Shared Pool is populated automatically, taking in all the Cores on the Node, getting their absolute maximum and
minimum frequencies, and creates the Shared Pool’s Core list. The IsReservedSystemCPU value will be explained in the
Pool section. Initially - without the presence of a Shared Power Workload - every Core belongs to the Default Pool, or
the Pool that does not have any Power Profile associated with it and does not tune the Cores’ frequencies. When a Shared
Workload is created, the Cores that are specified to be part the ReservedSystemCPUs subset are not tuned, while
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

The Cores value is the list of Cores that are associated with that Pool. If it is an Exclusive Pool, Cores will be taken
out of the Shared Pool when a Guaranteed Pod is created and its Cores are placed in the associated Power Workload. The
operation of moving the Cores from the Shared Pool to the Exclusive Pool is done in the Power Workload controller when a
Power Workload is created, updated, or deleted.

The Shared Pool, while a singular Pool object, technically consists of two separate pools of Cores. The first pool is
the Default pool, where no frequency tuning takes place on the Cores. The second is the Shared pool, which is associated
with the Cores on the Node that will be tuned down to the lower frequencies of the Shared Power Profile. The Default
pool is the initial pool created for the Shared Pool when the Power Library instance is created. Every Core on the
system is a part of the Default pool at the beginning, with the MaximumFrequency value and the MinimumFrequency value of
the Core object being set to the absolute maximum and absolute minimum values of the Core on the system respectively.
The IsReservedSystemCPU is set to True for each Core in the Default pool.

When a Shared Power Workload is created by the user, the reservedCPUs flag in the Workload spec is used to determine
which Cores are to be left alone and which are to be tuned to the Shared Power Profile’s lower frequency values. This is
done by changing the IsReservedSystemCPU value in the Core object to False if the Core is not part of the Power
Workload’s reservedCPUs list. When the Power Library runs through a Pool to change the frequencies of all Cores in its
Core list, it skips over any Cores that have a True IsSystemReservedCPU value.

The PowerProfile value is simply the name of the Power Profile that is associated with the Pool. It is only the string
value of the name and not the actual Power Profile, that can be retrieved through the Node’s PowerProfiles list.

## Profile

````
    Name     string
    Max      int
    Min      int
    Governor string
    Epp      string
````

The Profile object is a replica of the Power Profile CRD. It’s just a way that the Power
Library can get the information about a Power Profile without having to constantly query the Kubernetes API.
The NewEcorePowerProfile constructor has 2 extra frequency fields called ``emin`` and ``emax`` and generates a profile
that will set different frequencies based on the core type. This is intended to be used on systems that have performance and efficiency cores.

## CPU

````
    ID                  int
    MinimumFreq         int
    MaximumFreq         int
    IsReservedSystemCPU bool
````

The ID value is simply the Core’s ID on the system.

The MaximumFrequency value is the frequency you want placed in the Core’s
/sys/devices/system/cpu/cpuN/cpufreq/scaling_max_freq file, which determines the maximum frequency the Core can run at.
Initially, when the Power Library is initialized and each Core object is placed into the Shared Pool’s Core list, this
value will take on the number in the Core’s cpuinfo_max_freq, which is the absolute maximum frequency the Core can run
at. This value is taken, as when the Core’s scaling values are not changed, this is the value that will be in the
scaling_max_freq file.

The MinimumFrequency value is the frequency you want placed in the Core’s
/sys/devices/system/cpu/cpuN/cpufreq/scaling_min_freq file, which determines the minimum frequency the Core can run at.
Initially, when the Power Library is initialized and each Core object is placed into the Shared Pool’s Core list, this
value will take on the number in the Core’s cpuinfo_min_freq, which is the absolute minimum frequency the Core can run
at. This value is taken, as when the Core’s scaling values are not changed, this is the value that will be in the
scaling_min_freq file.

The MaximumFrequency and MinimumFrequency values are updated when a Core is placed into a new Pool. For example, when a
Core goes from the Shared Pool to an Exclusive Pool, the values will be changed from - for example - 1500/1000 to
3500/3300. Then when the Cores are returned to the Shared Pool, they will revert from 3500/3300 to 1500/1000.

The IsReservedSystemCPU is the value which is used to determine whether the Core’s frequency values should be changed on
the system. If the value is True, when the Power Library is updated the frequency values on the Node, the Core will be
passed over and no changes will occur. The reason for this is to determine which Cores have been delegated as the
Reserved System CPUs for Kubelet, which we don’t want to update the frequency to as those cores will always be doing
work for Kubernetes. If there is no Shared Power Workload and a Core is taken out of the Shared Pool and given to an
Exclusive Pool, when the Core is given back to the Shared Pool, it’s scaling frequencies will still be updated to the
absolute maximum and minimum. There can never be an instance where a Core is taken out of the Shared Pool before a
Shared Power Workload is created, and then returned after the Shared Power Workload is created, accidentally setting the
Core’s maximum and minimum frequencies to the absolute values instead of those of the Shared Power Profile, as an
Exclusive Pool will never be given a core from the system that is a part of the Reserved System CPU list. So when
returned to the Shared Pool, if there is a Shared Power Workload available, it will take on the values in that, if not
it is given the absolute values.

# Features

## C-States

To save energy on a system, you can command the CPU to go into a low-power mode. Each CPU has several power modes, which
are collectively called C-States. These work by cutting the clock signal and power from idle CPUs, or CPUs that are not
executing commands.While you save more energy by sending CPUs into deeper C-State modes, it does take more time for the
CPU to fully “wake up” from sleep mode, so there is a tradeoff when it comes to deciding the depth of sleep.

### C-State Implementation in the Power Optimization Library

The driver that is used for C-States is the intel_idle driver. Everything associated with C-States in Linux is stored in
the /sys/devices/system/cpu/cpuN/cpuidle file or the /sys/devices/system/cpu/cpuidle file. To check the driver in use,
the user simply has to check the /sys/devices/system/cpu/cpuidle/current_driver file.

C-States have to be confirmed if they are actually active on the system. If a user requests any C-States, they need to
check on the system if they are activated and if they are not, reject the PowerConfig. The C-States are found in
/sys/devices/system/cpu/cpuN/cpuidle/stateN/.

### C-State Ranges

````
C0      Operating State
C1      Halt
C1E     Enhanced Halt
C2      Stop Grant   
C2E     Extended Stop Grant
C3      Deep Sleep
C4      Deeper Sleep
C4E/C5  Enhanced Deeper Sleep
C6      Deep Power Down
````

## Scaling Driver

### P-state
  The P-state governor feature allows the user to check if the P-state driver is enabled on the system. If the P-state
  driver is enabled while using the Kubernetes Power Manager, users may select a P-state governor per core, which are
  described as "performance" and "powersave" governors in the Power Profiles.

* Performance governor - The CPUfreq governor "performance" sets the CPU statically to the highest frequency within the
  borders of scaling_min_freq and scaling_max_freq.
* Powersave governor - The CPUfreq governor "powersave" sets the CPU statically to the lowest frequency within the
  borders of scaling_min_freq and scaling_max_freq.

### acpi-cpufreq
  The acpi-cpufreq driver setting operates much like the P-state driver but has a different set of available governors. For more information see [here](https://www.kernel.org/doc/html/v4.12/admin-guide/pm/cpufreq.html).
  One thing to note is that acpi-cpufreq reports the base clock as the frequency hardware limits however the P-state driver uses turbo frequency limits. 
  Both drivers can make use of turbo frequency; however, acpi-cpufreq can exceed hardware frequency limits when using turbo frequency. This is important to take into account when setting frequencies for profiles.
## Topology

Topology discovery is done via reading /sys/devices/system/cpuN/topology/{physical_package_id,die_id,core_id}. Based on
values there, the power library creates objects representing CPU package, die and core, and associates it with the
corresponding cores. this mapping is tree-like where associations are as important as id when identifying objects. E.g.
die 0 on package 0 is a different object to die 0 in package
one, ``topology().Package(0).Die(0) != topology().Package(1).Die(0)``

## Uncore

The power library provides an abstraction to manage Uncore frequency configuration. The driver allows setting
frequencies on die by die basis, and the library additionally allows setting frequency system-wide and package-wide with
different higher granularity objects taking precedence. \
The frequency setting is done via interacting with a kernel interface exposed by intel_uncore_frequency in
/sys/devices/system/cpu/intel_uncore_frequency/package_0N_die_0N/.

### References

* [Intel Uncore Frequency Scaling](https://www.kernel.org/doc/html/next/admin-guide/pm/intel_uncore_frequency_scaling.html)