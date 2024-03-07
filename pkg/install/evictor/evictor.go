package evictor

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

type EvictorMaintainer struct {
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *EvictorMaintainer {
	return &EvictorMaintainer{
		Client:          cli,
		ClusterInstance: clusterInstance,
	}
}

var replicas = int32(1)
var evictorLabelSelectorKey = "app"
var evictorLabelSelectorValue = "hwameistor-volume-evictor"
var defaultEvictorImageRegistry = "ghcr.m.daocloud.io"
var defaultEvictorImageRepository = "hwameistor/evictor"
var defaultEvictorImageTag = install.DefaultHwameistorVersion
var evictorContainerName = "evictor"

var evictorDeployment = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-volume-evictor",
		Labels: map[string]string{
			evictorLabelSelectorKey: evictorLabelSelectorValue,
		},
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &replicas,
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				evictorLabelSelectorKey: evictorLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					evictorLabelSelectorKey: evictorLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            evictorContainerName,
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
			},
		},
	},
}

func SetEvictor(clusterInstance *hwameistoriov1alpha1.Cluster) {
	evictorDeployment.Namespace = clusterInstance.Spec.TargetNamespace
	evictorDeployment.OwnerReferences = append(evictorDeployment.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	replicas := getEvictorReplicasFromClusterInstance(clusterInstance)
	evictorDeployment.Spec.Replicas = &replicas
	evictorDeployment.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	setEvictorContainers(clusterInstance)
}

func setEvictorContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
	for i, container := range evictorDeployment.Spec.Template.Spec.Containers {
		if container.Name == evictorContainerName {
			// container.Resources = *clusterInstance.Spec.Evictor.Evictor.Resources
			if resources := clusterInstance.Spec.Evictor.Evictor.Resources; resources != nil {
				container.Resources = *resources
			}
			container.Image = getEvictorContainerImageStringFromClusterInstance(clusterInstance)
		}
		evictorDeployment.Spec.Template.Spec.Containers[i] = container
	}
}

func getEvictorContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.Evictor.Evictor.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getEvictorReplicasFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) int32 {
	return clusterInstance.Spec.Evictor.Replicas
}

func needOrNotToUpdateEvictor(cluster *hwameistoriov1alpha1.Cluster, gottenEvictor appsv1.Deployment) (bool, *appsv1.Deployment) {
	evictorToUpdate := gottenEvictor.DeepCopy()
	var needToUpdate bool

	for i, container := range evictorToUpdate.Spec.Template.Spec.Containers {
		if container.Name == evictorContainerName {
			wantedImage := getEvictorContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				evictorToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	wantedReplicas := getEvictorReplicasFromClusterInstance(cluster)
	if *evictorToUpdate.Spec.Replicas != wantedReplicas {
		evictorToUpdate.Spec.Replicas = &wantedReplicas
		needToUpdate = true
	}

	return needToUpdate, evictorToUpdate
}

func (m *EvictorMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	SetEvictor(newClusterInstance)
	key := types.NamespacedName{
		Namespace: evictorDeployment.Namespace,
		Name:      evictorDeployment.Name,
	}

	if newClusterInstance.Spec.Evictor.Disable {
		d := &appsv1.Deployment{}
		if err := m.Client.Get(context.TODO(), key, d); err != nil {
			if apierrors.IsNotFound(err) {
				return newClusterInstance, nil
			}
			log.Errorf("Get Evictor err:%v", err)
			return newClusterInstance, err
		}
		if err := m.Client.Delete(context.TODO(), d); err != nil {
			log.Errorf("Delete Evictor err:%v", err)
			return newClusterInstance, err
		}
	}

	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &evictorDeployment); errCreate != nil {
				log.Errorf("Create Evictor err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get Evictor err: %v", err)
			return newClusterInstance, err
		}
	}

	needToUpdate, evictorToUpdate := needOrNotToUpdateEvictor(newClusterInstance, gotten)
	if needToUpdate {
		log.Infof("need to update evictor")
		if err := m.Client.Update(context.TODO(), evictorToUpdate); err != nil {
			log.Errorf("Update evictor err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: evictorDeployment.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[evictorLabelSelectorKey] == evictorLabelSelectorValue {
			podsManaged = append(podsManaged, pod)
		}
	}

	if len(podsManaged) > int(gotten.Status.Replicas) {
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

	instancesStatus := hwameistoriov1alpha1.DeployStatus{
		Pods:              podsStatus,
		DesiredPodCount:   gotten.Status.Replicas,
		AvailablePodCount: gotten.Status.AvailableReplicas,
		WorkloadType:      "Deployment",
		WorkloadName:      gotten.Name,
	}

	if newClusterInstance.Status.ComponentStatus.Evictor == nil {
		newClusterInstance.Status.ComponentStatus.Evictor = &hwameistoriov1alpha1.EvictorStatus{
			Instances: &instancesStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.Evictor.Instances == nil {
			newClusterInstance.Status.ComponentStatus.Evictor.Instances = &instancesStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.Evictor.Instances, instancesStatus) {
				newClusterInstance.Status.ComponentStatus.Evictor.Instances = &instancesStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func FulfillEvictorSpec(clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.Evictor == nil {
		clusterInstance.Spec.Evictor = &hwameistoriov1alpha1.EvictorSpec{}
	}
	if clusterInstance.Spec.Evictor.Evictor == nil {
		clusterInstance.Spec.Evictor.Evictor = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.Evictor.Evictor.Image == nil {
		clusterInstance.Spec.Evictor.Evictor.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.Evictor.Evictor.Image.Registry == "" {
		clusterInstance.Spec.Evictor.Evictor.Image.Registry = defaultEvictorImageRegistry
	}
	if clusterInstance.Spec.Evictor.Evictor.Image.Repository == "" {
		clusterInstance.Spec.Evictor.Evictor.Image.Repository = defaultEvictorImageRepository
	}
	if clusterInstance.Spec.Evictor.Evictor.Image.Tag == "" {
		clusterInstance.Spec.Evictor.Evictor.Image.Tag = defaultEvictorImageTag
	}

	return clusterInstance
}
