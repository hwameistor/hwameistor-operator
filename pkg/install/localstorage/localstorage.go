package localstorage

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *LocalStorageMaintainer {
	return &LocalStorageMaintainer{
		Client:          cli,
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
var memberContainerName = "member"
var registrarContainerName = "registrar"

var juicesyncEnvName = "MIGRAGE_JUICESYNC_IMAGE"

var lsDaemonSetTemplate = appsv1.DaemonSet{
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
				Annotations: map[string]string{
					"kubectl.kubernetes.io/default-container": memberContainerName,
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
											Key:      "lvm.hwameistor.io/enable",
											Operator: "NotIn",
											Values:   []string{"false"},
										},
									},
								},
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: registrarContainerName,
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
										FieldPath:  "spec.nodeName",
									},
								},
							},
						},
						ImagePullPolicy: "IfNotPresent",
						Lifecycle: &corev1.Lifecycle{
							PreStop: &corev1.LifecycleHandler{
								Exec: &corev1.ExecAction{
									Command: []string{
										"/bin/sh",
										"-c",
										"rm -rf /registration/lvm.hwameistor.io /registration/lvm.hwameistor.io-reg.sock",
									},
								},
							},
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "socket-dir",
								MountPath: "/csi",
							},
							{
								Name:      "registration-dir",
								MountPath: "/registration",
							},
						},
					},
					{
						Name: memberContainerName,
						Args: []string{
							"--nodename=$(MY_NODENAME)",
							"--namespace=$(POD_NAMESPACE)",
							"--csi-address=$(CSI_ENDPOINT)",
							"--http-port=80",
							"--v=5",
							//Maximum number of concurrent volume migrations
							"--max-migrate-count=1",
							//Whether to enable data verification during migration
							"--migrate-check=false",
							//The default snapshot recovery time is ten minutes
							"--snapshot-restore-timeout=600",
						},
						Env: []corev1.EnvVar{
							{
								Name: "POD_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath:  "metadata.name",
									},
								},
							},
							{
								Name: "MY_NODENAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath:  "spec.nodeName",
									},
								},
							},
							{
								Name: "POD_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath:  "metadata.namespace",
									},
								},
							},
							{
								Name:  "NODE_ANNOTATION_KEY_STORAGE_IPV4",
								Value: "localstorage.hwameistor.io/storage-ipv4",
							},
						},
						ImagePullPolicy: "IfNotPresent",
						Ports: []corev1.ContainerPort{
							{
								Name:          "healthz",
								Protocol:      "TCP",
								ContainerPort: 80,
							},
						},
						ReadinessProbe: &corev1.Probe{
							FailureThreshold: 5,
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.IntOrString{
										Type:   intstr.String,
										StrVal: "healthz",
									},
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 10,
							PeriodSeconds:       2,
							SuccessThreshold:    1,
							TimeoutSeconds:      3,
						},
						SecurityContext: &corev1.SecurityContext{
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{
									"SYS_ADMIN",
								},
							},
							Privileged: &install.SecurityContextPrivilegedTrue,
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "host-dev",
								MountPath: "/dev",
							},
							{
								Name:             "host-etc-drbd",
								MountPath:        "/etc/drbd.d",
								MountPropagation: &install.MountPropagationBidirectional,
							},
							{
								Name:             "ssh-dir",
								MountPath:        "/root/.ssh",
								MountPropagation: &install.MountPropagationBidirectional,
							},
							{
								Name:             "host-mnt",
								MountPath:        "/mnt",
								MountPropagation: &install.MountPropagationBidirectional,
							},
						},
					},
				},
				DNSPolicy:                     corev1.DNSClusterFirstWithHostNet,
				HostPID:                       true,
				RestartPolicy:                 corev1.RestartPolicyAlways,
				SchedulerName:                 "default-scheduler",
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
					Type:   intstr.Int,
				},
			},
			Type: appsv1.RollingUpdateDaemonSetStrategyType,
		},
	},
}

