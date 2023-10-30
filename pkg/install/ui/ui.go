package ui

import (
	"context"

	operatorv1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"github.com/hwameistor/hwameistor-operator/pkg/install"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UIMaintainer struct {
	Client client.Client
	ClusterInstance *operatorv1alpha1.Cluster
}

func NewUIMaintainer(cli client.Client, clusterInstance *operatorv1alpha1.Cluster) *UIMaintainer {
	return &UIMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var uiLabelKey = "app"
var uiLabelValue = "hwameistor-ui"
var defaultUIImageRegistry = "ghcr.m.daocloud.io"
var defaultUIImageRepository = "hwameistor/hwameistor-ui"
var defaultUIImageTag = install.DefaultHwameistorVersion
var uiContainerName = "hwameistor-ui"

var ui = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-ui",
		Labels: map[string]string{
			uiLabelKey: uiLabelValue,
		},
	},
	Spec: appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				uiLabelKey: uiLabelValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: "hwameistor-ui",
				Labels: map[string]string{
					uiLabelKey: uiLabelValue,
				},
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyAlways,
				Containers: []corev1.Container{
					{
						ImagePullPolicy: corev1.PullIfNotPresent,
						Name: uiContainerName,
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 8080,
								Protocol: corev1.ProtocolTCP,
								Name: "http",
							},
						},
					},
				},
			},
		},
	},
}

func SetUI(clusterInstance *operatorv1alpha1.Cluster) {
	ui.Namespace = clusterInstance.Spec.TargetNamespace
	replicas := getUIReplicasFromClusterInstance(clusterInstance)
	ui.Spec.Replicas = &replicas
	ui.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	for i, container := range ui.Spec.Template.Spec.Containers {
		if container.Name == uiContainerName {
			container.Image = getUIContainerImageStringFromClusterInstance(clusterInstance)
			if resources := clusterInstance.Spec.UI.UI.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		ui.Spec.Template.Spec.Containers[i] = container
	}
}

func getUIContainerImageStringFromClusterInstance(clusterInstance *operatorv1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.UI.UI.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getUIReplicasFromClusterInstance(clusterInstance *operatorv1alpha1.Cluster) int32 {
	return clusterInstance.Spec.UI.Replicas
}

func needOrNotToUpdateUI (cluster *operatorv1alpha1.Cluster, gottenUI appsv1.Deployment) (bool, *appsv1.Deployment) {
	uiToUpdate := gottenUI.DeepCopy()
	var needToUpdate bool

	for i, container := range uiToUpdate.Spec.Template.Spec.Containers {
		if container.Name == uiContainerName {
			wantedImage := getUIContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				uiToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	wantedReplicas := getUIReplicasFromClusterInstance(cluster)
	if *uiToUpdate.Spec.Replicas != wantedReplicas {
		uiToUpdate.Spec.Replicas = &wantedReplicas
		needToUpdate = true
	}

	return needToUpdate, uiToUpdate
}

func (m *UIMaintainer) Ensure() (*operatorv1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	SetUI(newClusterInstance)
	key := types.NamespacedName{
		Namespace: ui.Namespace,
		Name: ui.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &ui); errCreate != nil {
				log.Errorf("Create UI err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get UI err: %v", err)
			return newClusterInstance, err
		}
	}

	needToUpdate, uiToUpdate := needOrNotToUpdateUI(newClusterInstance, gotten)
	if needToUpdate {
		log.Infof("need to update ui")
		if err := m.Client.Update(context.TODO(), uiToUpdate); err != nil {
			log.Errorf("Update ui err: %v", err)
			return newClusterInstance, err
		}
	}

	return newClusterInstance, nil
}

func FulfillUISpec(clusterInstance *operatorv1alpha1.Cluster) *operatorv1alpha1.Cluster {
	if clusterInstance.Spec.UI == nil {
		clusterInstance.Spec.UI = &operatorv1alpha1.UISpec{}
	}
	if clusterInstance.Spec.UI.UI == nil {
		clusterInstance.Spec.UI.UI = &operatorv1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.UI.UI.Image == nil {
		clusterInstance.Spec.UI.UI.Image = &operatorv1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.UI.UI.Image.Registry == "" {
		clusterInstance.Spec.UI.UI.Image.Registry = defaultUIImageRegistry
	}
	if clusterInstance.Spec.UI.UI.Image.Repository == "" {
		clusterInstance.Spec.UI.UI.Image.Repository = defaultUIImageRepository
	}
	if clusterInstance.Spec.UI.UI.Image.Tag == "" {
		clusterInstance.Spec.UI.UI.Image.Tag = defaultUIImageTag
	}

	return clusterInstance
}