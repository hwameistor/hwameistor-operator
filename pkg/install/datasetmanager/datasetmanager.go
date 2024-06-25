package datasetmanager

import (
	"context"
	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var dsmDeploymentLabelSelectorKey = "app"
var dsmDeploymentLabelSelectorValue = "hwameistor-dataset-manager"
var dsmContainerName = "dataset-manager"
var defaultDSMDeploymentImageRegistry = "ghcr.m.daocloud.io"
var defaultDSMDeploymentImageRepository = "hwameistor/dataset-manager"
var defaultDSMDeploymentImageTag = "v0.0.1"

type DataSetManagerMaintainer struct {
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *DataSetManagerMaintainer {
	return &DataSetManagerMaintainer{
		Client:          cli,
		ClusterInstance: clusterInstance,
	}
}

var dsmDeploymentTemplate = appsv1.Deployment{

	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-dataset-manager",
		Labels: map[string]string{
			"app": "hwameistor-dataset-manager",
		},
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "hwameistor-dataset-manager",
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "hwameistor-dataset-manager",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "dataset-manager",
						Args:            []string{"--v=4", "--leader-election=true"},
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
				RestartPolicy:                 corev1.RestartPolicyAlways,
				DNSPolicy:                     corev1.DNSClusterFirst,
				TerminationGracePeriodSeconds: int64Ptr(30),
			},
		},
		ProgressDeadlineSeconds: int32Ptr(600),
		RevisionHistoryLimit:    int32Ptr(10),
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
	},
}

func int32Ptr(i int32) *int32 { return &i }

func int64Ptr(i int64) *int64 { return &i }

func SetDSMDeployment(clusterInstance *hwameistoriov1alpha1.Cluster) *appsv1.Deployment {
	dsmDeploymentToCreate := dsmDeploymentTemplate.DeepCopy()
	dsmDeploymentToCreate.OwnerReferences = append(dsmDeploymentToCreate.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	dsmDeploymentToCreate.Namespace = clusterInstance.Spec.TargetNamespace
	dsmDeploymentToCreate.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	dsmDeploymentToCreate = setdsmDeploymentContainers(clusterInstance, dsmDeploymentToCreate)

	if clusterInstance.Status.DatasetDefaultPoolClass != "" {
		pool_env := corev1.EnvVar{
			Name:  "DEFAULT_POOL_CLASS",
			Value: clusterInstance.Status.DatasetDefaultPoolClass,
		}
		dsmDeploymentToCreate.Spec.Template.Spec.Containers[0].Env = append(dsmDeploymentToCreate.Spec.Template.Spec.Containers[0].Env, pool_env)
	}

	return dsmDeploymentToCreate

}

func setdsmDeploymentContainers(clusterInstance *hwameistoriov1alpha1.Cluster, dsmDeploymentToCreate *appsv1.Deployment) *appsv1.Deployment {
	for i, container := range dsmDeploymentToCreate.Spec.Template.Spec.Containers {
		if container.Name == dsmContainerName {
			container.Image = getDSMContainerRegistrarImageStringFromClusterInstance(clusterInstance)
		}
		dsmDeploymentToCreate.Spec.Template.Spec.Containers[i] = container
	}
	return dsmDeploymentToCreate
}

func getDSMContainerRegistrarImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.DataSetManager.DataSetManagerContainer.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func needOrNotToUpdateDSMDeployment(cluster *hwameistoriov1alpha1.Cluster, gotten appsv1.Deployment) (bool, *appsv1.Deployment) {
	ds := gotten.DeepCopy()
	var needToUpdate bool

	for i, container := range ds.Spec.Template.Spec.Containers {
		if container.Name == dsmContainerName {
			wantedImage := getDSMContainerRegistrarImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				ds.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	return needToUpdate, ds
}

func (m *DataSetManagerMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	dsmDeploymentToCreate := SetDSMDeployment(newClusterInstance)
	key := types.NamespacedName{
		Namespace: dsmDeploymentToCreate.Namespace,
		Name:      dsmDeploymentToCreate.Name,
	}
	var gottenDS appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gottenDS); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), dsmDeploymentToCreate); errCreate != nil {
				log.Errorf("Create DataSetManager Deployment err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get DataSetManager Deployment err: %v", err)
			return newClusterInstance, err
		}
	}
	needToUpdate, dsToUpdate := needOrNotToUpdateDSMDeployment(newClusterInstance, gottenDS)
	if needToUpdate {
		log.Infof("need to update DataSetManager Deployment")
		if err := m.Client.Update(context.TODO(), dsToUpdate); err != nil {
			log.Errorf("Update DataSetManager Deployment err: %v", err)
			return newClusterInstance, err
		}
	}

	return newClusterInstance, nil
}

func FulfillDataSetManagerSpec(clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.DataSetManager == nil {
		clusterInstance.Spec.DataSetManager = &hwameistoriov1alpha1.DataSetManagerSpec{}
	}

	if clusterInstance.Spec.DataSetManager.DataSetManagerContainer == nil {
		clusterInstance.Spec.DataSetManager.DataSetManagerContainer = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}

	if clusterInstance.Spec.DataSetManager.DataSetManagerContainer.Image == nil {
		clusterInstance.Spec.DataSetManager.DataSetManagerContainer.Image = &hwameistoriov1alpha1.ImageSpec{}
	}

	if clusterInstance.Spec.DataSetManager.DataSetManagerContainer.Image.Registry == "" {
		clusterInstance.Spec.DataSetManager.DataSetManagerContainer.Image.Registry = defaultDSMDeploymentImageRegistry
	}

	if clusterInstance.Spec.DataSetManager.DataSetManagerContainer.Image.Repository == "" {
		clusterInstance.Spec.DataSetManager.DataSetManagerContainer.Image.Repository = defaultDSMDeploymentImageRepository
	}
	if clusterInstance.Spec.DataSetManager.DataSetManagerContainer.Image.Tag == "" {
		clusterInstance.Spec.DataSetManager.DataSetManagerContainer.Image.Tag = defaultDSMDeploymentImageTag
	}
	return clusterInstance
}
