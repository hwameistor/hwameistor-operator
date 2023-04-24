## Introduction of hwameistor-operator

Hwameistor-operator will be used for HwameiStor components management and installation automation.

### Versioning

Versions of Hwameistor are listed below:

|  Components   | Hwameistor  |
|---------------|-------------|
|  Versions     | 0.8.x `[1]` | 

NOTES:

[1] `.x` means all the patch releases of Hwameistor can be naturally supported in one operator version.

### Life Cycle Management (LCM) for HwameiStor components:

    Apiserver
    LocalStorage
    LocalDiskManager
    Scheduler
    AdmissionController
    VolumeEvictor
    Exporter

### Local disk claim automation for ensuring HwameiStor ready

### Admission control configuration management for HwameiStor volume verification


## HwameiStor-operator installation with Hwameistor cluster setup

$ helm repo add hwameistor https://hwameistor.io/hwameistor-operator

$ helm repo update hwameistor

$ helm install hwameistor-operator hwameistor/hwameistor-operator

## Roadmap

TODO
