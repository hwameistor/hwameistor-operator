package exporter

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

type ExporterMaintainer struct {
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewExporterMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *ExporterMaintainer {
	return &ExporterMaintainer{
		Client:          cli,
		ClusterInstance: clusterInstance,
	}
}

var exporterLabelSelectorKey = "app"
var exporterLabelSelectorValue = "hwameistor-exporter"
var defaultExporterReplicas = int32(1)
var defaultExporterImageRegistry = "ghcr.m.daocloud.io"
var defaultExporterImageRepository = "hwameistor/exporter"
var defaultExporterImageTag = install.DefaultHwameistorVersion
var exporterContainerName = "exporter"

var exporter = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-exporter",
		Labels: map[string]string{
			exporterLabelSelectorKey: exporterLabelSelectorValue,
		},
	},
	Spec: appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				exporterLabelSelectorKey: exporterLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					exporterLabelSelectorKey: exporterLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            exporterContainerName,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{
								Name:          "metrics",
								ContainerPort: 80,
							},
						},
						Env: []corev1.EnvVar{
							{
								Name: "NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

func SetExporter(clusterInstance *hwameistoriov1alpha1.Cluster) {
	exporter.Namespace = clusterInstance.Spec.TargetNamespace
	exporter.OwnerReferences = append(exporter.OwnerReferences, *metav1.NewControllerRef(clusterInstance, schema.FromAPIVersionAndKind("hwameistor.io/v1alpha1", "Cluster")))
	replicas := getExporterReplicasFromClusterInstance(clusterInstance)
	exporter.Spec.Replicas = &replicas
	exporter.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	for i, container := range exporter.Spec.Template.Spec.Containers {
		if container.Name == exporterContainerName {
			container.Image = getExporterContainerImageStringFromClusterInstance(clusterInstance)
			if resources := clusterInstance.Spec.Exporter.Collector.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		exporter.Spec.Template.Spec.Containers[i] = container
	}
}

func getExporterContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.Exporter.Collector.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getExporterReplicasFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) int32 {
	return clusterInstance.Spec.Exporter.Replicas
}

func needOrNotToUpdateExporter(cluster *hwameistoriov1alpha1.Cluster, gottenExporter appsv1.Deployment) (bool, *appsv1.Deployment) {
	exporterToUpdate := gottenExporter.DeepCopy()
	var needToUpdate bool

	for i, container := range exporterToUpdate.Spec.Template.Spec.Containers {
		if container.Name == exporterContainerName {
			wantedImage := getExporterContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				exporterToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	wantedReplicas := getExporterReplicasFromClusterInstance(cluster)
	if *exporterToUpdate.Spec.Replicas != wantedReplicas {
		exporterToUpdate.Spec.Replicas = &wantedReplicas
		needToUpdate = true
	}

	return needToUpdate, exporterToUpdate
}

func (m *ExporterMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	SetExporter(newClusterInstance)
	key := types.NamespacedName{
		Namespace: exporter.Namespace,
		Name:      exporter.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			resourceCreate := exporter.DeepCopy()
			if errCreate := m.Client.Create(context.TODO(), resourceCreate); errCreate != nil {
				log.Errorf("Create Exporter err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get Exporter err: %v", err)
			return newClusterInstance, err
		}
	}

	needToUpdate, exporterToUpdate := needOrNotToUpdateExporter(newClusterInstance, gotten)
	if needToUpdate {
		log.Infof("need to update exporter")
		if err := m.Client.Update(context.TODO(), exporterToUpdate); err != nil {
			log.Errorf("Update exporter err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: exporter.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[exporterLabelSelectorKey] == exporterLabelSelectorValue {
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

	if newClusterInstance.Status.ComponentStatus.Exporter == nil {
		newClusterInstance.Status.ComponentStatus.Exporter = &hwameistoriov1alpha1.ExporterStatus{
			Instances: &instancesStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.Exporter.Instances == nil {
			newClusterInstance.Status.ComponentStatus.Exporter.Instances = &instancesStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.Exporter.Instances, instancesStatus) {
				newClusterInstance.Status.ComponentStatus.Exporter.Instances = &instancesStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func (m *ExporterMaintainer) Uninstall() error {
	key := types.NamespacedName{
		Namespace: m.ClusterInstance.Spec.TargetNamespace,
		Name:      exporter.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			log.Errorf("get Exporter err: %v", err)
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
	log.Errorf("Exporter Owner is not %s, can't delete ", m.ClusterInstance.Name)
	return nil
}

func FulfillExporterSpec(clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.Exporter == nil {
		clusterInstance.Spec.Exporter = &hwameistoriov1alpha1.ExporterSpec{}
	}
	if clusterInstance.Spec.Exporter.Replicas == 0 {
		clusterInstance.Spec.Exporter.Replicas = defaultExporterReplicas
	}
	if clusterInstance.Spec.Exporter.Collector == nil {
		clusterInstance.Spec.Exporter.Collector = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.Exporter.Collector.Image == nil {
		clusterInstance.Spec.Exporter.Collector.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.Exporter.Collector.Image.Registry == "" {
		clusterInstance.Spec.Exporter.Collector.Image.Registry = defaultExporterImageRegistry
	}
	if clusterInstance.Spec.Exporter.Collector.Image.Repository == "" {
		clusterInstance.Spec.Exporter.Collector.Image.Repository = defaultExporterImageRepository
	}
	if clusterInstance.Spec.Exporter.Collector.Image.Tag == "" {
		clusterInstance.Spec.Exporter.Collector.Image.Tag = defaultExporterImageTag
	}

	return clusterInstance
}
