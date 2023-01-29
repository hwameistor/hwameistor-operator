package lscsicontroller

import (
	"context"
	"errors"
	"reflect"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"github.com/hwameistor/hwameistor-operator/pkg/install"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LSCSIMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *LSCSIMaintainer {
	return &LSCSIMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var lsCSIControllerLabelSelectorKey = "app"
var lsCSIControllerLabelSelectorValue =  "hwameistor-local-storage-csi-controller"

var lsCSIController = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-local-storage-csi-controller",
	},
	Spec: appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				lsCSIControllerLabelSelectorKey: lsCSIControllerLabelSelectorValue,
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					lsCSIControllerLabelSelectorKey: lsCSIControllerLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key: "app",
											Operator: "In",
											Values: []string{"hwameistor-local-storage"},
										},
									},
								},
								TopologyKey: "topology.lvm.hwameistor.io/node",
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: "provisioner",
						Args: []string{
							"--v=5",
							"--csi-address=$(CSI_ADDRESS)",
							"--leader-election=true",
							"--feature-gates=Topology=true",
							"--strict-topology",
							"--extra-create-metadata=true",
						},
						Env: []corev1.EnvVar{
							{
								Name: "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						ImagePullPolicy: "IfNotPresent",
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "socket-dir",
								MountPath: "/csi",
							},
						},
					},
					{
						Name: "attacher",
						Args: []string{
							"--v=5",
							"--csi-address=$(CSI_ADDRESS)",
							"--leader-election=true",
							"--timeout=120s",
						},
						Env: []corev1.EnvVar{
							{
								Name: "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						ImagePullPolicy: "IfNotPresent",
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "socket-dir",
								MountPath: "/csi",
							},
						},
					},
					{
						Name: "resizer",
						Args: []string{
							"--v=5",
							"--csi-address=$(CSI_ADDRESS)",
							"--leader-election=true",
						},
						Env: []corev1.EnvVar{
							{
								Name: "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						ImagePullPolicy: "IfNotPresent",
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "socket-dir",
								MountPath: "/csi",
							},
						},
					},
				},
				DNSPolicy: corev1.DNSClusterFirst,
				RestartPolicy: corev1.RestartPolicyAlways,
				TerminationGracePeriodSeconds: &install.TerminationGracePeriodSeconds30s,
				Tolerations: []corev1.Toleration{
					{
						Key: "CriticalAddonsOnly",
						Operator: corev1.TolerationOpExists,
					},
					{
						Key: "node.kubernetes.io/not-ready",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key: "node-role.kubernetes.io/master",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key: "node-role.kubernetes.io/control-plane",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key: "node.cloudprovider.kubernetes.io/uninitialized",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
			},
		},
	},
}

func SetLSCSIController(clusterInstance *hwameistoriov1alpha1.Cluster) {
	lsCSIController.Namespace = clusterInstance.Spec.TargetNamespace
	lsCSIController.OwnerReferences = append(lsCSIController.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	replicas := clusterInstance.Spec.LocalStorage.CSI.Controller.Replicas
	lsCSIController.Spec.Replicas = &replicas
	lsCSIController.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	// lsCSIController.Spec.Template.Spec.PriorityClassName = clusterInstance.Spec.LocalStorage.Common.PriorityClassName
	socketDirVolume := corev1.Volume{
		Name: "socket-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins/lvm.hwameistor.io",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	lsCSIController.Spec.Template.Spec.Volumes = append(lsCSIController.Spec.Template.Spec.Volumes, socketDirVolume)
	setLSCSIControllerContainers(clusterInstance)
}

func setLSCSIControllerContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
	for i, container := range lsCSIController.Spec.Template.Spec.Containers {
		if container.Name == "provisioner" {
			imageSpec := clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
			// container.Resources = *clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Resources
		}
		if container.Name == "attacher" {
			imageSpec := clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
			// container.Resources = *clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Resources
		}
		if container.Name == "resizer" {
			imageSpec := clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
			// container.Resources = *clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Resources
		}
		lsCSIController.Spec.Template.Spec.Containers[i] = container
	}
}

func (m *LSCSIMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	SetLSCSIController(newClusterInstance)
	key := types.NamespacedName{
		Namespace: lsCSIController.Namespace,
		Name: lsCSIController.Name,
	}
	var gottenCSIController appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gottenCSIController); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &lsCSIController); errCreate != nil {
				log.Errorf("Create LS CSIController err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get LS CSIController err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: lsCSIController.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[lsCSIControllerLabelSelectorKey] == lsCSIControllerLabelSelectorValue {
			podsManaged = append(podsManaged, pod)
		}
	}

	if len(podsManaged) > int(gottenCSIController.Status.Replicas) {
		podsManagedErr := errors.New("pods managed more than desired")
		log.Errorf("err: %v", podsManagedErr)
		return newClusterInstance, podsManagedErr
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

	csiDeployStatus := hwameistoriov1alpha1.DeployStatus{
		Pods: podsStatus,
		DesiredPodCount: gottenCSIController.Status.Replicas,
		AvailablePodCount: gottenCSIController.Status.AvailableReplicas,
		WorkloadType: "Deployment",
		WorkloadName: gottenCSIController.Name,
	}

	if newClusterInstance.Status.LocalStorage == nil {
		newClusterInstance.Status.LocalStorage = &hwameistoriov1alpha1.LocalStorageStatus{
			CSI: &csiDeployStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.LocalStorage.CSI == nil {
			newClusterInstance.Status.LocalStorage.CSI = &csiDeployStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.LocalStorage.CSI, csiDeployStatus) {
				newClusterInstance.Status.LocalStorage.CSI = &csiDeployStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}