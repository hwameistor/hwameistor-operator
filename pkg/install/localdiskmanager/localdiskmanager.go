package localdiskmanager

import (
	"context"
	"reflect"
	"strconv"

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
						Name: "manager",
						Command: []string{"/local-disk-manager"},
						Args: []string{
							"--endpoint=$(CSI_ENDPOINT)",
							"--nodeid=$(NODENAME)",

						},
						ImagePullPolicy: "IfNotPresent",
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
				},
			},
		},
	},
}

func SetLDMDaemonSet(clusterInstance *hwameistoriov1alpha1.Cluster) {
	ldmDaemonSet.OwnerReferences = append(ldmDaemonSet.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	ldmDaemonSet.Namespace = clusterInstance.Spec.TargetNamespace
	ldmDaemonSet.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	setLDMDaemonSetVolumes(clusterInstance)
	setLDMDaemonSetContainers(clusterInstance)
}

func setLDMDaemonSetVolumes(clusterInstance *hwameistoriov1alpha1.Cluster) {
	sockeDirVolume := corev1.Volume{
		Name: "socket-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins/disk.hwameistor.io",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	ldmDaemonSet.Spec.Template.Spec.Volumes = append(ldmDaemonSet.Spec.Template.Spec.Volumes, sockeDirVolume)
	registrationDirVolume := corev1.Volume{
		Name: "registration-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins_registry/",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	ldmDaemonSet.Spec.Template.Spec.Volumes = append(ldmDaemonSet.Spec.Template.Spec.Volumes, registrationDirVolume)
	pluginDirVolume := corev1.Volume{
		Name: "plugin-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	ldmDaemonSet.Spec.Template.Spec.Volumes = append(ldmDaemonSet.Spec.Template.Spec.Volumes, pluginDirVolume)
	podsMountDir := corev1.Volume{
		Name: "pods-mount-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/pods",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	ldmDaemonSet.Spec.Template.Spec.Volumes = append(ldmDaemonSet.Spec.Template.Spec.Volumes, podsMountDir)
}

func setLDMDaemonSetContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
	for i, container := range ldmDaemonSet.Spec.Template.Spec.Containers {
		if container.Name == "manager" {
			if resources := clusterInstance.Spec.LocalDiskManager.Manager.Resources; resources != nil {
				container.Resources = *resources
			}
			imageSpec := clusterInstance.Spec.LocalDiskManager.Manager.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
			container.Args = append(container.Args, "--csi-enable=" + strconv.FormatBool(clusterInstance.Spec.LocalDiskManager.CSI.Enable))
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
			ldmDaemonSet.Spec.Template.Spec.Containers[i] = container
		}
	}

	if clusterInstance.Spec.LocalDiskManager.CSI.Enable {
		imageSpec := clusterInstance.Spec.LocalDiskManager.CSI.Registrar.Image
		container := corev1.Container{
			Name: "registrar",
			Image: imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag,
			ImagePullPolicy: "IfNotPresent",
			Args: []string{
				"--v=5",
				"--csi-address=/csi/csi.sock",
				"--kubelet-registration-path=" + clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins/disk.hwameistor.io/csi.sock",
			},
			Lifecycle: &corev1.Lifecycle{
				PreStop: &corev1.Handler{
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
		}
		ldmDaemonSet.Spec.Template.Spec.Containers = append(ldmDaemonSet.Spec.Template.Spec.Containers, container)
	}
}

func (m *LocalDiskManagerMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	SetLDMDaemonSet(newClusterInstance)
	key := types.NamespacedName{
		Namespace: ldmDaemonSet.Namespace,
		Name: ldmDaemonSet.Name,
	}
	var gottenDS appsv1.DaemonSet
	if err := m.Client.Get(context.TODO(), key, &gottenDS); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &ldmDaemonSet); errCreate != nil {
				log.Errorf("Create LocalDiskManager DaemonSet err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get LocalDiskManager DaemonSet err: %v", err)
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

	if newClusterInstance.Status.LocalDiskManager == nil {
		newClusterInstance.Status.LocalDiskManager = &hwameistoriov1alpha1.LocalDiskManagerStatus{
			Instances: &instancesDeployStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.LocalDiskManager.Instances == nil {
			newClusterInstance.Status.LocalDiskManager.Instances = &instancesDeployStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.LocalDiskManager.Instances, instancesDeployStatus) {
				newClusterInstance.Status.LocalDiskManager.Instances = &instancesDeployStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}