func SetLSDaemonSet(clusterInstance *hwameistoriov1alpha1.Cluster) *appsv1.DaemonSet {
	lsDaemonSetToCreate := lsDaemonSetTemplate.DeepCopy()

	lsDaemonSetToCreate.OwnerReferences = append(lsDaemonSetToCreate.OwnerReferences, *metav1.NewControllerRef(clusterInstance, schema.FromAPIVersionAndKind("hwameistor.io/v1alpha1", "Cluster")))
	lsDaemonSetToCreate.Namespace = clusterInstance.Spec.TargetNamespace
	// lsDaemonSet.Spec.Template.Spec.PriorityClassName = clusterInstance.Spec.LocalStorage.Common.PriorityClassName
	lsDaemonSetToCreate.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	lsDaemonSetToCreate = setLSDaemonSetVolumes(clusterInstance, lsDaemonSetToCreate)
	lsDaemonSetToCreate = setLSDaemonSetContainers(clusterInstance, lsDaemonSetToCreate)

	if clusterInstance.Spec.LocalStorage.TolerationOnMaster {
		lsDaemonSetToCreate.Spec.Template.Spec.Tolerations = []corev1.Toleration{
			{
				Key:      "CriticalAddonsOnly",
				Operator: corev1.TolerationOpExists,
			},
			{
				Effect:   corev1.TaintEffectNoSchedule,
				Key:      "node.kubernetes.io/not-ready",
				Operator: corev1.TolerationOpExists,
			},
			{
				Effect:   corev1.TaintEffectNoSchedule,
				Key:      "node-role.kubernetes.io/master",
				Operator: corev1.TolerationOpExists,
			},
			{
				Effect:   corev1.TaintEffectNoSchedule,
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: corev1.TolerationOpExists,
			},
			{
				Effect:   corev1.TaintEffectNoSchedule,
				Key:      "node.cloudprovider.kubernetes.io/uninitialized",
				Operator: corev1.TolerationOpExists,
			},
		}
	}

	return lsDaemonSetToCreate
}

