# Hwameistor-operator

Hwameistor-operator will be used for [HwameiStor](https://github.com/hwameistor/hwameistor) components management and installation automation. 

![System architecture](https://raw.githubusercontent.com/hwameistor/hwameistor/main/docs/docs/img/architecture.png)

## Versioning

Versions of HwameiStor are listed below:

|  Components   | Hwameistor  |
|---------------|-------------|
|  Versions     | 0.8.x `[1]` | 

Notes:

[1] `.x` means all the patch releases of HwameiStor can be naturally supported in one operator version.

## Features

### Life Cycle Management (LCM) for HwameiStor components:

- Apiserver
- LocalStorage
- LocalDiskManager
- Scheduler
- AdmissionController
- VolumeEvictor
- Exporter


### Admission control configuration management for HwameiStor volume verification

### Local disk claim automation for ensuring HwameiStor ready
HwameiStor-operator will automate the installation and management of the HwameiStor system. 
It automatically discovers the disk types of the nodes and creates HwameiStor storage pools accordingly. 
It also automatically creates corresponding storage classes (SC) based on the configuration and functionality of the HwameiStor system. 
Additionally, it provides full lifecycle management (LCM) of the components within the HwameiStor system.

Notes: If there are no available clean disks, the operator will not automatically create a storage class. 
During the installation process, the operator will automatically manage the disks and add the available disks to the pool of local storage. 
If the available disks are provided after the installation, you will need to manually issue a local disk claim to add the disks to the local 
storage node. Once there are disks in the pool of the local storage node, the operator will automatically create a storage class. In other words, 
if there is no capacity available, the operator will not create a storage class automatically.

## Quick Start

You can follow our [Quick Start](https://hwameistor.io/docs/quick_start/install/operator) guide to quickly play with HwameiStor on your kubernetes cluster.

## Documentation

You can see our documentation at HwameiStor website for more in-depth installation and instructions for production:
- [English](https://hwameistor.io/)
- [简体中文](https://hwameistor.io/cn/)

### Blog

- [English](https://hwameistor.io/blog/)
- [简体中文](https://hwameistor.io/cn//blog)

### Slack

Welcome to join our [Slack channel](https://join.slack.com/t/hwameistor/shared_invite/zt-1dkabcq2c-KIRBJDBc_GgZZfeLrooK6g).

## License

Copyright (c) 2014-2023 The HwameiStor Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
<http://www.apache.org/licenses/LICENSE-2.0>.
