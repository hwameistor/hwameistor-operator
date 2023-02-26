package exporter

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

type ExporterMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewExporterMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *ExporterMaintainer {
	return &ExporterMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var exporterLabelSelectorKey = "app"
var exporterLabelSelectorValue = "hwameistor-exporter"
var defaultExporterReplicas = int32(1)
var defaultExporterImageRegistry = "ghcr.m.daocloud.io"
var defaultExporterImageRepository = "hwameistor/exporter"
var defaultExporterImageTag = install.DefaultHwameistorVersion

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
						Name: "exporter",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{
								Name: "exporter-apis",
								ContainerPort: 8080,
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
	exporter.OwnerReferences = append(exporter.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	exporter.Spec.Replicas = &clusterInstance.Spec.Exporter.Replicas
	exporter.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	for i, container := range exporter.Spec.Template.Spec.Containers {
		if container.Name == "exporter" {
			imageSpec := clusterInstance.Spec.Exporter.Collector.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
		}
		exporter.Spec.Template.Spec.Containers[i] = container
	}
}

func (m *ExporterMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	SetExporter(newClusterInstance)
	key := types.NamespacedName{
		Namespace: exporter.Namespace,
		Name: exporter.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &exporter); errCreate != nil {
				log.Errorf("Create Exporter err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get Exporter err: %v", err)
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

	if newClusterInstance.Status.Exporter == nil {
		newClusterInstance.Status.Exporter = &hwameistoriov1alpha1.ExporterStatus{
			Instances: &instancesStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.Exporter.Instances == nil {
			newClusterInstance.Status.Exporter.Instances = &instancesStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.Exporter.Instances, instancesStatus) {
				newClusterInstance.Status.Exporter.Instances = &instancesStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func FulfillExporterSpec (clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
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