# libraries.orchestrators.resourcemanagement.power.powerlibrary
The Power Library is intended to monitor the systems CPUs

It checks: 
* the number of cores on a system
* the min and max frequency
* it gets the core IDs
* Sets the scaling frequencies
* Restores the CPUs to initial state if required

Functions include:
* Get number of CPUs on system
* Get the CPU max frequency
* Get the CPU min frequency
* Get the CPU scaling max frequencey
* Get the CPU scaling min frequencey
* Get the base frequency
* Get the scaling driver used in each CPU
* Set the core's Scailing max frequency
* Set the core's Scailing min frequency
* Restore CPUs to initial state
* Get the online status of the core
* Set the core's governor

