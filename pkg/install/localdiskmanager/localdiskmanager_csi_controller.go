package localdiskmanager

// import (
// 	"context"
// 	"errors"
// 	"reflect"

// 	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
// 	installhwamei "github.com/hwameistor/hwameistor-operator/installhwamei"
// 	log "github.com/sirupsen/logrus"
// 	appsv1 "k8s.io/api/apps/v1"
// 	corev1 "k8s.io/api/core/v1"
// 	apierrors "k8s.io/apimachinery/pkg/api/errors"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/types"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// )

// type LDMCSIMaintainer struct {
// 	Client client.Client
// 	ClusterInstance *hwameistoriov1alpha1.Cluster
// }

// func NewLDMCSIMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *LDMCSIMaintainer {
// 	return &LDMCSIMaintainer{
// 		Client: cli,
// 		ClusterInstance: clusterInstance,
// 	}
// }

// var ldmCSIControllerReplicas = int32(1)
// var ldmCSIControllerLabelSelectorKey = "app"
// var ldmCSIControllerLabelSelectorValue = "hwameistor-local-disk-csi-controller"

// var ldmCSIController = appsv1.Deployment{
// 	ObjectMeta: metav1.ObjectMeta{
// 		Name: "hwameistor-local-disk-csi-controller",
// 	},
// 	Spec: appsv1.DeploymentSpec{
// 		Replicas: &ldmCSIControllerReplicas,
// 		Selector: &metav1.LabelSelector{
// 			MatchLabels: map[string]string{
// 				ldmCSIControllerLabelSelectorKey: ldmCSIControllerLabelSelectorValue,
// 			},
// 		},
// 		Template: corev1.PodTemplateSpec{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Labels: map[string]string{
// 					ldmCSIControllerLabelSelectorKey: ldmCSIControllerLabelSelectorValue,
// 				},
// 			},
// 			Spec: corev1.PodSpec{
// 				Affinity: &corev1.Affinity{
// 					PodAffinity: &corev1.PodAffinity{
// 						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
// 							{
// 								LabelSelector: &metav1.LabelSelector{
// 									MatchExpressions: []metav1.LabelSelectorRequirement{
// 										{
// 											Key: "app",
// 											Operator: "In",
// 											Values: []string{"hwameistor-local-disk-manager"},
// 										},
// 									},
// 								},
// 								TopologyKey: "topology.disk.hwameistor.io/node",
// 							},
// 						},
// 					},
// 				},
// 				Containers: []corev1.Container{
// 					{
// 						Name: "provisioner",
// 						ImagePullPolicy: "IfNotPresent",
// 						Args: []string{
// 							"--v=5",
// 							"--csi-address=$(CSI_ADDRESS)",
// 							"--leader-election=true",
// 							"--feature-gates=Topology=true",
// 							"--strict-topology",
// 							"--extra-create-metadata=true",
// 						},
// 						Env: []corev1.EnvVar{
// 							{
// 								Name: "CSI_ADDRESS",
// 								Value: "/csi/csi.sock",
// 							},
// 						},
// 						VolumeMounts: []corev1.VolumeMount{
// 							{
// 								Name: "socket-dir",
// 								MountPath: "/csi",
// 							},
// 						},
// 					},
// 					{
// 						Name: "attacher",
// 						ImagePullPolicy: "IfNotPresent",
// 						Args: []string{
// 							"--v=5",
// 							"--csi-address=$(CSI_ADDRESS)",
// 							"--leader-election=true",
// 							"--timeout=120s",
// 						},
// 						Env: []corev1.EnvVar{
// 							{
// 								Name: "CSI_ADDRESS",
// 								Value: "/csi/csi.sock",
// 							},
// 						},
// 						VolumeMounts: []corev1.VolumeMount{
// 							{
// 								Name: "socket-dir",
// 								MountPath: "/csi",
// 							},
// 						},
// 					},
// 				},
// 			},
// 		},
// 	},
// }

// func SetLDMCSIController(clusterInstance *hwameistoriov1alpha1.Cluster) {
// 	ldmCSIController.Namespace = clusterInstance.Spec.TargetNamespace
// 	ldmCSIController.OwnerReferences = append(ldmCSIController.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
// 	// ldmCSIController.Spec.Template.Spec.PriorityClassName = clusterInstance.Spec.LocalDiskManager.CSI.Controller.Common.PriorityClassName
// 	ldmCSIController.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
// 	setLDMCSIControllerVolumes(clusterInstance)
// 	setLDMCSIControllerContainers(clusterInstance)
// }

