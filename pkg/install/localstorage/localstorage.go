package localstorage

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
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalStorageMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *LocalStorageMaintainer {
	return &LocalStorageMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var lsDaemonSetLabelSelectorKey = "app"
var lsDaemonSetLabelSelectorValue = "hwameistor-local-storage"
var defaultKubeletRootDir = "/var/lib/kubelet"
var defaultLSDaemonsetImageRegistry = "ghcr.m.daocloud.io"
var defaultLSDaemonsetImageRepository = "hwameistor/local-storage"
var defaultLSDaemonsetImageTag = install.DefaultHwameistorVersion
var defaultLSDaemonsetCSIRegistrarImageRegistry = "k8s-gcr.m.daocloud.io"
var defaultLSDaemonsetCSIRegistrarImageRepository = "sig-storage/csi-node-driver-registrar"
var defaultLSDaemonsetCSIRegistrarImageTag = "v2.5.0"
var defaultRCloneImageRepository = "rclone/rclone"
var defaultRCloneImageTag = "1.53.2"

var lsDaemonSet = appsv1.DaemonSet{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-local-storage",
	},
	Spec: appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				lsDaemonSetLabelSelectorKey: lsDaemonSetLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					lsDaemonSetLabelSelectorKey: lsDaemonSetLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key: "lvm.hwameistor.io/enable",
											Operator: "NotIn",
											Values: []string{"false"},
										},
									},
								},
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: "registrar",
						Args: []string{
							"--v=5",
							"--csi-address=/csi/csi.sock",
						},
						Env: []corev1.EnvVar{
							{
								Name: "KUBE_NODE_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath: "spec.nodeName",
									},
								},
							},
						},
						ImagePullPolicy: "IfNotPresent",
						Lifecycle: &corev1.Lifecycle{
							PreStop: &corev1.Handler{
								Exec: &corev1.ExecAction{
									Command: []string{
										"/bin/sh",
										"-c",
										"rm -rf /registration/lvm.hwameistor.io /registration/lvm.hwameistor.io-reg.sock",
									},
								},
							},
						},
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: "File",
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
					{
						Name: "member",
						Args: []string{
							"--nodename=$(MY_NODENAME)",
							"--namespace=$(POD_NAMESPACE)",
							"--csi-address=$(CSI_ENDPOINT)",
							"--http-port=80",
							"--debug=true",
						},
						Env: []corev1.EnvVar{
							{
								Name: "POD_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath: "metadata.name",
									},
								},
							},
							{
								Name: "MY_NODENAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath: "spec.nodeName",
									},
								},
							},
							{
								Name: "POD_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath: "metadata.namespace",
									},
								},
							},
							{
								Name: "NODE_ANNOTATION_KEY_STORAGE_IPV4",
								Value: "localstorage.hwameistor.io/storage-ipv4",
							},
						},
						ImagePullPolicy: "IfNotPresent",
						Ports: []corev1.ContainerPort{
							{
								Name: "healthz",
								Protocol: "TCP",
								ContainerPort: 80,
							},
						},
						ReadinessProbe: &corev1.Probe{
							FailureThreshold: 5,
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.IntOrString{
										Type: intstr.String,
										StrVal: "healthz",
									},
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 10,
							PeriodSeconds: 2,
							SuccessThreshold: 1,
							TimeoutSeconds: 3,
						},
						SecurityContext: &corev1.SecurityContext{
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{
									"SYS_ADMIN",
								},
							},
							Privileged: &install.SecurityContextPrivilegedTrue,
						},
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "host-dev",
								MountPath: "/dev",
							},
							{
								Name: "host-etc-drbd",
								MountPath: "/etc/drbd.d",
								MountPropagation: &install.MountPropagationBidirectional,
							},
							{
								Name: "ssh-dir",
								MountPath: "/root/.ssh",
								MountPropagation: &install.MountPropagationBidirectional,
							},
							{
								Name: "host-mnt",
								MountPath: "/mnt",
								MountPropagation: &install.MountPropagationBidirectional,
							},
						},
					},
				},
				DNSPolicy: corev1.DNSClusterFirstWithHostNet,
				HostPID: true,
				RestartPolicy: corev1.RestartPolicyAlways,
				SchedulerName: "default-scheduler",
				TerminationGracePeriodSeconds: &install.TerminationGracePeriodSeconds30s,
				Volumes: []corev1.Volume{
					{
						Name: "host-dev",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/dev",
								Type: &install.HostPathTypeUnset,
							},
						},
					},
					{
						Name: "host-etc-drbd",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/etc/drbd.d",
								Type: &install.HostPathDirectoryOrCreate,
							},
						},
					},
					{
						Name: "ssh-dir",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/root/.ssh",
								Type: &install.HostPathDirectoryOrCreate,
							},
						},
					},
					{
						Name: "host-mnt",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/mnt",
								Type: &install.HostPathDirectoryOrCreate,
							},
						},
					},
				},
			},
		},
		UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
			RollingUpdate: &appsv1.RollingUpdateDaemonSet{
				MaxUnavailable: &intstr.IntOrString{
					IntVal: 1,
					Type: intstr.Int,
				},
			},
			Type: appsv1.RollingUpdateDaemonSetStrategyType,
		},
	},
}

