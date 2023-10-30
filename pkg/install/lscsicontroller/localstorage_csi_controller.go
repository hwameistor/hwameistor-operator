package lscsicontroller

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
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LSCSIMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *LSCSIMaintainer {
	return &LSCSIMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var lsCSIControllerLabelSelectorKey = "app"
var lsCSIControllerLabelSelectorValue =  "hwameistor-local-storage-csi-controller"
var defaultKubeletRootDir = "/var/lib/kubelet"
var defaultLSCSIControllerReplicas = int32(1)
var defaultLSCSIProvisionerRegistry = "k8s-gcr.m.daocloud.io"
var defaultLSCSIProvisionerRepository = "sig-storage/csi-provisioner"
var defaultLSCSIProvisionerTag = "v2.0.3"
var defaultLSCSIAttacherRegistry = "k8s-gcr.m.daocloud.io"
var defaultLSCSIAttacherRepository = "sig-storage/csi-attacher"
var defaultLSCSIAttacherTag = "v3.0.1"
var defaultLSCSIMonitorRegistry = "k8s-gcr.m.daocloud.io"
var defaultLSCSIMonitorRepository = "sig-storage/csi-external-health-monitor-controller"
var defaultLSCSIMonitorTag = "v0.8.0"
var defaultLSCSIResizerRegistry = "k8s-gcr.m.daocloud.io"
var defaultLSCSIResizerRepository = "sig-storage/csi-resizer"
var defaultLSCSIResizerTag = "v1.0.1"
var provisionerContainerName = "provisioner"
var attacherContainerName = "attacher"
var monitorContainerName = "monitor"
var resizerContainerName = "resizer"
var snapshotControllerContainerName = "snapshot-controller"
var snapshotterContainerName = "csi-snapshotter"

var lsCSIController = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-local-storage-csi-controller",
	},
	Spec: appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				lsCSIControllerLabelSelectorKey: lsCSIControllerLabelSelectorValue,
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					lsCSIControllerLabelSelectorKey: lsCSIControllerLabelSelectorValue,
				},
			},
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key: "app",
											Operator: "In",
											Values: []string{"hwameistor-local-storage"},
										},
									},
								},
								TopologyKey: "topology.lvm.hwameistor.io/node",
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: provisionerContainerName,
						Args: []string{
							"--v=5",
							"--csi-address=$(CSI_ADDRESS)",
							"--leader-election=true",
							"--feature-gates=Topology=true",
							"--strict-topology",
							"--extra-create-metadata=true",
						},
						Env: []corev1.EnvVar{
							{
								Name: "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						ImagePullPolicy: "IfNotPresent",
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "socket-dir",
								MountPath: "/csi",
							},
						},
					},
					{
						Name: attacherContainerName,
						Args: []string{
							"--v=5",
							"--csi-address=$(CSI_ADDRESS)",
							"--leader-election=true",
							"--timeout=120s",
						},
						Env: []corev1.EnvVar{
							{
								Name: "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						ImagePullPolicy: "IfNotPresent",
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "socket-dir",
								MountPath: "/csi",
							},
						},
					},
					{
						Name: monitorContainerName,
						Args: []string{
							"--v=5",
							"--csi-address=$(CSI_ADDRESS)",
							"--leader-election",
							"--http-endpoint=:8080",
						},
						Env: []corev1.EnvVar{
							{
								Name: "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "socket-dir",
								MountPath: "/csi",
							},
						},
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 8080,
								Name: "http-endpoint",
								Protocol: corev1.ProtocolTCP,
							},
						},
						LivenessProbe: &corev1.Probe{
							FailureThreshold: 1,
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz/leader-election",
									Port: intstr.IntOrString{
										Type: intstr.String,
										StrVal: "http-endpoint",
									},
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds: 10,
							PeriodSeconds: 20,
						},
					},
					{
						Name: resizerContainerName,
						Args: []string{
							"--v=5",
							"--csi-address=$(CSI_ADDRESS)",
							"--leader-election=true",
						},
						Env: []corev1.EnvVar{
							{
								Name: "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						ImagePullPolicy: "IfNotPresent",
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "socket-dir",
								MountPath: "/csi",
							},
						},
					},
					{
						Name: snapshotControllerContainerName,
						Args: []string{
							"--v=5",
							"--leader-election=true",
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
					{
						Name: snapshotterContainerName,
						Args: []string{
							"--v=5",
							"--leader-election=true",
							"--csi-address=$(CSI_ADDRESS)",
							"--leader-election",
						},
						Env: []corev1.EnvVar{
							{
								Name: "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								MountPath: "/csi",
								Name: "socket-dir",
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
				},
				DNSPolicy: corev1.DNSClusterFirst,
				RestartPolicy: corev1.RestartPolicyAlways,
				TerminationGracePeriodSeconds: &install.TerminationGracePeriodSeconds30s,
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

func SetLSCSIController(clusterInstance *hwameistoriov1alpha1.Cluster) {
	lsCSIController.Namespace = clusterInstance.Spec.TargetNamespace
	lsCSIController.OwnerReferences = append(lsCSIController.OwnerReferences, *metav1.NewControllerRef(clusterInstance, clusterInstance.GroupVersionKind()))
	replicas := getReplicasFromClusterInstance(clusterInstance)
	lsCSIController.Spec.Replicas = &replicas
	lsCSIController.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	// lsCSIController.Spec.Template.Spec.PriorityClassName = clusterInstance.Spec.LocalStorage.Common.PriorityClassName
	socketDirVolume := corev1.Volume{
		Name: "socket-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: clusterInstance.Spec.LocalStorage.KubeletRootDir + "/plugins/lvm.hwameistor.io",
				Type: &install.HostPathDirectoryOrCreate,
			},
		},
	}
	lsCSIController.Spec.Template.Spec.Volumes = append(lsCSIController.Spec.Template.Spec.Volumes, socketDirVolume)
	setLSCSIControllerContainers(clusterInstance)
}

func setLSCSIControllerContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
	for i, container := range lsCSIController.Spec.Template.Spec.Containers {
		if container.Name == provisionerContainerName {
			container.Image = getProvisionerContainerImageStringFromClusterInstance(clusterInstance)
			// container.Resources = *clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Resources
			if resources := clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		if container.Name == attacherContainerName {
			container.Image = getAttacherContainerImageStringFromClusterInstance(clusterInstance)
			// container.Resources = *clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Resources
			if resources := clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		if container.Name == resizerContainerName {
			container.Image = getResizerContainerImageStringFromClusterInstance(clusterInstance)
			// container.Resources = *clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Resources
			if resources := clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		if container.Name == monitorContainerName {
			container.Image = getMonitorContainerImageStringFromClusterInstance(clusterInstance)
			if resources := clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		if container.Name == snapshotControllerContainerName {
			container.Image = getSnapshotControllerImageStringFromClusterInstance(clusterInstance)
			if resources := clusterInstance.Spec.LocalStorage.CSI.Controller.SnapshotController.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		if container.Name == snapshotterContainerName {
			container.Image = getSnapshotterImageStringFromClusterInstance(clusterInstance)
			if resources := clusterInstance.Spec.LocalStorage.CSI.Controller.Snapshotter.Resources; resources != nil {
				container.Resources = *resources
			}
		}
		lsCSIController.Spec.Template.Spec.Containers[i] = container
	}

	if clusterInstance.Spec.LocalStorage.Snapshot.Disable {
		containers := make([]corev1.Container, 0)
		for _, container := range lsCSIController.Spec.Template.Spec.Containers {
			if container.Name == snapshotControllerContainerName || container.Name == snapshotterContainerName {
				continue
			}
			containers = append(containers, container)
		}
		lsCSIController.Spec.Template.Spec.Containers = containers
	}
}

func getProvisionerContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getAttacherContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getResizerContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getMonitorContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getSnapshotControllerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalStorage.CSI.Controller.SnapshotController.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getSnapshotterImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.LocalStorage.CSI.Controller.Snapshotter.Image
	return imageSpec.Registry + "/" +imageSpec.Repository + ":" + imageSpec.Tag
}

func getReplicasFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) int32 {
	return clusterInstance.Spec.LocalStorage.CSI.Controller.Replicas
}

func needOrNotToUpdateExporter (cluster *hwameistoriov1alpha1.Cluster, gottenCSIController appsv1.Deployment) (bool, *appsv1.Deployment) {
	lsCSIControllerToUpdate := gottenCSIController.DeepCopy()
	var needToUpdate bool

	for i, container := range lsCSIControllerToUpdate.Spec.Template.Spec.Containers {
		if container.Name == provisionerContainerName {
			wantedImage := getProvisionerContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				lsCSIControllerToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
		if container.Name == attacherContainerName {
			wantedImage := getAttacherContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				lsCSIControllerToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
		if container.Name == resizerContainerName {
			wantedImage := getResizerContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				lsCSIControllerToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
		if container.Name == monitorContainerName {
			wantedImage := getMonitorContainerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				lsCSIControllerToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
		if container.Name == snapshotControllerContainerName {
			wantedImage := getSnapshotControllerImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				container.Image = wantedImage
				lsCSIControllerToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
		if container.Name == snapshotterContainerName {
			wantedImage := getSnapshotterImageStringFromClusterInstance(cluster)
			if container.Image != wantedImage {
				lsCSIControllerToUpdate.Spec.Template.Spec.Containers[i] = container
				needToUpdate = true
			}
		}
	}

	wantedReplicas := getReplicasFromClusterInstance(cluster)
	if *lsCSIControllerToUpdate.Spec.Replicas != wantedReplicas {
		lsCSIControllerToUpdate.Spec.Replicas = &wantedReplicas
		needToUpdate = true
	}

	return needToUpdate, lsCSIControllerToUpdate
}

func (m *LSCSIMaintainer) Ensure() (*hwameistoriov1alpha1.Cluster, error) {
	newClusterInstance := m.ClusterInstance.DeepCopy()
	SetLSCSIController(newClusterInstance)
	key := types.NamespacedName{
		Namespace: lsCSIController.Namespace,
		Name: lsCSIController.Name,
	}
	var gottenCSIController appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gottenCSIController); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &lsCSIController); errCreate != nil {
				log.Errorf("Create LS CSIController err: %v", errCreate)
				return newClusterInstance, errCreate
			}
			return newClusterInstance, nil
		} else {
			log.Errorf("Get LS CSIController err: %v", err)
			return newClusterInstance, err
		}
	}

	needToUpdate, csiControllerToUpdate := needOrNotToUpdateExporter(newClusterInstance, gottenCSIController)
	if needToUpdate {
		log.Infof("need to update ls csiController")
		if err := m.Client.Update(context.TODO(), csiControllerToUpdate); err != nil {
			log.Errorf("Update ls csiController err: %v", err)
			return newClusterInstance, err
		}
	}

	var podList corev1.PodList
	if err := m.Client.List(context.TODO(), &podList, &client.ListOptions{Namespace: lsCSIController.Namespace}); err != nil {
		log.Errorf("List pods err: %v", err)
		return newClusterInstance, err
	}

	var podsManaged []corev1.Pod
	for _, pod := range podList.Items {
		if pod.Labels[lsCSIControllerLabelSelectorKey] == lsCSIControllerLabelSelectorValue {
			podsManaged = append(podsManaged, pod)
		}
	}

	if len(podsManaged) > int(gottenCSIController.Status.Replicas) {
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

	csiDeployStatus := hwameistoriov1alpha1.DeployStatus{
		Pods: podsStatus,
		DesiredPodCount: gottenCSIController.Status.Replicas,
		AvailablePodCount: gottenCSIController.Status.AvailableReplicas,
		WorkloadType: "Deployment",
		WorkloadName: gottenCSIController.Name,
	}

	if newClusterInstance.Status.ComponentStatus.LocalStorage == nil {
		newClusterInstance.Status.ComponentStatus.LocalStorage = &hwameistoriov1alpha1.LocalStorageStatus{
			CSI: &csiDeployStatus,
		}
		return newClusterInstance, nil
	} else {
		if newClusterInstance.Status.ComponentStatus.LocalStorage.CSI == nil {
			newClusterInstance.Status.ComponentStatus.LocalStorage.CSI = &csiDeployStatus
			return newClusterInstance, nil
		} else {
			if !reflect.DeepEqual(newClusterInstance.Status.ComponentStatus.LocalStorage.CSI, csiDeployStatus) {
				newClusterInstance.Status.ComponentStatus.LocalStorage.CSI = &csiDeployStatus
				return newClusterInstance, nil
			}
		}
	}
	return newClusterInstance, nil
}

func FulfillLSCSISpec (clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.LocalStorage == nil {
		clusterInstance.Spec.LocalStorage = &hwameistoriov1alpha1.LocalStorageSpec{}
	}
	if clusterInstance.Spec.LocalStorage.KubeletRootDir == "" {
		clusterInstance.Spec.LocalStorage.KubeletRootDir = defaultKubeletRootDir
	}
	if clusterInstance.Spec.LocalStorage.CSI == nil {
		clusterInstance.Spec.LocalStorage.CSI = &hwameistoriov1alpha1.CSISpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller == nil {
		clusterInstance.Spec.LocalStorage.CSI.Controller = &hwameistoriov1alpha1.CSIControllerSpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Replicas == 0 {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Replicas = defaultLSCSIControllerReplicas
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner == nil {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Image == nil {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Image.Registry == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Image.Registry = defaultLSCSIProvisionerRegistry
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Image.Repository == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Image.Repository = defaultLSCSIProvisionerRepository
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Image.Tag == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Provisioner.Image.Tag = defaultLSCSIProvisionerTag
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher == nil {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Image == nil {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Image.Registry == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Image.Registry = defaultLSCSIAttacherRegistry
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Image.Repository == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Image.Repository = defaultLSCSIAttacherRepository
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Image.Tag == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Attacher.Image.Tag = defaultLSCSIAttacherTag
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor == nil {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor.Image == nil {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor.Image.Registry == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor.Image.Registry = defaultLSCSIMonitorRegistry
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor.Image.Repository == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor.Image.Repository = defaultLSCSIMonitorRepository
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor.Image.Tag == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Monitor.Image.Tag = defaultLSCSIMonitorTag
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer == nil {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer = &hwameistoriov1alpha1.ContainerCommonSpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Image == nil {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Image = &hwameistoriov1alpha1.ImageSpec{}
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Image.Registry == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Image.Registry = defaultLSCSIResizerRegistry
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Image.Repository == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Image.Repository = defaultLSCSIResizerRepository
	}
	if clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Image.Tag == "" {
		clusterInstance.Spec.LocalStorage.CSI.Controller.Resizer.Image.Tag = defaultLSCSIResizerTag
	}

	return clusterInstance
}