v0.16.4 / 2025-6-20
========================
* use hwameistor v0.16.4 #359 (@FloatXD )
* detech CRD exist or not before install servicemonitor #358 (@SSmallMonster )
* Increase pr test coverage #357 (@FloatXD )
* add hwameistor servicemonitor to charts #354 (@SSmallMonster )
* [Test]fix permission #352 (@FloatXD )
* upgrade modules.txt #350 (@FloatXD )

v0.16.3 / 2025-4-22
========================
* upgrade hwameistor to v0.16.3 #346 (@FloatXD )

v0.16.2 / 2025-2-25
========================
* add args in local-storage ds #331 （peng9808）
* fix member args snapshot-restore-timeout #332（peng9808）
* Add parameter description #333（peng9808）
* use hwameistor v0.16.2 #335（peng9808）

v0.16.1 / 2024-11-26
========================
* use hwameistor v0.16.1 #327（peng9808）
* update drbd v0.4.3 #326（peng9808）

v0.16.0 / 2024-10-28
========================
* use hwameistor v0.16.0 #319（peng9808）
* add readme to helm #318 (peng9808)
* Fixed the issue that UI cannot be automatically uninstalled #316 (peng9808)

v0.15.0 / 2024-9-23
========================
* use hwameistor v0.15.0 #313（peng9808）
* Fixed minor version not being recognized correctly #310 (peng9808)
* use rockylinux to baseImage,Fixed drbd minor version not being recognized correctly #306 (peng9808)
* add trivy，gosec scan #304 (peng9808)
* correct default container annotation #303 (peng9808)

v0.14.9 / 2024-8-29
========================

* Optimize the Drbd installation process #299(peng9808)
* use hwameistor v0.14.7 #300 (peng9808)

v0.14.8 / 2024-7-31
========================

* fix DatasetDefaultPoolClass bug #285 (@peng9808)
* Inject dataload-init into the Admission module,Fixed the issue that changing the loop variable does not take effect #287 (@peng9808)
* Added automatic uninstallation function #288 (@peng9808)
* make data safer by default.in case user delete pvc by accident , and limit RBAC hwameistor-role permissions #289 (@peng9808)
* fix(InstallCRDs): reverse CRD install status when necessary #292 (@SSmallMonster)
* fix(admission): update when secrets found but not found data #293 (@SSmallMonster)
* update base image #294 (@peng9808)
* Fix the bug of reinstallation #296 (@peng9808)
* use hwameistor v0.14.6 #295 (@peng9808)

v0.14.7 / 2024-6-25
========================

* Need to implement the installation of the two modules dataload-manager and dataset-manager #278 (@peng9808)
* fix reinstalling RBAC bug #276 (@peng9808)
* use hwameistor v0.14.5 #273 (@peng9808)

v0.14.6 / 2024-4-30
========================

* use hwameistor v0.14.3 #273 (@peng9808)

v0.14.5 / 2024-4-7
========================

* use hwameistor v0.14.2, ha v0.4.1, ui v0.16.0 #267 (@peng9808)

v0.14.4 / 2024-3-13
========================

* Solve the bug of drbd install

v0.14.3 / 2024-3-12
========================

* Solve the bug of charts-syncer #262 (@peng9808)

v0.14.2 / 2024-3-11
========================

* Optimize drbd installation #257 (@peng9808 )

v0.14.1 / 2023-1-29
========================

* list pvc_autoresizer pods with labelselector #246 (@hellokg21 )
* disable evictor #247 (@buffalo1024 )
* update drbd-adapter #248 (@peng9808 )
* use hwameistor v0.14.1 #249 (@buffalo1024 )

v0.14.0 / 2023-12-29
========================

* support set evictor.disable in day 2 #240 (@hellokg21 )
* Ensure ldmcsicontroller when deleted #241 (@hellokg21 )
* ensure lscsicontroller when deleted #242 (@hellokg21 )

v0.13.4 / 2023-12-1
========================

* bump hwameistor-ui version to v0.14.1 #237 (@buffalo1024 )

v0.13.3 / 2023-11-27
========================

* use hwameistor v0.13.1 #234 (@buffalo1024 )

v0.13.2 / 2023-11-16
========================

* add option to disable containers about snapshot #222 (@buffalo1024 )
* fix CVE-2021-43527 #223 (@buffalo1024 )
* reuse kubeconfig in installing CRDs #225 (@hellokg21 )
* set namespace when listing ldm pods #226 (@hellokg21 )
* fulfill snapshot spec in localstorage spec #228 (@buffalo1024 )
* comment fulfilling cluster spec #230 (@buffalo1024 )
* update juicesync env for local-storage #231 (@buffalo1024 )

