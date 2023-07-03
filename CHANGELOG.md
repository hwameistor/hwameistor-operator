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

