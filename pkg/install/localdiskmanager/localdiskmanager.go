package localdiskmanager

import (
	"context"
	"reflect"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"github.com/hwameistor/hwameistor-operator/pkg/install"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalDiskManagerMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

// func NewLocalDiskManagerMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *LocalDiskManagerMaintainer {
// 	return &LocalDiskManagerMaintainer{
// 		Client: cli,
// 		ClusterInstance: clusterInstance,
// 	}
// }

func NewMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *LocalDiskManagerMaintainer {
	return &LocalDiskManagerMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var ldmDaemonSetLabelSelectorKey = "app"
var ldmDaemonSetLabelSelectorValue = "hwameistor-local-disk-manager"
var defaultKubeletRootDir = "/var/lib/kubelet"
var defaultLDMDaemonsetImageRegistry = "ghcr.m.daocloud.io"
var defaultLDMDaemonsetImageRepository = "hwameistor/local-disk-manager"
var defaultLDMDaemonsetImageTag = install.DefaultHwameistorVersion
var defaultLDMDaemonsetCSIRegistrarImageRegistry = "k8s-gcr.m.daocloud.io"
var defaultLDMDaemonsetCSIRegistrarImageRepository = "sig-storage/csi-node-driver-registrar"
var defaultLDMDaemonsetCSIRegistrarImageTag = "v2.5.0"
var managerContainerName = "manager"
var registrarContainerName = "registrar"

var ldmDaemonSet = appsv1.DaemonSet{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-local-disk-manager",
	},
	Spec: appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				ldmDaemonSetLabelSelectorKey: ldmDaemonSetLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					ldmDaemonSetLabelSelectorKey: ldmDaemonSetLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				HostNetwork: true,
				HostPID: true,
				Containers: []corev1.Container{
					{
						Name: managerContainerName,
						Command: []string{"/local-disk-manager"},
						Args: []string{
							"--endpoint=$(CSI_ENDPOINT)",
							"--nodeid=$(NODENAME)",
							"--v=5",
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: &corev1.SecurityContext{
							Privileged: &install.SecurityContextPrivilegedTrue,
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "udev",
								MountPath: "/run/udev",
							},
							{
								Name: "procmount",
								MountPath: "/host/proc",
								ReadOnly: true,
							},
							{
								Name: "devmount",
								MountPath: "/dev",
							},
							{
								Name: "host-etc-hwameistor",
								MountPath: "/etc/hwameistor",
								MountPropagation: &install.MountPropagationBidirectional,
							},
						},
						Env: []corev1.EnvVar{
							{
								Name: "NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
							{
								Name: "WATCH_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
							{
								Name: "POD_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.name",
									},
								},
							},
							{
								Name: "NODENAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "spec.nodeName",
									},
								},
							},
							{
								Name: "OPERATOR_NAME",
								Value: "local-disk-manager",
							},
						},
					},
					{
						Name: registrarContainerName,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args: []string{
							"--v=5",
							"--csi-address=/csi/csi.sock",
						},
						Lifecycle: &corev1.Lifecycle{
							PreStop: &corev1.LifecycleHandler{
								Exec: &corev1.ExecAction{
									Command: []string{
										"/bin/sh",
										"-c",
										"rm -rf /registration/disk.hwameistor.io  /registration/disk.hwameistor.io-reg.sock",
									},
								},
							},
						},
						Env: []corev1.EnvVar{
							{
								Name: "KUBE_NODE_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "spec.nodeName",
									},
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "socket-dir",
								MountPath: "/csi",
							},
							{
								Name: "registration-dir",
								MountPath: "/registration",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "udev",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/run/udev",
								Type: &install.HostPathDirectory,
							},
						},
					},
					{
						Name: "procmount",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/proc",
								Type: &install.HostPathDirectory,
							},
						},
					},
					{
						Name: "devmount",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/dev",
								Type: &install.HostPathDirectory,
							},
						},
					},
					{
						Name: "host-etc-hwameistor",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/etc/hwameistor",
								Type: &install.HostPathDirectoryOrCreate,
							},
						},
					},
				},
			},
		},
	},
}