v0.13.1 / 2023-10-20
========================

* update scheduler configmap when source yaml file updated #219 (@buffalo1024 )

v0.13.0 / 2023-10-18
========================

* add new components in .relok8s-images.yaml #194 (@buffalo1024 )
* reuse kubeconfig #201 (@hellokg21 )
* reuse kubeconfig in setting up LDN informer #202 (@hellokg21 )
* do not print error when storageclass already exists #203 (@hellokg21 )
* upper default logger level(debug) for ldm #204 (@SSmallMonster )
* support set resources of component while installing #206 (@buffalo1024 )
* update csi-provisioner image tag of localstorage to v3.5.0 #207 (@buffalo1024 )
* add failurePolicy of admission controller in helm chart #208 (@buffalo1024 )
* add resources value in values.extra.prod.yaml for new components #209 (@buffalo1024 )
* use hwameistor v0.12.4 #210 (@buffalo1024 )
* add tool juicesync #211 (@buffalo1024 )
* use hwameistor v0.13.0 #212 (@buffalo1024 )

v0.12.2 / 2023-9-19
========================

* modify localdiskmanager #190(@hellokg21)
* use hwameistor v0.12.3 and add localdiskactioncontroller #191(@buffalo1024)

v0.12.1 / 2023-9-5
========================

* fix read crds files err #181(@buffalo1024)
* add new volume for localdiskmanager #182(@buffalo1024)
* add snapshot-controller and snapshotter containers #183(@buffalo1024)
* config two hostpath volumes of localstorage #184(@buffalo1024)
* modify scheduler-config.yaml #185(@buffalo1024)

v0.12.0 / 2023-8-29
========================

* update hwameistor version to v0.12.1 #175(@buffalo1024)
* add auditor,failover-assistant,pvc-autoresizer #176(@buffalo1024)
* udpate helm prehook #177(@buffalo1024)
* update .relok8s-images.yaml #178(@buffalo1024)

v0.10.8 / 2023-8-23
========================

* optimize handling of localstorage #165(@hellokg21)
* set localstorage rclone env while cluster changed #166(@hellokg21)
* support option not to claim disk #167(@buffalo1024)
* fix wrong poolClass ins storageclass parameters #168(@buffalo1024)
* add option to disable component in helm chart #169(@buffalo1024)

v0.10.7 / 2023-7-14
========================

* add preHookJob to Update operator crds #159(@buffalo1024)
* support setting disk reserve configurations by helm values #160(@buffalo1024)
* use phase to represent phase of hwameistor cluster cr #161(@buffalo1024)

v0.10.6 / 2023-7-3
========================

* bump operator image tag

v0.10.5 / 2023-7-3
========================

* fix image of rclone in helm chart template #149(@Vacant2333)
* support modifying cluster cr to update components container image #151(@buffalo1024)
* remove hook annotations of cluster cr in helm chart templates #152(@buffalo1024)
* support modifying cluster cr to update components deployment replicas #153(@buffalo1024)
* support update hwameistor crds after installing first time #154(@buffalo1024)

v0.10.4 / 2023-6-8
========================

* add extra check to ensure localdiskmanager is really ready #142(@buffalo1024)
* add icon in Chart.yaml #143(@buffalo1024)

v0.10.3 / 2023-6-6
========================

* wait 2 minutes for localdiskmanager created localdisks #136(@buffalo1024)

v0.10.2 / 2023-5-26
========================

*  add authentication for hwameistor-apiserver and bump hwameistor version to v0.10.3 #128(@buffalo1024)

v0.10.1 / 2023-5-24
========================

* add NODENAME env for apiserver #120(@buffalo1024)
* fix parameter name in helm template #122(@Vacant2333)
* support disable ha while installing #124(@buffalo1024)

v0.9.3 / 2023-5-12
========================

* support specifying the namespace to install operator #98(@buffalo1024)
* support setting replicas of deployments by helm chart values #101(@buffalo1024)
* bump the version of hwameistor to install to v0.9.3 #102(@buffalo1024)

v0.0.1 / 2023-02-22
========================

# Operator
* [1] Descriptions about a feature(#<releated_pr>, @Author)
* [2] Descriptions about bug fixes(#<releated_pr>, @Author)
...

