## Introduction of hwameistor-operator

Hwameistor-operator will be used for HwameiStor components management and installation automation.

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


## HwameiStor-operator installation

$ helm repo add hwameistor-operator https://hwameistor.io/hwameistor-operator

$ helm repo update hwameistor-operator

$ helm install hwameistor-operator hwameistor-operator/hwameistor-operator


## HwameiStor installation with hwameistor-operator

$ kubectl apply -f https://raw.githubusercontent.com/hwameistor/hwameistor-operator/main/config/samples/hwameistor.io_hmcluster.yaml

## Roadmap

TODO