// func setLDMCSIControllerVolumes(clusterInstance *hwameistoriov1alpha1.Cluster) {
// 	volume := corev1.Volume{
// 		Name: "socket-dir",
// 		VolumeSource: corev1.VolumeSource{
// 			HostPath: &corev1.HostPathVolumeSource{
// 				Type: &installhwamei.HostPathDirectoryOrCreate,
// 				Path: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins/disk.hwameistor.io",
// 			},
// 		},
// 	}

// 	ldmCSIController.Spec.Template.Spec.Volumes = append(ldmCSIController.Spec.Template.Spec.Volumes, volume)
// }

// func setLDMCSIControllerContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
// 	for i, container := range ldmCSIController.Spec.Template.Spec.Containers {
// 		if container.Name == "provisioner" {
// 			// container.Resources = *clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Resources
// 			imageSpec := clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Image
// 			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
// 		}
// 		if container.Name == "attacher" {
// 			// container.Resources = *clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Resources
// 			imageSpec := clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Image
// 			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
// 		}

// 		ldmCSIController.Spec.Template.Spec.Containers[i] = container
// 	}
// }

// func (m *LDMCSIMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
// 	newClusterInstance := m.ClusterInstance.DeepCopy()
// 	SetLDMCSIController(newClusterInstance)
// 	key := types.NamespacedName{
// 		Namespace: ldmCSIController.Namespace,
// 		Name: ldmCSIController.Name,
// 	}
// 	var gottenCSIController appsv1.Deployment
// 	if err := m.Client.Get(context.TODO(), key, &gottenCSIController); err != nil {
// 		if apierrors.IsNotFound(err) {
// 			if errCreate := m.Client.Create(context.TODO(), &ldmCSIController); errCreate != nil {
// 				log.Errorf("Create LDM CSIController err: %v", errCreate)
// 				return newClusterInstance, errCreate
// 			}
// 			return newClusterInstance, nil
// 		} else {
// 			log.Errorf("Get LDM CSIController err: %v", err)
// 			return newClusterInstance, err
// 		}
// 	}

// 	var podList corev1.PodList
// 	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: ldmCSIController.Namespace}); err != nil {
// 		log.Errorf("List pods err: %v", err)
// 		return newClusterInstance, err
// 	}

// 	var podsManaged []corev1.Pod
// 	for _, pod := range podList.Items {
// 		if pod.Labels[ldmCSIControllerLabelSelectorKey] == ldmCSIControllerLabelSelectorValue {
// 			podsManaged = append(podsManaged, pod)
// 		}
// 	}

// 	if len(podsManaged) > int(gottenCSIController.Status.Replicas) {
// 		podsManagedErr := errors.New("pods managed more than desired")
// 		log.Errorf("err: %v", podsManagedErr)
// 		return newClusterInstance, podsManagedErr
// 	}

// 	podsStatus := make([]hwameistoriov1alpha1.PodStatus, 0)
// 	for _, pod := range podsManaged {
// 		podStatus := hwameistoriov1alpha1.PodStatus{
// 			Name: pod.Name,
// 			Node: pod.Spec.NodeName,
// 			Status: string(pod.Status.Phase),
// 		}
// 		podsStatus = append(podsStatus, podStatus)
// 	}

// 	csiDeployStatus := hwameistoriov1alpha1.DeployStatus{
// 		Pods: podsStatus,
// 		DesiredPodCount: gottenCSIController.Status.Replicas,
// 		AvailablePodCount: gottenCSIController.Status.AvailableReplicas,
// 		WorkloadType: "Deployment",
// 	}

// 	if newClusterInstance.Status.LocalDiskManager == nil {
// 		newClusterInstance.Status.LocalDiskManager = &hwameistoriov1alpha1.LocalDiskManagerStatus{
// 			CSI: &csiDeployStatus,
// 		}
// 		return newClusterInstance, nil
// 	} else {
// 		if newClusterInstance.Status.LocalDiskManager.CSI == nil {
// 			newClusterInstance.Status.LocalDiskManager.CSI = &csiDeployStatus
// 			return newClusterInstance, nil
// 		} else {
// 			if !reflect.DeepEqual(newClusterInstance.Status.LocalDiskManager.CSI, csiDeployStatus) {
// 				newClusterInstance.Status.LocalDiskManager.CSI = &csiDeployStatus
// 				return newClusterInstance, nil
// 			}
// 		}
// 	}
// 	return newClusterInstance, nil
// }