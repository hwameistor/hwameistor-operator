package apiserver

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

type ApiServerMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewApiServerMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *ApiServerMaintainer {
	return &ApiServerMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var apiServerLabelSelectorKey = "app"
var apiServerLabelSelectorValue = "hwameistor-apiserver"
var defaultApiServerReplicas = int32(1)
var defaultApiServerImageRegistry = "ghcr.m.daocloud.io"
var defaultApiServerImageRepository = "hwameistor/apiserver"
var defaultApiServerImageTag = "v0.7.1"

var apiServer = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-apiserver",
		Labels: map[string]string{
			apiServerLabelSelectorKey: apiServerLabelSelectorValue,
		},
	},
	Spec: appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				apiServerLabelSelectorKey: apiServerLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					apiServerLabelSelectorKey: apiServerLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "server",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{
								Name: "http",
								ContainerPort: 80,
							},
						},
					},
				},
			},
		},
	},
}

func SetApiServer(clusterInstance *hwameistoriov1alpha1.Cluster) {
	apiServer.Namespace = clusterInstance.Spec.TargetNamespace
	apiServer.OwnerReferences = append(apiServer.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	apiServer.Spec.Replicas = &clusterInstance.Spec.ApiServer.Replicas
	apiServer.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	for i, container := range apiServer.Spec.Template.Spec.Containers {
		if container.Name == "server" {
			imageSpec := clusterInstance.Spec.ApiServer.Server.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
		}
		apiServer.Spec.Template.Spec.Containers[i] = container
	}
}

func (m *ApiServerMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	SetApiServer(newClusterInstance)
	key := types.NamespacedName{
		Namespace: apiServer.Namespace,
		Name: apiServer.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &apiServer); errCreate != nil {
				log.Errorf("Create ApiServer err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get ApiServer err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: apiServer.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[apiServerLabelSelectorKey] == apiServerLabelSelectorValue {
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

	if newClusterInstance.Status.ApiServer == nil {
		newClusterInstance.Status.ApiServer = &hwameistoriov1alpha1.ApiServerStatus{
			Instances: &instancesStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ApiServer.Instances == nil {
			newClusterInstance.Status.ApiServer.Instances = &instancesStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ApiServer.Instances, instancesStatus) {
				newClusterInstance.Status.ApiServer.Instances = &instancesStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func FulfillApiServerSpec (clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.ApiServer == nil {
		clusterInstance.Spec.ApiServer = &hwameistoriov1alpha1.ApiServerSpec{}
	}
	if clusterInstance.Spec.ApiServer.Replicas == 0 {
		clusterInstance.Spec.ApiServer.Replicas = defaultApiServerReplicas
	}
	if clusterInstance.Spec.ApiServer.Server == nil {
		clusterInstance.Spec.ApiServer.Server = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.ApiServer.Server.Image == nil {
		clusterInstance.Spec.ApiServer.Server.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.ApiServer.Server.Image.Registry == "" {
		clusterInstance.Spec.ApiServer.Server.Image.Registry = defaultApiServerImageRegistry
	}
	if clusterInstance.Spec.ApiServer.Server.Image.Repository == "" {
		clusterInstance.Spec.ApiServer.Server.Image.Repository = defaultApiServerImageRepository
	}
	if clusterInstance.Spec.ApiServer.Server.Image.Tag == "" {
		clusterInstance.Spec.ApiServer.Server.Image.Tag = defaultApiServerImageTag
	}

	return clusterInstance
}