func setLSDaemonSetVolumes(clusterInstance *hwameistoriov1alpha1.Cluster, lsDaemonSetToCreate *appsv1.DaemonSet) *appsv1.DaemonSet {
	sockerDirVolume := corev1.Volume{
		Name: "socket-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins/lvm.hwameistor.io",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	lsDaemonSetToCreate.Spec.Template.Spec.Volumes = append(lsDaemonSetToCreate.Spec.Template.Spec.Volumes, sockerDirVolume)
	pluginDirVolume := corev1.Volume{
		Name: "plugin-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins",
				Type: &install.HostPathDirectory,
			},
		},
	}
	lsDaemonSetToCreate.Spec.Template.Spec.Volumes = append(lsDaemonSetToCreate.Spec.Template.Spec.Volumes, pluginDirVolume)
	registrationDirVolume := corev1.Volume{
		Name: "registration-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins_registry/",
				Type: &install.HostPathDirectory,
			},
		},
	}
	lsDaemonSetToCreate.Spec.Template.Spec.Volumes = append(lsDaemonSetToCreate.Spec.Template.Spec.Volumes, registrationDirVolume)
	podsMountDirVolume := corev1.Volume{
		Name: "pods-mount-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/pods",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	lsDaemonSetToCreate.Spec.Template.Spec.Volumes = append(lsDaemonSetToCreate.Spec.Template.Spec.Volumes, podsMountDirVolume)
	hostetcDRBDVolume := corev1.Volume{
		Name: "host-etc-drbd",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.Member.HostPathDRBDDir,
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	lsDaemonSetToCreate.Spec.Template.Spec.Volumes = append(lsDaemonSetToCreate.Spec.Template.Spec.Volumes, hostetcDRBDVolume)
	hostetcSSHVolume := corev1.Volume{
		Name: "ssh-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.Member.HostPathSSHDir,
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	lsDaemonSetToCreate.Spec.Template.Spec.Volumes = append(lsDaemonSetToCreate.Spec.Template.Spec.Volumes, hostetcSSHVolume)

	return lsDaemonSetToCreate
}

func setLSDaemonSetContainers(clusterInstance *hwameistoriov1alpha1.Cluster, lsDaemonSetToCreate *appsv1.DaemonSet) *appsv1.DaemonSet {
	for i, container := range lsDaemonSetToCreate.Spec.Template.Spec.Containers {
		if container.Name == registrarContainerName {
			container.Args = append(container.Args, "--kubelet-registration-path="+clusterInstance.Spec.LocalStorage.KubeletRootDir+"/plugins/lvm.hwameistor.io/csi.sock")
			container.Image = getLSContainerRegistrarImageStringFromClusterInstance(clusterInstance)
			// container.Resources = *clusterInstance.Spec.LocalStorage.CSI.Registrar.Resources
			if resources := clusterInstance.Spec.LocalStorage.CSI.Registrar.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		if container.Name == memberContainerName {
			if resources := clusterInstance.Spec.LocalStorage.Member.Resources; resources != nil {
				container.Resources = *resources
			}
			// if clusterInstance.Spec.LocalStorage.Member.DRBDStartPort != 0 {
			// 	container.Args = append(container.Args, "--drbd-start-port=" + fmt.Sprintf("%v", clusterInstance.Spec.LocalStorage.Member.DRBDStartPort))
			// }
			// if clusterInstance.Spec.LocalStorage.Member.MaxHAVolumeCount != 0 {
			// 	container.Args = append(container.Args, "--max-ha-volume-count=" + fmt.Sprintf("%v", clusterInstance.Spec.LocalStorage.Member.MaxHAVolumeCount))
			// }
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "CSI_ENDPOINT",
				Value: "unix:/" + clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins/lvm.hwameistor.io/csi.sock",
			})

			container.Env = append(container.Env, corev1.EnvVar{
				Name:  juicesyncEnvName,
				Value: getJuicesyncEnvFromClusterInstance(clusterInstance),
			})
			container.Image = getLSContainerMemberImageStringFromClusterInstance(clusterInstance)
			// container.Resources = *clusterInstance.Spec.LocalStorage.Member.Resources
			pluginDirVolumeMount := corev1.VolumeMount{
				Name:             "plugin-dir",
				MountPath:        clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins",
				MountPropagation: &install.MountPropagationBidirectional,
			}
			container.VolumeMounts = append(container.VolumeMounts, pluginDirVolumeMount)
			registrationDirVolumeMount := corev1.VolumeMount{
				Name:      "registration-dir",
				MountPath: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins_registry",
			}
			container.VolumeMounts = append(container.VolumeMounts, registrationDirVolumeMount)
			podsMountDirVolumeMount := corev1.VolumeMount{
				Name:             "pods-mount-dir",
				MountPath:        clusterInstance.Spec.LocalStorage.KubeletRootDir + "/pods",
				MountPropagation: &install.MountPropagationBidirectional,
			}
			container.VolumeMounts = append(container.VolumeMounts, podsMountDirVolumeMount)
		}
		lsDaemonSetToCreate.Spec.Template.Spec.Containers[i] = container
	}

	return lsDaemonSetToCreate
}

func getLSContainerMemberImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalStorage.Member.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getLSContainerRegistrarImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalStorage.CSI.Registrar.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getJuicesyncEnvFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	juicesyncImage := clusterInstance.Spec.LocalStorage.Member.JuicesyncImage
	return juicesyncImage.Registry + "/" + juicesyncImage.Repository + ":" + juicesyncImage.Tag
}

