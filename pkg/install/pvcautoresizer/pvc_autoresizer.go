package pvcautoresizer

import (
	"context"
	"errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PVCAutoResizerMaintainer struct {
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewPVCAutoResizerMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *PVCAutoResizerMaintainer {
	return &PVCAutoResizerMaintainer{
		Client:          cli,
		ClusterInstance: clusterInstance,
	}
}

var pvcAutoResizerLabelSelectorKey = "app"
var pvcAutoResizerLabelSelectorValue = "hwameistor-pvc-autoresizer"
var pvcAutoResizerContainerName = "pvc-autoresizer"

var deployTemplate = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-pvc-autoresizer",
		Labels: map[string]string{
			pvcAutoResizerLabelSelectorKey: pvcAutoResizerLabelSelectorValue,
		},
	},
	Spec: appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				pvcAutoResizerLabelSelectorKey: pvcAutoResizerLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					pvcAutoResizerLabelSelectorKey: pvcAutoResizerLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            pvcAutoResizerContainerName,
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
			},
		},
	},
}

func getPVCAutoResizerReplicasFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) int32 {
	return clusterInstance.Spec.PVCAutoResizer.Replicas
}

func getPVCAutoResizerContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.PVCAutoResizer.AutoResizer.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func needOrNotToUpdatePVCAutoResizer(cluster *hwameistoriov1alpha1.Cluster, gotten appsv1.Deployment) (bool, *appsv1.Deployment) {
	toUpdate := gotten.DeepCopy()
	var needToUpdate bool

	for i, container := range toUpdate.Spec.Template.Spec.Containers {
		if container.Name == pvcAutoResizerContainerName {
			wantedImage := getPVCAutoResizerContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				toUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	wantedReplicas := getPVCAutoResizerReplicasFromClusterInstance(cluster)
	if *toUpdate.Spec.Replicas != wantedReplicas {
		toUpdate.Spec.Replicas = &wantedReplicas
		needToUpdate = true
	}

	return needToUpdate, toUpdate
}

func SetPVCAutoResizer(clusterInstance *hwameistoriov1alpha1.Cluster) *appsv1.Deployment {
	deployToCreate := deployTemplate.DeepCopy()

	deployToCreate.Namespace = clusterInstance.Spec.TargetNamespace
	deployToCreate.OwnerReferences = append(deployToCreate.OwnerReferences, *metav1.NewControllerRef(clusterInstance, schema.FromAPIVersionAndKind("hwameistor.io/v1alpha1", "Cluster")))
	replicas := getPVCAutoResizerReplicasFromClusterInstance(clusterInstance)
	deployToCreate.Spec.Replicas = &replicas
	deployToCreate.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	for i, container := range deployToCreate.Spec.Template.Spec.Containers {
		if container.Name == pvcAutoResizerContainerName {
			container.Image = getPVCAutoResizerContainerImageStringFromClusterInstance(clusterInstance)
			if resources := clusterInstance.Spec.PVCAutoResizer.AutoResizer.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		deployToCreate.Spec.Template.Spec.Containers[i] = container
	}

	return deployToCreate
}

func (m *PVCAutoResizerMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	deployToCreate := SetPVCAutoResizer(newClusterInstance)
	key := types.NamespacedName{
		Namespace: deployToCreate.Namespace,
		Name:      deployToCreate.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), deployToCreate); errCreate != nil {
				log.Errorf("create pvc-autoresizer err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("get pvc-autoresizer err: %v", err)
			return newClusterInstance, err
		}
	}

	needToUpdate, toUpdate := needOrNotToUpdatePVCAutoResizer(newClusterInstance, gotten)
	if needToUpdate {
		log.Infof("need to update pvc-autoresizer")
		if err := m.Client.Update(context.TODO(), toUpdate); err != nil {
			log.Errorf("update pvc-autoresizer err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	selector, err := metav1.LabelSelectorAsSelector(deployToCreate.Spec.Selector)
	if err != nil {
		log.Errorf("convert LabelSelector to Selector err: %v", err)
	}
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{LabelSelector: selector}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		podsManaged = append(podsManaged, pod)
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

	if newClusterInstance.Status.ComponentStatus.PVCAutoResizer == nil {
		newClusterInstance.Status.ComponentStatus.PVCAutoResizer = &hwameistoriov1alpha1.PVCAutoResizerStatus{
			Instances: &instancesStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.PVCAutoResizer.Instances == nil {
			newClusterInstance.Status.ComponentStatus.PVCAutoResizer.Instances = &instancesStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.PVCAutoResizer.Instances, instancesStatus) {
				newClusterInstance.Status.ComponentStatus.PVCAutoResizer.Instances = &instancesStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func (m *PVCAutoResizerMaintainer) Uninstall() error {
	key := types.NamespacedName{
		Namespace: m.ClusterInstance.Spec.TargetNamespace,
		Name:      deployTemplate.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			log.Errorf("get PVCAutoResizer err: %v", err)
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
	log.Errorf("PVCAutoResizer Owner is not %s,can't delete ", m.ClusterInstance.Name)
	return nil
}
