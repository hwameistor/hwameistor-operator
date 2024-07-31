package apiserver

import (
	"context"
	"errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
	"strconv"

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

type ApiServerMaintainer struct {
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewApiServerMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *ApiServerMaintainer {
	return &ApiServerMaintainer{
		Client:          cli,
		ClusterInstance: clusterInstance,
	}
}

var apiServerLabelSelectorKey = "app"
var apiServerLabelSelectorValue = "hwameistor-apiserver"
var defaultApiServerReplicas = int32(1)
var defaultApiServerImageRegistry = "ghcr.m.daocloud.io"
var defaultApiServerImageRepository = "hwameistor/apiserver"
var defaultApiServerImageTag = install.DefaultHwameistorVersion
var apiserverContainerName = "server"

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
						Name:            apiserverContainerName,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env: []corev1.EnvVar{
							{
								Name: "NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
							{
								Name: "NODENAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "spec.nodeName",
									},
								},
							},
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								ContainerPort: 80,
							},
						},
					},
				},
			},
		},
	},
}

func SetApiServer(clusterInstance *hwameistoriov1alpha1.Cluster) *appsv1.Deployment {
	resourceCreate := apiServer.DeepCopy()
	resourceCreate.Namespace = clusterInstance.Spec.TargetNamespace
	resourceCreate.OwnerReferences = append(resourceCreate.OwnerReferences, *metav1.NewControllerRef(clusterInstance, schema.FromAPIVersionAndKind("hwameistor.io/v1alpha1", "Cluster")))
	replicas := getApiserverReplicasFromClusterInstance(clusterInstance)
	resourceCreate.Spec.Replicas = &replicas
	resourceCreate.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	for i, container := range resourceCreate.Spec.Template.Spec.Containers {
		if container.Name == apiserverContainerName {
			container.Image = getApiserverContainerImageStringFromClusterInstance(clusterInstance)
			if resources := clusterInstance.Spec.ApiServer.Server.Resources; resources != nil {
				container.Resources = *resources
			}
			container.Env = append(container.Env, []corev1.EnvVar{
				{
					Name:  "EnableAuth",
					Value: strconv.FormatBool(clusterInstance.Spec.ApiServer.Authentication.Enable),
				},
				{
					Name:  "AuthAccessId",
					Value: clusterInstance.Spec.ApiServer.Authentication.AccessId,
				},
				{
					Name:  "AuthSecretKey",
					Value: clusterInstance.Spec.ApiServer.Authentication.SecretKey,
				},
			}...)
		}
		resourceCreate.Spec.Template.Spec.Containers[i] = container
	}
	return resourceCreate
}

func getApiserverContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.ApiServer.Server.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getApiserverReplicasFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) int32 {
	return clusterInstance.Spec.ApiServer.Replicas
}

func needOrNotToUpdateApiserver(cluster *hwameistoriov1alpha1.Cluster, gottenApiserver appsv1.Deployment) (bool, *appsv1.Deployment) {
	apiserverToUpdate := gottenApiserver.DeepCopy()
	var needToUpdate bool

	for i, container := range apiserverToUpdate.Spec.Template.Spec.Containers {
		if container.Name == apiserverContainerName {
			wantedImage := getApiserverContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				apiserverToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	wantedReplicas := getApiserverReplicasFromClusterInstance(cluster)
	if *apiserverToUpdate.Spec.Replicas != wantedReplicas {
		apiserverToUpdate.Spec.Replicas = &wantedReplicas
		needToUpdate = true
	}

	return needToUpdate, apiserverToUpdate
}

func (m *ApiServerMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	resourceCreate := SetApiServer(newClusterInstance)
	key := types.NamespacedName{
		Namespace: resourceCreate.Namespace,
		Name:      resourceCreate.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), resourceCreate); errCreate != nil {
				log.Errorf("Create ApiServer err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get ApiServer err: %v", err)
			return newClusterInstance, err
		}
	}

	needToUpdate, apiserverToUpdate := needOrNotToUpdateApiserver(newClusterInstance, gotten)
	if needToUpdate {
		log.Infof("need to update apiserver")
		if err := m.Client.Update(context.TODO(), apiserverToUpdate); err != nil {
			log.Errorf("Update apiserver err: %v", err)
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

	if newClusterInstance.Status.ComponentStatus.ApiServer == nil {
		newClusterInstance.Status.ComponentStatus.ApiServer = &hwameistoriov1alpha1.ApiServerStatus{
			Instances: &instancesStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.ApiServer.Instances == nil {
			newClusterInstance.Status.ComponentStatus.ApiServer.Instances = &instancesStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.ApiServer.Instances, instancesStatus) {
				newClusterInstance.Status.ComponentStatus.ApiServer.Instances = &instancesStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func (m *ApiServerMaintainer) Uninstall() error {
	key := types.NamespacedName{
		Namespace: m.ClusterInstance.Spec.TargetNamespace,
		Name:      apiServer.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			log.Errorf("get ApiServer err: %v", err)
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
	log.Errorf("ApiServer Owner is not %s", m.ClusterInstance.Name)
	return nil
}

func FulfillApiServerSpec(clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.ApiServer == nil {
		clusterInstance.Spec.ApiServer = &hwameistoriov1alpha1.ApiServerSpec{}
	}
	if clusterInstance.Spec.ApiServer.Replicas == 0 {
		clusterInstance.Spec.ApiServer.Replicas = defaultApiServerReplicas
	}
	if clusterInstance.Spec.ApiServer.Authentication == nil {
		clusterInstance.Spec.ApiServer.Authentication = &hwameistoriov1alpha1.Authentication{}
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
