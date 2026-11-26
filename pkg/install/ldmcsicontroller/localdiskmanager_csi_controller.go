package ldmcsicontroller

import (
	"context"
	"errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

type LDMCSIMaintainer struct {
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *LDMCSIMaintainer {
	return &LDMCSIMaintainer{
		Client:          cli,
		ClusterInstance: clusterInstance,
	}
}

var ldmCSIControllerReplicas = int32(1)
var ldmCSIControllerLabelSelectorKey = "app"
var ldmCSIControllerLabelSelectorValue = "hwameistor-local-disk-csi-controller"
var defaultKubeletRootDir = "/var/lib/kubelet"
var defaultLDMCSIProvisionerRegistry = "k8s-gcr.m.daocloud.io"
var defaultLDMCSIProvisionerRepository = "sig-storage/csi-provisioner"
var defaultLDMCSIProvisionerTag = "v2.0.3"
var defaultLDMCSIAttacherRegistry = "k8s-gcr.m.daocloud.io"
var defaultLDMCSIAttacherRepository = "sig-storage/csi-attacher"
var defaultLDMCSIAttacherTag = "v3.0.1"
var provisionerContainerName = "provisioner"
var attacherContainerName = "attacher"

var ldmCSIController = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-local-disk-csi-controller",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &ldmCSIControllerReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				ldmCSIControllerLabelSelectorKey: ldmCSIControllerLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					ldmCSIControllerLabelSelectorKey: ldmCSIControllerLabelSelectorValue,
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
											Key:      "app",
											Operator: "In",
											Values:   []string{"hwameistor-local-disk-manager"},
										},
									},
								},
								TopologyKey: "topology.disk.hwameistor.io/node",
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:            provisionerContainerName,
						ImagePullPolicy: "IfNotPresent",
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
								Name:  "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "socket-dir",
								MountPath: "/csi",
							},
						},
					},
					{
						Name:            attacherContainerName,
						ImagePullPolicy: "IfNotPresent",
						Args: []string{
							"--v=5",
							"--csi-address=$(CSI_ADDRESS)",
							"--leader-election=true",
							"--timeout=120s",
						},
						Env: []corev1.EnvVar{
							{
								Name:  "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "socket-dir",
								MountPath: "/csi",
							},
						},
					},
				},
			},
		},
	},
}

func SetLDMCSIController(clusterInstance *hwameistoriov1alpha1.Cluster) *appsv1.Deployment {
	ldmCSIControllerToCreate := ldmCSIController.DeepCopy()
	ldmCSIControllerToCreate.Namespace = clusterInstance.Spec.TargetNamespace
	ldmCSIControllerToCreate.OwnerReferences = append(ldmCSIControllerToCreate.OwnerReferences, *metav1.NewControllerRef(clusterInstance, schema.FromAPIVersionAndKind("hwameistor.io/v1alpha1", "Cluster")))
	// ldmCSIController.Spec.Template.Spec.PriorityClassName = clusterInstance.Spec.LocalDiskManager.CSI.Controller.Common.PriorityClassName
	replicas := getReplicasFromClusterInstance(clusterInstance)
	ldmCSIControllerToCreate.Spec.Replicas = &replicas
	ldmCSIControllerToCreate.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	ldmCSIControllerToCreate = setLDMCSIControllerVolumes(clusterInstance, ldmCSIControllerToCreate)
	ldmCSIControllerToCreate = setLDMCSIControllerContainers(clusterInstance, ldmCSIControllerToCreate)

	return ldmCSIControllerToCreate
}

func setLDMCSIControllerVolumes(clusterInstance *hwameistoriov1alpha1.Cluster, ldmCSIControllerToCreate *appsv1.Deployment) *appsv1.Deployment {
	volume := corev1.Volume{
		Name: "socket-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Type: &install.HostPathDirectoryOrCreate,
				Path: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins/disk.hwameistor.io",
			},
		},
	}

	ldmCSIControllerToCreate.Spec.Template.Spec.Volumes = append(ldmCSIControllerToCreate.Spec.Template.Spec.Volumes, volume)

	return ldmCSIControllerToCreate
}

func setLDMCSIControllerContainers(clusterInstance *hwameistoriov1alpha1.Cluster, ldmCSIControllerToCreate *appsv1.Deployment) *appsv1.Deployment {
	for i, container := range ldmCSIControllerToCreate.Spec.Template.Spec.Containers {
		if container.Name == provisionerContainerName {
			// container.Resources = *clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Resources
			if resources := clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Resources; resources != nil {
				container.Resources = *resources
			}
			container.Image = getProvisionerContainerImageStringFromClusterInstance(clusterInstance)
		}
		if container.Name == attacherContainerName {
			// container.Resources = *clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Resources
			if resources := clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Resources; resources != nil {
				container.Resources = *resources
			}
			container.Image = getAttacherContainerImageStringFromClusterInstance(clusterInstance)
		}

		ldmCSIControllerToCreate.Spec.Template.Spec.Containers[i] = container
	}
	return ldmCSIControllerToCreate
}

func getProvisionerContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getAttacherContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getReplicasFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) int32 {
	return clusterInstance.Spec.LocalDiskManager.CSI.Controller.Replicas
}

func needOrNotToUpdateLDMCSIController(cluster *hwameistoriov1alpha1.Cluster, gottenLDMCSIController appsv1.Deployment) (bool, *appsv1.Deployment) {
	ldmCSIControllerToUpdate := gottenLDMCSIController.DeepCopy()
	var needToUpdate bool

	for i, container := range ldmCSIControllerToUpdate.Spec.Template.Spec.Containers {
		if container.Name == provisionerContainerName {
			wantedImage := getProvisionerContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				ldmCSIControllerToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
		if container.Name == attacherContainerName {
			wantedImage := getAttacherContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				ldmCSIControllerToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	wantedReplicas := getReplicasFromClusterInstance(cluster)
	if *ldmCSIControllerToUpdate.Spec.Replicas != wantedReplicas {
		ldmCSIControllerToUpdate.Spec.Replicas = &wantedReplicas
		needToUpdate = true
	}

	return needToUpdate, ldmCSIControllerToUpdate
}

func (m *LDMCSIMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	ldmCSIControllerToCreate := SetLDMCSIController(newClusterInstance)
	key := types.NamespacedName{
		Namespace: ldmCSIControllerToCreate.Namespace,
		Name:      ldmCSIControllerToCreate.Name,
	}
	var gottenCSIController appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gottenCSIController); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), ldmCSIControllerToCreate); errCreate != nil {
				log.Errorf("Create LDM CSIController err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get LDM CSIController err: %v", err)
			return newClusterInstance, err
		}
	}

	needToUpdate, csiControllerToUpdate := needOrNotToUpdateLDMCSIController(newClusterInstance, gottenCSIController)
	if needToUpdate {
		log.Infof("need to update ldm csiController")
		if err := m.Client.Update(context.TODO(), csiControllerToUpdate); err != nil {
			log.Errorf("Update ldm csiController err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: ldmCSIControllerToCreate.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[ldmCSIControllerLabelSelectorKey] == ldmCSIControllerLabelSelectorValue {
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
			Name:   pod.Name,
			Node:   pod.Spec.NodeName,
			Status: string(pod.Status.Phase),
		}
		podsStatus = append(podsStatus, podStatus)
	}

	csiDeployStatus := hwameistoriov1alpha1.DeployStatus{
		Pods:              podsStatus,
		DesiredPodCount:   gottenCSIController.Status.Replicas,
		AvailablePodCount: gottenCSIController.Status.AvailableReplicas,
		WorkloadType:      "Deployment",
		WorkloadName:      gottenCSIController.Name,
	}

	if newClusterInstance.Status.ComponentStatus.LocalDiskManager == nil {
		newClusterInstance.Status.ComponentStatus.LocalDiskManager = &hwameistoriov1alpha1.LocalDiskManagerStatus{
			CSI: &csiDeployStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.LocalDiskManager.CSI == nil {
			newClusterInstance.Status.ComponentStatus.LocalDiskManager.CSI = &csiDeployStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.LocalDiskManager.CSI, csiDeployStatus) {
				newClusterInstance.Status.ComponentStatus.LocalDiskManager.CSI = &csiDeployStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func FulfillLDMCSISpec(clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.LocalDiskManager == nil {
		clusterInstance.Spec.LocalDiskManager = &hwameistoriov1alpha1.LocalDiskManagerSpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.KubeletRootDir == "" {
		clusterInstance.Spec.LocalDiskManager.KubeletRootDir = defaultKubeletRootDir
	}
	if clusterInstance.Spec.LocalDiskManager.CSI == nil {
		clusterInstance.Spec.LocalDiskManager.CSI = &hwameistoriov1alpha1.CSISpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Controller == nil {
		clusterInstance.Spec.LocalDiskManager.CSI.Controller = &hwameistoriov1alpha1.CSIControllerSpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner == nil {
		clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Image == nil {
		clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Image.Registry == "" {
		clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Image.Registry = defaultLDMCSIProvisionerRegistry
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Image.Repository == "" {
		clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Image.Repository = defaultLDMCSIProvisionerRepository
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Image.Tag == "" {
		clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Image.Tag = defaultLDMCSIProvisionerTag
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher == nil {
		clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Image == nil {
		clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Image.Registry == "" {
		clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Image.Registry = defaultLDMCSIAttacherRegistry
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Image.Repository == "" {
		clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Image.Repository = defaultLDMCSIAttacherRepository
	}
	if clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Image.Tag == "" {
		clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Image.Tag = defaultLDMCSIAttacherTag
	}

	return clusterInstance
}
