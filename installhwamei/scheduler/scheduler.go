package scheduler

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

type SchedulerMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewSchedulerMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *SchedulerMaintainer {
	return &SchedulerMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var schedulerLabelSelectorKey = "app"
var schedulerLabelSelectorValue = "hwameistor-scheduler"

var schedulerDeploy = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-scheduler",
	},
	Spec: appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				schedulerLabelSelectorKey: schedulerLabelSelectorValue,
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "hwameistor-scheduler",
				},
			},
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
							{
								Weight: 1,
								Preference: corev1.NodeSelectorTerm{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key: "node-role.kubernetes.io/master",
											Operator: corev1.NodeSelectorOpExists,
										},
										{
											Key: "node-role.kubernetes.io/control-plane",
											Operator: corev1.NodeSelectorOpExists,
										},
									},
								},
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: "hwameistor-kube-scheduler",
						Args: []string{
							"-v=2",
							"--bind-address=0.0.0.0",
							"--leader-elect=false",
							"--leader-elect-resource-name=hwameistor-scheduler",
							"--config=/etc/hwameistor/hwameistor-scheduler-config.yaml",
						},
						ImagePullPolicy: "IfNotPresent",
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "hwameistor-scheduler-config",
								MountPath: "/etc/hwameistor/",
								ReadOnly: true,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "hwameistor-scheduler-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								Items: []corev1.KeyToPath{
									{
										Key: "hwameistor-scheduler-config.yaml",
										Path: "hwameistor-scheduler-config.yaml",
									},
								},
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "hwameistor-scheduler-config",
								},
							},
						},
					},
				},
				Tolerations: []corev1.Toleration{
					{
						Key: "CriticalAddonsOnly",
						Operator: corev1.TolerationOpExists,
					},
					{
						Key: "node.kubernetes.io/not-ready",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key: "node-role.kubernetes.io/master",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key: "node-role.kubernetes.io/control-plane",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key: "node.cloudprovider.kubernetes.io/uninitialized",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
			},
		},
	},
}

func SetScheduler(clusterInstance *hwameistoriov1alpha1.Cluster) {
	schedulerDeploy.Namespace = clusterInstance.Spec.TargetNamespace
	schedulerDeploy.OwnerReferences = append(schedulerDeploy.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	schedulerDeploy.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	replicas := clusterInstance.Spec.Scheduler.Replicas
	schedulerDeploy.Spec.Replicas = &replicas
	setSchedulerContainers(clusterInstance)
}

func setSchedulerContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
	for i, container := range schedulerDeploy.Spec.Template.Spec.Containers {
		if container.Name == "hwameistor-kube-scheduler" {
			// container.Resources = *clusterInstance.Spec.Scheduler.Scheduler.Resources
			imageSpec := clusterInstance.Spec.Scheduler.Scheduler.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
		}
		schedulerDeploy.Spec.Template.Spec.Containers[i] = container
	}
}

func (m *SchedulerMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance
	SetScheduler(newClusterInstance)
	key := types.NamespacedName{
		Namespace: schedulerDeploy.Namespace,
		Name: schedulerDeploy.Name,
	}
	var gotten appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &schedulerDeploy); errCreate != nil {
				log.Errorf("Create Scheduler err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get Scheduler err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: schedulerDeploy.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[schedulerLabelSelectorKey] == schedulerLabelSelectorValue {
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
	}

	if newClusterInstance.Status.Scheduler == nil {
		newClusterInstance.Status.Scheduler = &hwameistoriov1alpha1.SchedulerStatus{
			Instances: &instancesStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.Scheduler.Instances == nil {
			newClusterInstance.Status.Scheduler.Instances = &instancesStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.Scheduler.Instances, instancesStatus) {
				newClusterInstance.Status.Scheduler.Instances = &instancesStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}