func SetLSDaemonSet(clusterInstance *hwameistoriov1alpha1.Cluster) {
	lsDaemonSet.OwnerReferences = append(lsDaemonSet.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	lsDaemonSet.Namespace = clusterInstance.Spec.TargetNamespace
	// lsDaemonSet.Spec.Template.Spec.PriorityClassName = clusterInstance.Spec.LocalStorage.Common.PriorityClassName
	lsDaemonSet.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	setLSDaemonSetVolumes(clusterInstance)
	setLSDaemonSetContainers(clusterInstance)

	if clusterInstance.Spec.LocalStorage.TolerationOnMaster {
		lsDaemonSet.Spec.Template.Spec.Tolerations = []corev1.Toleration{
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
}

func setLSDaemonSetVolumes(clusterInstance *hwameistoriov1alpha1.Cluster) {
	sockerDirVolume := corev1.Volume{
		Name: "socket-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins/lvm.hwameistor.io",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	lsDaemonSet.Spec.Template.Spec.Volumes = append(lsDaemonSet.Spec.Template.Spec.Volumes, sockerDirVolume)
	pluginDirVolume := corev1.Volume{
		Name: "plugin-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins",
				Type: &install.HostPathDirectory,
			},
		},
	}
	lsDaemonSet.Spec.Template.Spec.Volumes = append(lsDaemonSet.Spec.Template.Spec.Volumes, pluginDirVolume)
	registrationDirVolume := corev1.Volume{
		Name: "registration-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins_registry/",
				Type: &install.HostPathDirectory,
			},
		},
	}
	lsDaemonSet.Spec.Template.Spec.Volumes = append(lsDaemonSet.Spec.Template.Spec.Volumes, registrationDirVolume)
	podsMountDirVolume := corev1.Volume{
		Name: "pods-mount-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/pods",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	lsDaemonSet.Spec.Template.Spec.Volumes = append(lsDaemonSet.Spec.Template.Spec.Volumes, podsMountDirVolume)
}

func setLSDaemonSetContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
	for i, container := range lsDaemonSet.Spec.Template.Spec.Containers {
		if container.Name == "registrar" {
			container.Args = append(container.Args, "--kubelet-registration-path=" + clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins/lvm.hwameistor.io/csi.sock")
			imageSpec := clusterInstance.Spec.LocalStorage.CSI.Registrar.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
			// container.Resources = *clusterInstance.Spec.LocalStorage.CSI.Registrar.Resources
		}
		if container.Name == "member" {
			// if clusterInstance.Spec.LocalStorage.Member.DRBDStartPort != 0 {
			// 	container.Args = append(container.Args, "--drbd-start-port=" + fmt.Sprintf("%v", clusterInstance.Spec.LocalStorage.Member.DRBDStartPort))
			// }
			// if clusterInstance.Spec.LocalStorage.Member.MaxHAVolumeCount != 0 {
			// 	container.Args = append(container.Args, "--max-ha-volume-count=" + fmt.Sprintf("%v", clusterInstance.Spec.LocalStorage.Member.MaxHAVolumeCount))
			// }
			container.Env = append(container.Env, corev1.EnvVar{	
				Name: "CSI_ENDPOINT",
				Value: "unix:/" + clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins/lvm.hwameistor.io/csi.sock",
			})
			rcloneImageSpec := clusterInstance.Spec.LocalStorage.Member.RcloneImage
			container.Env = append(container.Env, corev1.EnvVar{
				Name: "MIGRAGE_RCLONE_IMAGE",
				// Value: rcloneImageSpec.Registry + "/" + rcloneImageSpec.Repository + ":" + rcloneImageSpec.Tag,
				Value: rcloneImageSpec.Repository + ":" + rcloneImageSpec.Tag,
			})
			imageSpec := clusterInstance.Spec.LocalStorage.Member.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" +imageSpec.Tag
			// container.Resources = *clusterInstance.Spec.LocalStorage.Member.Resources
			pluginDirVolumeMount := corev1.VolumeMount{
				Name: "plugin-dir",
				MountPath: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins",
				MountPropagation: &install.MountPropagationBidirectional,
			}
			container.VolumeMounts = append(container.VolumeMounts, pluginDirVolumeMount)
			registrationDirVolumeMount := corev1.VolumeMount{
				Name: "registration-dir",
				MountPath: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins_registry",
			}
			container.VolumeMounts = append(container.VolumeMounts, registrationDirVolumeMount)
			podsMountDirVolumeMount := corev1.VolumeMount{
				Name: "pods-mount-dir",
				MountPath: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/pods",
				MountPropagation: &install.MountPropagationBidirectional,
			}
			container.VolumeMounts = append(container.VolumeMounts, podsMountDirVolumeMount)
		}
		lsDaemonSet.Spec.Template.Spec.Containers[i] = container
	}
}

func (m *LocalStorageMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	SetLSDaemonSet(newClusterInstance)
	key := types.NamespacedName{
		Namespace: lsDaemonSet.Namespace,
		Name: lsDaemonSet.Name,
	}
	var gottenDS appsv1.DaemonSet
	if err := m.Client.Get(context.TODO(), key, &gottenDS); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &lsDaemonSet); errCreate != nil {
				log.Errorf("Create LocalStorage DaemonSet err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get LocalStorage DaemonSet err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: lsDaemonSet.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[lsDaemonSetLabelSelectorKey] == lsDaemonSetLabelSelectorValue {
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

	if newClusterInstance.Status.LocalStorage == nil {
		newClusterInstance.Status.LocalStorage = &hwameistoriov1alpha1.LocalStorageStatus{
			Instances: &instancesDeployStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.LocalStorage.Instances == nil {
			newClusterInstance.Status.LocalStorage.Instances = &instancesDeployStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.LocalStorage.Instances, instancesDeployStatus) {
				newClusterInstance.Status.LocalStorage.Instances = &instancesDeployStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func FulfillLSDaemonsetSpec (clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.LocalStorage == nil {
		clusterInstance.Spec.LocalStorage = &hwameistoriov1alpha1.LocalStorageSpec{}
	}
	if clusterInstance.Spec.LocalStorage.KubeletRootDir == "" {
		clusterInstance.Spec.LocalStorage.KubeletRootDir = defaultKubeletRootDir
	}
	if clusterInstance.Spec.LocalStorage.Member == nil {
		clusterInstance.Spec.LocalStorage.Member = &hwameistoriov1alpha1.MemberSpec{}
	}
	if clusterInstance.Spec.LocalStorage.Member.Image == nil {
		clusterInstance.Spec.LocalStorage.Member.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.LocalStorage.Member.Image.Registry == "" {
		clusterInstance.Spec.LocalStorage.Member.Image.Registry = defaultLSDaemonsetImageRegistry
	}
	if clusterInstance.Spec.LocalStorage.Member.Image.Repository == "" {
		clusterInstance.Spec.LocalStorage.Member.Image.Repository = defaultLSDaemonsetImageRepository
	}
	if clusterInstance.Spec.LocalStorage.Member.Image.Tag == "" {
		clusterInstance.Spec.LocalStorage.Member.Image.Tag = defaultLSDaemonsetImageTag
	}
	if clusterInstance.Spec.LocalStorage.Member.RcloneImage == nil {
		clusterInstance.Spec.LocalStorage.Member.RcloneImage = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.LocalStorage.Member.RcloneImage.Repository == "" {
		clusterInstance.Spec.LocalStorage.Member.RcloneImage.Repository = defaultRCloneImageRepository
	}
	if clusterInstance.Spec.LocalStorage.Member.RcloneImage.Tag == "" {
		clusterInstance.Spec.LocalStorage.Member.RcloneImage.Tag = defaultRCloneImageTag
	}
	if clusterInstance.Spec.LocalStorage.CSI == nil {
		clusterInstance.Spec.LocalStorage.CSI = &hwameistoriov1alpha1.CSISpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Registrar == nil {
		clusterInstance.Spec.LocalStorage.CSI.Registrar = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Registrar.Image == nil {
		clusterInstance.Spec.LocalStorage.CSI.Registrar.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Registrar.Image.Registry == "" {
		clusterInstance.Spec.LocalStorage.CSI.Registrar.Image.Registry = defaultLSDaemonsetCSIRegistrarImageRegistry
	}
	if clusterInstance.Spec.LocalStorage.CSI.Registrar.Image.Repository == "" {
		clusterInstance.Spec.LocalStorage.CSI.Registrar.Image.Repository = defaultLSDaemonsetCSIRegistrarImageRepository
	}
	if clusterInstance.Spec.LocalStorage.CSI.Registrar.Image.Tag == "" {
		clusterInstance.Spec.LocalStorage.CSI.Registrar.Image.Tag = defaultLSDaemonsetCSIRegistrarImageTag
	}

	return clusterInstance
}