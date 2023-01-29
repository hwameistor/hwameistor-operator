package metrics

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

type MetricsMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewMetricsMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *MetricsMaintainer {
	return &MetricsMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var metricsLabelSelectorKey = "app"
var metricsLabelSelectorValue = "hwameistor-metrics-collector"

var metrics = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-metrics-collector",
		Labels: map[string]string{
			metricsLabelSelectorKey: metricsLabelSelectorValue,
		},
	},
	Spec: appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				metricsLabelSelectorKey: metricsLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					metricsLabelSelectorKey: metricsLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "collector",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{
							{
								Name: "metrics-apis",
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

func SetMetrics(clusterInstance *hwameistoriov1alpha1.Cluster) {
	metrics.Namespace = clusterInstance.Spec.TargetNamespace
	metrics.OwnerReferences = append(metrics.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	metrics.Spec.Replicas = &clusterInstance.Spec.Metrics.Replicas
	metrics.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	for i, container := range metrics.Spec.Template.Spec.Containers {
		if container.Name == "collector" {
			imageSpec := clusterInstance.Spec.Metrics.Collector.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
		}
		metrics.Spec.Template.Spec.Containers[i] = container
	}
}

func (m *MetricsMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	SetMetrics(newClusterInstance)
	key := types.NamespacedName{
		Namespace: metrics.Namespace,
		Name: metrics.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &metrics); errCreate != nil {
				log.Errorf("Create Metrics Collector err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get Metrics Collector err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: metrics.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[metricsLabelSelectorKey] == metricsLabelSelectorValue {
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

	if newClusterInstance.Status.Metrics == nil {
		newClusterInstance.Status.Metrics = &hwameistoriov1alpha1.MetricsStatus{
			Instances: &instancesStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.Metrics.Instances == nil {
			newClusterInstance.Status.Metrics.Instances = &instancesStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.Metrics.Instances, instancesStatus) {
				newClusterInstance.Status.Metrics.Instances = &instancesStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}