func SetLDMDaemonSet(clusterInstance *hwameistoriov1alpha1.Cluster) *appsv1.DaemonSet{
	ldmDaemonSetToCreate := ldmDaemonSet.DeepCopy()

	ldmDaemonSetToCreate.OwnerReferences = append(ldmDaemonSetToCreate.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	ldmDaemonSetToCreate.Namespace = clusterInstance.Spec.TargetNamespace

	newClusterInstance := clusterInstance.DeepCopy()
	if newClusterInstance.Spec.LocalDiskManager == nil {
		newClusterInstance.Spec.LocalDiskManager = &hwameistoriov1alpha1.LocalDiskManagerSpec{}
	}

	if newClusterInstance.Spec.RBAC.ServiceAccountName == "" {
		ldmDaemonSetToCreate.Spec.Template.Spec.ServiceAccountName = "hwameistor-admin"
	} else {
		ldmDaemonSetToCreate.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	}

	// ldmDaemonSet.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	// setLDMDaemonSetVolumes(clusterInstance)
	// if newClusterInstance.Spec.LocalDiskManager.KubeletRootDir == "" {
	// 	newClusterInstance.Spec.LocalDiskManager.KubeletRootDir = defaultKubeletRootDir
	// }
	ldmDaemonSetToCreate = setLDMDaemonSetVolumes(newClusterInstance, ldmDaemonSetToCreate)
	// setLDMDaemonSetContainers(clusterInstance)
	ldmDaemonSetToCreate = setLDMDaemonSetContainers(newClusterInstance, ldmDaemonSetToCreate)

	if newClusterInstance.Spec.LocalDiskManager.TolerationOnMaster {
		ldmDaemonSetToCreate.Spec.Template.Spec.Tolerations = []corev1.Toleration{
			{
				Key: "CriticalAddonsOnly",
				Operator: corev1.TolerationOpExists,
			},
			{
				Effect: corev1.TaintEffectNoSchedule,
				Key: "node.kubernetes.io/not-ready",
				Operator: corev1.TolerationOpExists,
			},
			{
				Effect: corev1.TaintEffectNoSchedule,
				Key: "node-role.kubernetes.io/master",
				Operator: corev1.TolerationOpExists,
			},
			{
				Effect: corev1.TaintEffectNoSchedule,
				Key: "node-role.kubernetes.io/control-plane",
				Operator: corev1.TolerationOpExists,
			},
			{
				Effect: corev1.TaintEffectNoSchedule,
				Key: "node.cloudprovider.kubernetes.io/uninitialized",
				Operator: corev1.TolerationOpExists,
			},
		}
	}

	return ldmDaemonSetToCreate
}

func setLDMDaemonSetVolumes(clusterInstance *hwameistoriov1alpha1.Cluster, ldmDaemonSetToCreate *appsv1.DaemonSet) *appsv1.DaemonSet {
	sockeDirVolume := corev1.Volume{
		Name: "socket-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins/disk.hwameistor.io",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	ldmDaemonSetToCreate.Spec.Template.Spec.Volumes = append(ldmDaemonSetToCreate.Spec.Template.Spec.Volumes, sockeDirVolume)
	registrationDirVolume := corev1.Volume{
		Name: "registration-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins_registry/",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	ldmDaemonSetToCreate.Spec.Template.Spec.Volumes = append(ldmDaemonSetToCreate.Spec.Template.Spec.Volumes, registrationDirVolume)
	pluginDirVolume := corev1.Volume{
		Name: "plugin-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	ldmDaemonSetToCreate.Spec.Template.Spec.Volumes = append(ldmDaemonSetToCreate.Spec.Template.Spec.Volumes, pluginDirVolume)
	podsMountDir := corev1.Volume{
		Name: "pods-mount-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/pods",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	ldmDaemonSetToCreate.Spec.Template.Spec.Volumes = append(ldmDaemonSetToCreate.Spec.Template.Spec.Volumes, podsMountDir)

	return ldmDaemonSetToCreate
}

func setLDMDaemonSetContainers(clusterInstance *hwameistoriov1alpha1.Cluster, ldmDaemonSetToCreate *appsv1.DaemonSet) *appsv1.DaemonSet {
	// if clusterInstance.Spec.LocalDiskManager.Manager == nil {
	// 	clusterInstance.Spec.LocalDiskManager.Manager = &hwameistoriov1alpha1.ContainerCommonSpec{}
	// }
	// if clusterInstance.Spec.LocalDiskManager.CSI == nil {
	// 	clusterInstance.Spec.LocalDiskManager.CSI = &hwameistoriov1alpha1.CSISpec{}
	// }

	for i, container := range ldmDaemonSetToCreate.Spec.Template.Spec.Containers {
		if container.Name == managerContainerName {
			if resources := clusterInstance.Spec.LocalDiskManager.Manager.Resources; resources != nil {
				container.Resources = *resources
			}
			// if clusterInstance.Spec.LocalDiskManager.Manager.Image == nil {
			// 	clusterInstance.Spec.LocalDiskManager.Manager.Image = &hwameistoriov1alpha1.ImageSpec{}
			// }
			// if imageSpec.Registry == "" {
			// 	imageSpec.Registry = defaultLDMDaemonsetImageRegistry
			// }
			// if imageSpec.Repository == "" {
			// 	imageSpec.Repository = defaultLDMDaemonsetImageRepository
			// }
			// if imageSpec.Tag == "" {
			// 	imageSpec.Tag = defaultLDMDaemonsetImageTag
			// }
			container.Image = getLDMContainerManagerImageStringFromClusterInstance(clusterInstance)
			// container.Args = append(container.Args, "--csi-enable=" + strconv.FormatBool(clusterInstance.Spec.LocalDiskManager.CSI.Enable))
			container.Args = append(container.Args, "--csi-enable=true" )
			registrationDirVolumeMount := corev1.VolumeMount{
				Name: "registration-dir",
				MountPath: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins_registry",
			}
			container.VolumeMounts = append(container.VolumeMounts, registrationDirVolumeMount)
			pluginDirVolumeMount := corev1.VolumeMount{
				Name: "plugin-dir",
				MountPath: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins",
				MountPropagation: &install.MountPropagationBidirectional,
			}
			container.VolumeMounts = append(container.VolumeMounts, pluginDirVolumeMount)
			podsMountDirVolumeMount := corev1.VolumeMount{
				Name: "pods-mount-dir",
				MountPath: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/pods",
				MountPropagation: &install.MountPropagationBidirectional,
			}
			container.VolumeMounts = append(container.VolumeMounts, podsMountDirVolumeMount)
			container.Env = append(container.Env, corev1.EnvVar{
				Name: "CSI_ENDPOINT",
				Value: "unix:/" + clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins/disk.hwameistor.io/csi.sock",
			})
			ldmDaemonSetToCreate.Spec.Template.Spec.Containers[i] = container
		}

		if container.Name == registrarContainerName {
			if resources := clusterInstance.Spec.LocalDiskManager.CSI.Registrar.Resources; resources != nil {
				container.Resources = *resources
			}
			container.Image = getLDMContainerRegistrarImageStringFromClusterInstance(clusterInstance)
			container.Args = append(container.Args, "--kubelet-registration-path=" + clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins/disk.hwameistor.io/csi.sock")
			ldmDaemonSetToCreate.Spec.Template.Spec.Containers[i] = container
		}
	}

	return ldmDaemonSetToCreate
}

func getLDMContainerManagerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalDiskManager.Manager.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getLDMContainerRegistrarImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalDiskManager.CSI.Registrar.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func needOrNotToUpdateLDMDaemonset(cluster *hwameistoriov1alpha1.Cluster, gotten appsv1.DaemonSet) (bool, *appsv1.DaemonSet) {
	ds := gotten.DeepCopy()
	var needToUpdate bool

	for i, container := range ds.Spec.Template.Spec.Containers {
		if container.Name == managerContainerName {
			wantedImage := getLDMContainerManagerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				ds.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
		if container.Name == registrarContainerName {
			wantedImage := getLDMContainerRegistrarImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				ds.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	return needToUpdate, ds
}

func (m *LocalDiskManagerMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	ldmDaemonSetToCreate := SetLDMDaemonSet(newClusterInstance)
	key := types.NamespacedName{
		Namespace: ldmDaemonSetToCreate.Namespace,
		Name: ldmDaemonSetToCreate.Name,
	}
	var gottenDS appsv1.DaemonSet
	if err := m.Client.Get(context.TODO(), key, &gottenDS); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), ldmDaemonSetToCreate); errCreate != nil {
				log.Errorf("Create LocalDiskManager DaemonSet err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get LocalDiskManager DaemonSet err: %v", err)
			return newClusterInstance, err
		}
	}

	need, dsToUpdate := needOrNotToUpdateLDMDaemonset(newClusterInstance, gottenDS)
	if need {
		log.Infof("need to update ldm ds")
		if err := m.Client.Update(context.TODO(), dsToUpdate); err != nil {
			log.Errorf("Update LocalDiskManager DaemonSet err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: ldmDaemonSet.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		// if metav1.IsControlledBy(&pod, &ldmDaemonSet) {
		// 	podsManaged = append(podsManaged, pod)
		// }

		if pod.Labels[ldmDaemonSetLabelSelectorKey] == ldmDaemonSetLabelSelectorValue {
			podsManaged = append(podsManaged, pod)
		}
	}

	podsStatus := make([]hwameistoriov1alpha1.PodStatus, 0)
	for _, pod := range podsManaged {
		podStatus := hwameistoriov1alpha1.PodStatus{
			Name: pod.Name,
			Node: pod.Spec.NodeName,
			Status: string(pod.Status.Phase),
		}
		podsStatus = append(podsStatus, podStatus)
	}

	instancesDeployStatus := hwameistoriov1alpha1.DeployStatus{
		Pods: podsStatus,
		DesiredPodCount: gottenDS.Status.DesiredNumberScheduled,
		AvailablePodCount: gottenDS.Status.NumberAvailable,
		WorkloadType: "DaemonSet",
		WorkloadName: gottenDS.Name,
	}

	if newClusterInstance.Status.ComponentStatus.LocalDiskManager == nil {
		newClusterInstance.Status.ComponentStatus.LocalDiskManager = &hwameistoriov1alpha1.LocalDiskManagerStatus{
			Instances: &instancesDeployStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.LocalDiskManager.Instances == nil {
			newClusterInstance.Status.ComponentStatus.LocalDiskManager.Instances = &instancesDeployStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.LocalDiskManager.Instances, instancesDeployStatus) {
				newClusterInstance.Status.ComponentStatus.LocalDiskManager.Instances = &instancesDeployStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func CheckLDMReallyReady(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) bool {
	key := types.NamespacedName{
		Namespace: clusterInstance.Spec.TargetNamespace,
		Name: ldmDaemonSet.Name,
	}
	var gottenDS appsv1.DaemonSet
	if err := cli.Get(context.TODO(), key, &gottenDS); err != nil {
		log.Errorf("get localdiskmanager daemonset err: %v", err)
		return false
	}

	desiredPodsCount := gottenDS.Status.DesiredNumberScheduled
	availablePodsCount := gottenDS.Status.NumberAvailable
	if desiredPodsCount == 0 {
		log.Errorf("desiredPodsCount of localdiskmanager is zero, desiredPodsCount: %v, availablePodsCount: %v", desiredPodsCount, availablePodsCount)
		return false
	}

	if desiredPodsCount != availablePodsCount {
		log.Errorf("desiredPodsCount and availablePodsCount not equal, desiredPodsCount: %v, availablePodsCount: %v", desiredPodsCount, availablePodsCount)
		return false
	}

	var podList corev1.PodList
	if err := cli.List(context.TODO(), &podList, &client.ListOptions{Namespace: key.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return false
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[ldmDaemonSetLabelSelectorKey] == ldmDaemonSetLabelSelectorValue {
			podsManaged = append(podsManaged, pod)
		}
	}

	if len(podsManaged) != int(desiredPodsCount) {
		log.Errorf("localdiskmanager pods count not the same as desired, podsCount: %v, desired: %v", len(podsManaged), desiredPodsCount)
		return false
	}

	for _, pod := range podsManaged {
		if pod.Status.Phase != corev1.PodRunning {
			log.Errorf("podPhase is not running, pod: %+v", pod)
			return false
		}
	}

	return true
}

func FulfillLDMDaemonsetSpec (clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.LocalDiskManager == nil {
		clusterInstance.Spec.LocalDiskManager = &hwameistoriov1alpha1.LocalDiskManagerSpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.KubeletRootDir == "" {
		clusterInstance.Spec.LocalDiskManager.KubeletRootDir = defaultKubeletRootDir
	}
	if clusterInstance.Spec.LocalDiskManager.Manager == nil {
		clusterInstance.Spec.LocalDiskManager.Manager = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.Manager.Image == nil {
		clusterInstance.Spec.LocalDiskManager.Manager.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.Manager.Image.Registry == "" {
		clusterInstance.Spec.LocalDiskManager.Manager.Image.Registry = defaultLDMDaemonsetImageRegistry
	}
	if clusterInstance.Spec.LocalDiskManager.Manager.Image.Repository == "" {
		clusterInstance.Spec.LocalDiskManager.Manager.Image.Repository = defaultLDMDaemonsetImageRepository
	}
	if clusterInstance.Spec.LocalDiskManager.Manager.Image.Tag == "" {
		clusterInstance.Spec.LocalDiskManager.Manager.Image.Tag = defaultLDMDaemonsetImageTag
	}
	if clusterInstance.Spec.LocalDiskManager.CSI == nil {
		clusterInstance.Spec.LocalDiskManager.CSI = &hwameistoriov1alpha1.CSISpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Registrar == nil {
		clusterInstance.Spec.LocalDiskManager.CSI.Registrar = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Registrar.Image == nil {
		clusterInstance.Spec.LocalDiskManager.CSI.Registrar.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Registrar.Image.Registry == "" {
		clusterInstance.Spec.LocalDiskManager.CSI.Registrar.Image.Registry = defaultLDMDaemonsetCSIRegistrarImageRegistry
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Registrar.Image.Repository == "" {
		clusterInstance.Spec.LocalDiskManager.CSI.Registrar.Image.Repository = defaultLDMDaemonsetCSIRegistrarImageRepository
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Registrar.Image.Tag == "" {
		clusterInstance.Spec.LocalDiskManager.CSI.Registrar.Image.Tag = defaultLDMDaemonsetCSIRegistrarImageTag
	}


	return clusterInstance
}