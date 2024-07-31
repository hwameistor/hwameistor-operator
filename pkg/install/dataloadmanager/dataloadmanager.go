package dataloadmanager

import (
	"context"
	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"github.com/hwameistor/hwameistor-operator/pkg/install"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var dlmDaemonSetLabelSelectorKey = "app"
var dlmDaemonSetLabelSelectorValue = "hwameistor-dataload-manager"
var dlmContainerName = "dataload-manager"
var defaultDLMDaemonsetImageRegistry = "ghcr.m.daocloud.io"
var defaultDLMDaemonsetImageRepository = "hwameistor/dataload-manager"
var defaultDLMDaemonsetImageTag = "v0.0.1"

type DataLoadManagerMaintainer struct {
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *DataLoadManagerMaintainer {
	return &DataLoadManagerMaintainer{
		Client:          cli,
		ClusterInstance: clusterInstance,
	}
}

var dlmDaemonSetTemplate = appsv1.DaemonSet{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-dataload-manager",
	},
	Spec: appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				dlmDaemonSetLabelSelectorKey: dlmDaemonSetLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					dlmDaemonSetLabelSelectorKey: dlmDaemonSetLabelSelectorValue,
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
											Operator: corev1.NodeSelectorOpNotIn,
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
						Name: dlmContainerName,
						//Image:           "10.6.118.138:5000/hwameistor:dataload_manager_99.9-dev",
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: &corev1.SecurityContext{
							Privileged: &install.SecurityContextPrivilegedTrue,
						},
						Env: []corev1.EnvVar{
							{
								Name: "MY_NODENAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath:  "spec.nodeName",
									},
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:             "host-dev",
								MountPath:        "/dev",
								MountPropagation: nil,
							},
							{
								Name:             "host-mnt",
								MountPath:        "/mnt",
								MountPropagation: &[]corev1.MountPropagationMode{corev1.MountPropagationBidirectional}[0],
							},
						},
						Args: []string{"--nodename=$(MY_NODENAME)"},
					},
				},
				DNSPolicy: corev1.DNSClusterFirstWithHostNet,
				HostPID:   true,
				Volumes: []corev1.Volume{
					{
						Name: "host-dev",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/dev",
							},
						},
					},
					{
						Name: "host-mnt",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/mnt",
								Type: &[]corev1.HostPathType{corev1.HostPathDirectoryOrCreate}[0],
							},
						},
					},
				},
			},
		},
		UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
			RollingUpdate: &appsv1.RollingUpdateDaemonSet{
				MaxUnavailable: &[]intstr.IntOrString{intstr.FromInt(1)}[0],
			},
			Type: appsv1.RollingUpdateDaemonSetStrategyType,
		},
	},
}

func SetDLMDaemonSet(clusterInstance *hwameistoriov1alpha1.Cluster) *appsv1.DaemonSet {
	dlmDaemonSetToCreate := dlmDaemonSetTemplate.DeepCopy()
	dlmDaemonSetToCreate.OwnerReferences = append(dlmDaemonSetToCreate.OwnerReferences, *metav1.NewControllerRef(clusterInstance, schema.FromAPIVersionAndKind("hwameistor.io/v1alpha1", "Cluster")))
	dlmDaemonSetToCreate.Namespace = clusterInstance.Spec.TargetNamespace
	dlmDaemonSetToCreate.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	dlmDaemonSetToCreate = setDLMDaemonSetContainers(clusterInstance, dlmDaemonSetToCreate)

	return dlmDaemonSetToCreate

}

func setDLMDaemonSetContainers(clusterInstance *hwameistoriov1alpha1.Cluster, dlmDaemonSetToCreate *appsv1.DaemonSet) *appsv1.DaemonSet {
	for i, container := range dlmDaemonSetToCreate.Spec.Template.Spec.Containers {
		if container.Name == dlmContainerName {
			container.Image = getDLMContainerRegistrarImageStringFromClusterInstance(clusterInstance)
		}
		dlmDaemonSetToCreate.Spec.Template.Spec.Containers[i] = container
	}
	return dlmDaemonSetToCreate
}

func getDLMContainerRegistrarImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func needOrNotToUpdateDLMDaemonset(cluster *hwameistoriov1alpha1.Cluster, gotten appsv1.DaemonSet) (bool, *appsv1.DaemonSet) {
	ds := gotten.DeepCopy()
	var needToUpdate bool

	for i, container := range ds.Spec.Template.Spec.Containers {
		if container.Name == dlmContainerName {
			wantedImage := getDLMContainerRegistrarImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				ds.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	return needToUpdate, ds
}

func (m *DataLoadManagerMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	dlmDaemonSetToCreate := SetDLMDaemonSet(newClusterInstance)
	key := types.NamespacedName{
		Namespace: dlmDaemonSetToCreate.Namespace,
		Name:      dlmDaemonSetToCreate.Name,
	}
	var gottenDS appsv1.DaemonSet
	if err := m.Client.Get(context.TODO(), key, &gottenDS); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), dlmDaemonSetToCreate); errCreate != nil {
				log.Errorf("Create DataLoadManager DaemonSet err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get DataLoadManager DaemonSet err: %v", err)
			return newClusterInstance, err
		}
	}
	needToUpdate, dsToUpdate := needOrNotToUpdateDLMDaemonset(newClusterInstance, gottenDS)
	if needToUpdate {
		log.Infof("need to update DataLoadManager daemonset")
		if err := m.Client.Update(context.TODO(), dsToUpdate); err != nil {
			log.Errorf("Update DataLoadManager daemonset err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: dlmDaemonSetToCreate.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[dlmDaemonSetLabelSelectorKey] == dlmDaemonSetLabelSelectorValue {
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

	if newClusterInstance.Status.ComponentStatus.DataLoadManager == nil {
		newClusterInstance.Status.ComponentStatus.DataLoadManager = &hwameistoriov1alpha1.DataLoadManagerStatus{
			Instances: &instancesDeployStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.DataLoadManager.Instances == nil {
			newClusterInstance.Status.ComponentStatus.DataLoadManager.Instances = &instancesDeployStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.DataLoadManager.Instances, instancesDeployStatus) {
				newClusterInstance.Status.ComponentStatus.DataLoadManager.Instances = &instancesDeployStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func (m *DataLoadManagerMaintainer) Uninstall() error {
	key := types.NamespacedName{
		Namespace: m.ClusterInstance.Spec.TargetNamespace,
		Name:      dlmDaemonSetTemplate.Name,
	}
	var gotten appsv1.DaemonSet
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			log.Errorf("get DataLoadManager err: %v", err)
			return err
		}
	} else {
		for _, reference := range gotten.OwnerReferences {
			if reference.Name == m.ClusterInstance.Name {
				if err = m.Client.Delete(context.TODO(), &gotten); err != nil {
					return err
				} else {
					return nil
				}
			}
		}
	}
	log.Errorf("DataLoadManager Owner is not %s,can't delete.  ", m.ClusterInstance.Name)
	return nil
}

func FulfillDataLoadManagerSpec(clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.DataLoadManager == nil {
		clusterInstance.Spec.DataLoadManager = &hwameistoriov1alpha1.DataLoadManagerSpec{}
	}

	if clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer == nil {
		clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}

	if clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer.Image == nil {
		clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer.Image = &hwameistoriov1alpha1.ImageSpec{}
	}

	if clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer.Image.Registry == "" {
		clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer.Image.Registry = defaultDLMDaemonsetImageRegistry
	}

	if clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer.Image.Repository == "" {
		clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer.Image.Repository = defaultDLMDaemonsetImageRepository
	}
	if clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer.Image.Tag == "" {
		clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer.Image.Tag = defaultDLMDaemonsetImageTag
	}
	return clusterInstance
}