func needOrNotToUpdateLSDaemonset(cluster *hwameistoriov1alpha1.Cluster, gotten appsv1.DaemonSet) (bool, *appsv1.DaemonSet) {
	ds := gotten.DeepCopy()
	var needToUpdate bool

	for i, container := range ds.Spec.Template.Spec.Containers {
		if container.Name == memberContainerName {
			var containerModified bool

			wantedJuicesyncEnv := getJuicesyncEnvFromClusterInstance(cluster)
			juicesyncEnvNotFound := true
			for i, env := range container.Env {
				if env.Name == juicesyncEnvName {
					juicesyncEnvNotFound = false
					if env.Value != wantedJuicesyncEnv {
						env.Value = wantedJuicesyncEnv
						container.Env[i] = env
						containerModified = true
					}
				}
			}
			if juicesyncEnvNotFound {
				container.Env = append(container.Env, corev1.EnvVar{
					Name:  juicesyncEnvName,
					Value: wantedJuicesyncEnv,
				})
				containerModified = true
			}

			wantedImage := getLSContainerMemberImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				containerModified = true
			}

			if containerModified {
				ds.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}

		}
		if container.Name == registrarContainerName {
			wantedImage := getLSContainerRegistrarImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				ds.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	return needToUpdate, ds
}

func (m *LocalStorageMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	lsDaemonSetToCreate := SetLSDaemonSet(newClusterInstance)
	key := types.NamespacedName{
		Namespace: lsDaemonSetToCreate.Namespace,
		Name:      lsDaemonSetToCreate.Name,
	}
	var gottenDS appsv1.DaemonSet
	if err := m.Client.Get(context.TODO(), key, &gottenDS); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), lsDaemonSetToCreate); errCreate != nil {
				log.Errorf("Create LocalStorage DaemonSet err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get LocalStorage DaemonSet err: %v", err)
			return newClusterInstance, err
		}
	}

	needToUpdate, dsToUpdate := needOrNotToUpdateLSDaemonset(newClusterInstance, gottenDS)
	if needToUpdate {
		log.Infof("need to update ls daemonset")
		if err := m.Client.Update(context.TODO(), dsToUpdate); err != nil {
			log.Errorf("Update ls daemonset err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: lsDaemonSetToCreate.Namespace}); err != nil {
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
			Name:   pod.Name,
			Node:   pod.Spec.NodeName,
			Status: string(pod.Status.Phase),
		}
		podsStatus = append(podsStatus, podStatus)
	}

	instancesDeployStatus := hwameistoriov1alpha1.DeployStatus{
		Pods:              podsStatus,
		DesiredPodCount:   gottenDS.Status.DesiredNumberScheduled,
		AvailablePodCount: gottenDS.Status.NumberAvailable,
		WorkloadType:      "DaemonSet",
		WorkloadName:      gottenDS.Name,
	}

	if newClusterInstance.Status.ComponentStatus.LocalStorage == nil {
		newClusterInstance.Status.ComponentStatus.LocalStorage = &hwameistoriov1alpha1.LocalStorageStatus{
			Instances: &instancesDeployStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.LocalStorage.Instances == nil {
			newClusterInstance.Status.ComponentStatus.LocalStorage.Instances = &instancesDeployStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.LocalStorage.Instances, instancesDeployStatus) {
				newClusterInstance.Status.ComponentStatus.LocalStorage.Instances = &instancesDeployStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func (m *LocalStorageMaintainer) Uninstall() error {
	key := types.NamespacedName{
		Namespace: m.ClusterInstance.Spec.TargetNamespace,
		Name:      lsDaemonSetTemplate.Name,
	}
	var gottenDS appsv1.DaemonSet
	if err := m.Client.Get(context.TODO(), key, &gottenDS); err != nil {
		if errors.IsNotFound(err) {
			return nil
		} else {
			log.Errorf("Get LocalStorage DaemonSet err: %v", err)
			return err
		}
	} else {
		for _, reference := range gottenDS.OwnerReferences {
			if reference.Name == m.ClusterInstance.Name {
				if err = m.Client.Delete(context.TODO(), &gottenDS); err != nil {
					return err
				} else {
					return nil
				}
			}
		}
	}

	log.Errorf("LocalStorage Owner is not %s, can't delete", m.ClusterInstance.Name)
	return nil
}

func FulfillLSDaemonsetSpec(clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
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
