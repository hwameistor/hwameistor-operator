package admissioncontroller

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

type AdmissionControllerMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewAdmissionControllerMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *AdmissionControllerMaintainer {
	return &AdmissionControllerMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var replicas = int32(1)
var admissionControllerLabelSelectorKey = "app"
var admissionControllerLabelSelectorValue = "hwameistor-admission-controller"
var defaultAdmissionControllerImageRegistry = "ghcr.m.daocloud.io"
var defaultAdmissionControllerImageRepository = "hwameistor/admission"
var defaultAdmissionControllerImageTag = install.DefaultHwameistorVersion
var admissionControllerContainerName = "server"

var admissionController = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-admission-controller",
		Labels: map[string]string{
			admissionControllerLabelSelectorKey: admissionControllerLabelSelectorValue,
		},
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &replicas,
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				admissionControllerLabelSelectorKey: admissionControllerLabelSelectorValue,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					admissionControllerLabelSelectorKey: admissionControllerLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: admissionControllerContainerName,
						Args: []string{
							"--cert-dir=/etc/webhook/certs",
							"--tls-private-key-file=tls.key",
							"--tls-cert-file=tls.crt",
						},
						ImagePullPolicy: "IfNotPresent",
						Env: []corev1.EnvVar{
							{
								Name: "MUTATE_CONFIG",
								Value: "hwameistor-admission-mutate",
							},
							{
								Name: "WEBHOOK_SERVICE",
								Value: "hwameistor-admission-controller",
							},
							{
								Name: "MUTATE_PATH",
								Value: "/mutate",
							},
						},
						Ports: []corev1.ContainerPort{
							{
								Name: "admission-api",
								ContainerPort: 18443,
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "admission-controller-tls-certs",
								MountPath: "/etc/webhook/certs",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "admission-controller-tls-certs",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		},
	},
}

func SetAdmissionController(clusterInstance *hwameistoriov1alpha1.Cluster) {
	admissionController.Namespace = clusterInstance.Spec.TargetNamespace
	admissionController.OwnerReferences = append(admissionController.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	replicas := getAdmissionControllerReplicasFromClusterInstance(clusterInstance)
	admissionController.Spec.Replicas = &replicas
	admissionController.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	setAdmissionControllerContainers(clusterInstance)
}

func setAdmissionControllerContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
	for i, container := range admissionController.Spec.Template.Spec.Containers {
		if container.Name == admissionControllerContainerName {
			// container.Resources = *clusterInstance.Spec.AdmissionController.Controller.Resources
			imageSpec := clusterInstance.Spec.AdmissionController.Controller.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
			container.Env = append(container.Env, []corev1.EnvVar{
				{
					Name: "WEBHOOK_NAMESPACE",
					Value: clusterInstance.Spec.TargetNamespace,
				},
				{
					Name: "FAILURE_POLICY",
					Value: clusterInstance.Spec.AdmissionController.FailurePolicy,
				},
			}...)
		}
		admissionController.Spec.Template.Spec.Containers[i] = container
	}
}

func getAdmissionControllerContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.AdmissionController.Controller.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getAdmissionControllerReplicasFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) int32 {
	return clusterInstance.Spec.AdmissionController.Replicas
}

func needOrNotToUpdateAdmissionController (cluster *hwameistoriov1alpha1.Cluster, gottenAdmissionController appsv1.Deployment) (bool, *appsv1.Deployment) {
	admissionControllerToUpdate := gottenAdmissionController.DeepCopy()
	var needToUpdate bool

	for i, container := range admissionControllerToUpdate.Spec.Template.Spec.Containers {
		if container.Name == admissionControllerContainerName {
			wantedImage := getAdmissionControllerContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				admissionControllerToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	wantedReplicas := getAdmissionControllerReplicasFromClusterInstance(cluster)
	if *admissionControllerToUpdate.Spec.Replicas != wantedReplicas {
		admissionControllerToUpdate.Spec.Replicas = &wantedReplicas
		needToUpdate = true
	}

	return needToUpdate, admissionControllerToUpdate
}

func (m *AdmissionControllerMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	SetAdmissionController(newClusterInstance)
	key := types.NamespacedName{
		Namespace: admissionController.Namespace,
		Name: admissionController.Name,
	}
	var gottenAdmissionController appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gottenAdmissionController); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &admissionController); errCreate != nil {
				log.Errorf("Create AdmissionController err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get AdmissionController err: %v", err)
			return newClusterInstance, err
		}
	}

	needToUpdate, admissionControllerToUpdate := needOrNotToUpdateAdmissionController(newClusterInstance, gottenAdmissionController)
	if needToUpdate {
		log.Infof("need to update admissionController")
		if err := m.Client.Update(context.TODO(), admissionControllerToUpdate); err != nil {
			log.Errorf("Update admissionController err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: admissionController.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[admissionControllerLabelSelectorKey] == admissionControllerLabelSelectorValue {
			podsManaged = append(podsManaged, pod)
		}
	}

	if len(podsManaged) > int(gottenAdmissionController.Status.Replicas) {
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
		DesiredPodCount: gottenAdmissionController.Status.Replicas,
		AvailablePodCount: gottenAdmissionController.Status.AvailableReplicas,
		WorkloadType: "Deployment",
		WorkloadName: gottenAdmissionController.Name,
	}

	if newClusterInstance.Status.ComponentStatus.AdmissionController == nil {
		newClusterInstance.Status.ComponentStatus.AdmissionController = &hwameistoriov1alpha1.AdmissionControllerStatus{
			Instances: &instancesStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.AdmissionController.Instances == nil {
			newClusterInstance.Status.ComponentStatus.AdmissionController.Instances = &instancesStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.AdmissionController.Instances, instancesStatus) {
				newClusterInstance.Status.ComponentStatus.AdmissionController.Instances = &instancesStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func FulfillAdmissionControllerSpec (clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.AdmissionController == nil {
		clusterInstance.Spec.AdmissionController = &hwameistoriov1alpha1.AdmissionControllerSpec{}
	}
	if clusterInstance.Spec.AdmissionController.Controller == nil {
		clusterInstance.Spec.AdmissionController.Controller = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.AdmissionController.Controller.Image == nil {
		clusterInstance.Spec.AdmissionController.Controller.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.AdmissionController.Controller.Image.Registry == "" {
		clusterInstance.Spec.AdmissionController.Controller.Image.Registry = defaultAdmissionControllerImageRegistry
	}
	if clusterInstance.Spec.AdmissionController.Controller.Image.Repository == "" {
		clusterInstance.Spec.AdmissionController.Controller.Image.Repository = defaultAdmissionControllerImageRepository
	}
	if clusterInstance.Spec.AdmissionController.Controller.Image.Tag == "" {
		clusterInstance.Spec.AdmissionController.Controller.Image.Tag = defaultAdmissionControllerImageTag
	}

	return clusterInstance
}