package localdiskactioncontroller

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

type ActionControllerMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}


var actionControllerLabelSelectorKey = "app"
var actionControllerLabelSelectorValue = "hwameistor-local-disk-action-controller"
var ldaContainerName = "lda-controller"
var replicas = int32(1)

func NewActionControllerMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *ActionControllerMaintainer {
	return &ActionControllerMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var deployTemplate = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-local-disk-action-controller",
		Labels: map[string]string{
			actionControllerLabelSelectorKey: actionControllerLabelSelectorValue,
		},
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &replicas,
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				actionControllerLabelSelectorKey: actionControllerLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					actionControllerLabelSelectorKey: actionControllerLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "hwameistor-admin",
				Containers: []corev1.Container{
					{
						Name: ldaContainerName,
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
			},
		},
	},
}

func getActionControllerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalDiskActionController.Controller.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func needOrNotToUpdateActionController (cluster *hwameistoriov1alpha1.Cluster, gotten appsv1.Deployment) (bool, *appsv1.Deployment) {
	toUpdate := gotten.DeepCopy()
	var needToUpdate bool

	for i, container := range toUpdate.Spec.Template.Spec.Containers {
		if container.Name == ldaContainerName {
			wantedImage := getActionControllerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				toUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	// wantedReplicas := getActionControllerImageStringFromClusterInstance(cluster)
	// if *toUpdate.Spec.Replicas != wantedReplicas {
	// 	toUpdate.Spec.Replicas = &wantedReplicas
	// 	needToUpdate = true
	// }

	return needToUpdate, toUpdate
}

func SetActionController(clusterInstance *hwameistoriov1alpha1.Cluster) *appsv1.Deployment {
	deployToCreate := deployTemplate.DeepCopy()

	deployToCreate.Namespace = clusterInstance.Spec.TargetNamespace
	deployToCreate.OwnerReferences = append(deployToCreate.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	deployToCreate.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	for i, container := range deployToCreate.Spec.Template.Spec.Containers {
		if container.Name == ldaContainerName {
			container.Image = getActionControllerImageStringFromClusterInstance(clusterInstance)
			if resources := clusterInstance.Spec.LocalDiskActionController.Controller.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		deployToCreate.Spec.Template.Spec.Containers[i] = container
	}

	return deployToCreate
}

func (m *ActionControllerMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	deployToCreate := SetActionController(newClusterInstance)
	key := types.NamespacedName{
		Namespace: deployToCreate.Namespace,
		Name: deployToCreate.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), deployToCreate); errCreate != nil {
				log.Errorf("create localdiskactioncontroller err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("get localdiskactioncontroller err: %v", err)
			return newClusterInstance, err
		}
	}

	needToUpdate, toUpdate := needOrNotToUpdateActionController(newClusterInstance, gotten)
	if needToUpdate {
		log.Infof("need to update localdiskactioncontroller")
		if err := m.Client.Update(context.TODO(), toUpdate); err != nil {
			log.Errorf("update localdiskactioncontroller err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: deployToCreate.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[actionControllerLabelSelectorKey] == actionControllerLabelSelectorValue {
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
			Name: pod.Name,
			Node: pod.Spec.NodeName,
			Status: string(pod.Status.Phase),
		}
		podsStatus = append(podsStatus, podStatus)
	}

	instancesStatus := hwameistoriov1alpha1.DeployStatus{
		Pods: podsStatus,
		DesiredPodCount: gotten.Status.Replicas,
		AvailablePodCount: gotten.Status.AvailableReplicas,
		WorkloadType: "Deployment",
		WorkloadName: gotten.Name,
	}

	if newClusterInstance.Status.ComponentStatus.LocalDiskActionController == nil {
		newClusterInstance.Status.ComponentStatus.LocalDiskActionController = &hwameistoriov1alpha1.LocalDiskActionControllerStatus{
			Instances: &instancesStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.LocalDiskActionController.Instances == nil {
			newClusterInstance.Status.ComponentStatus.LocalDiskActionController.Instances = &instancesStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.LocalDiskActionController.Instances, instancesStatus) {
				newClusterInstance.Status.ComponentStatus.LocalDiskActionController.Instances = &instancesStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}