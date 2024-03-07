package auditor

import (
	"context"
	"errors"
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

type AuditorMaintainer struct {
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewAuditorMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *AuditorMaintainer {
	return &AuditorMaintainer{
		Client:          cli,
		ClusterInstance: clusterInstance,
	}
}

var auditorLabelSelectorKey = "app"
var auditorLabelSelectorValue = "hwameistor-auditor"
var auditorContainerName = "auditor"

var auditorTemplate = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-auditor",
		Labels: map[string]string{
			auditorLabelSelectorKey: auditorLabelSelectorValue,
		},
	},
	Spec: appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				auditorLabelSelectorKey: auditorLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					auditorLabelSelectorKey: auditorLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            auditorContainerName,
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
			},
		},
	},
}

func getAuditorReplicasFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) int32 {
	return clusterInstance.Spec.Auditor.Replicas
}

func getAuditorContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.Auditor.Auditor.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func needOrNotToUpdateAuditor(cluster *hwameistoriov1alpha1.Cluster, gottenAuditor appsv1.Deployment) (bool, *appsv1.Deployment) {
	auditorToUpdate := gottenAuditor.DeepCopy()
	var needToUpdate bool

	for i, container := range auditorToUpdate.Spec.Template.Spec.Containers {
		if container.Name == auditorContainerName {
			wantedImage := getAuditorContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				auditorToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	wantedReplicas := getAuditorReplicasFromClusterInstance(cluster)
	if *auditorToUpdate.Spec.Replicas != wantedReplicas {
		auditorToUpdate.Spec.Replicas = &wantedReplicas
		needToUpdate = true
	}

	return needToUpdate, auditorToUpdate
}

func SetAuditor(clusterInstance *hwameistoriov1alpha1.Cluster) *appsv1.Deployment {
	auditorToCreate := auditorTemplate.DeepCopy()

	auditorToCreate.Namespace = clusterInstance.Spec.TargetNamespace
	auditorToCreate.OwnerReferences = append(auditorToCreate.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	replicas := getAuditorReplicasFromClusterInstance(clusterInstance)
	auditorToCreate.Spec.Replicas = &replicas
	auditorToCreate.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	for i, container := range auditorToCreate.Spec.Template.Spec.Containers {
		if container.Name == auditorContainerName {
			container.Image = getAuditorContainerImageStringFromClusterInstance(clusterInstance)
			if resources := clusterInstance.Spec.Auditor.Auditor.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		auditorToCreate.Spec.Template.Spec.Containers[i] = container
	}

	return auditorToCreate
}

func (m *AuditorMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	auditorToCreate := SetAuditor(newClusterInstance)
	key := types.NamespacedName{
		Namespace: auditorToCreate.Namespace,
		Name:      auditorToCreate.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), auditorToCreate); errCreate != nil {
				log.Errorf("create auditor err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("get auditor err: %v", err)
			return newClusterInstance, err
		}
	}

	needToUpdate, auditorToUpdate := needOrNotToUpdateAuditor(newClusterInstance, gotten)
	if needToUpdate {
		log.Infof("need to update auditor")
		if err := m.Client.Update(context.TODO(), auditorToUpdate); err != nil {
			log.Errorf("update auditor err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: auditorToCreate.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[auditorLabelSelectorKey] == auditorLabelSelectorValue {
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

	if newClusterInstance.Status.ComponentStatus.Auditor == nil {
		newClusterInstance.Status.ComponentStatus.Auditor = &hwameistoriov1alpha1.AuditorStatus{
			Instances: &instancesStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.Auditor.Instances == nil {
			newClusterInstance.Status.ComponentStatus.Auditor.Instances = &instancesStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.Auditor.Instances, instancesStatus) {
				newClusterInstance.Status.ComponentStatus.Auditor.Instances = &instancesStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}
