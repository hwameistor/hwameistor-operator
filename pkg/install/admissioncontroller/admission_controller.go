package admissioncontroller

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"reflect"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"github.com/hwameistor/hwameistor-operator/pkg/install"
	"github.com/hwameistor/hwameistor/pkg/utils/certmanager"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AdmissionControllerMaintainer struct {
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
	watchChan       chan time.Time
	lock            sync.Mutex
}

var DefaultBackoff = wait.Backoff{
	Duration: 5 * time.Second,
	Factor:   2.0,
	Jitter:   0.1,
	Steps:    5,
	Cap:      5 * time.Minute,
}

var once sync.Once
var maintainer *AdmissionControllerMaintainer

func NewAdmissionControllerMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *AdmissionControllerMaintainer {
	once.Do(func() {
		maintainer = &AdmissionControllerMaintainer{
			Client:          cli,
			ClusterInstance: clusterInstance,
			watchChan:       make(chan time.Time, 1),
		}
		go maintainer.StartCertificateWatcher(context.TODO())
	})
	maintainer.ClusterInstance = clusterInstance
	return maintainer
}

var replicas = int32(1)
var admissionControllerLabelSelectorKey = "app"
var admissionControllerLabelSelectorValue = "hwameistor-admission-controller"
var defaultAdmissionControllerImageRegistry = "ghcr.m.daocloud.io"
var defaultAdmissionControllerImageRepository = "hwameistor/admission"
var defaultAdmissionControllerImageTag = install.DefaultHwameistorVersion
var admissionControllerContainerName = "server"
var defaultEffectiveYear = 10
var organization = "hwameistor.io"
var webhookService = "hwameistor-admission-controller"

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
								Name:  "MUTATE_CONFIG",
								Value: "hwameistor-admission-mutate",
							},
							{
								Name:  "WEBHOOK_SERVICE",
								Value: "hwameistor-admission-controller",
							},
							{
								Name:  "MUTATE_PATH",
								Value: "/mutate",
							},
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "admission-api",
								ContainerPort: 18443,
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "admission-controller-tls-certs",
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

func SetAdmissionController(clusterInstance *hwameistoriov1alpha1.Cluster) *appsv1.Deployment {

	resourceCreate := admissionController.DeepCopy()
	resourceCreate.Namespace = clusterInstance.Spec.TargetNamespace
	resourceCreate.OwnerReferences = append(resourceCreate.OwnerReferences, *metav1.NewControllerRef(clusterInstance, schema.FromAPIVersionAndKind("hwameistor.io/v1alpha1", "Cluster")))
	replicas := getAdmissionControllerReplicasFromClusterInstance(clusterInstance)
	resourceCreate.Spec.Replicas = &replicas
	resourceCreate.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	return setAdmissionControllerContainers(clusterInstance, resourceCreate)
}

func setAdmissionControllerContainers(clusterInstance *hwameistoriov1alpha1.Cluster, resourceCreate *appsv1.Deployment) *appsv1.Deployment {
	for i, container := range resourceCreate.Spec.Template.Spec.Containers {
		if container.Name == admissionControllerContainerName {
			// container.Resources = *clusterInstance.Spec.AdmissionController.Controller.Resources
			if resources := clusterInstance.Spec.AdmissionController.Controller.Resources; resources != nil {
				container.Resources = *resources
			}
			imageSpec := clusterInstance.Spec.AdmissionController.Controller.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag

			dataLoadManager := clusterInstance.Spec.DataLoadManager.DataLoadManagerContainer.Image
			dataloadImageValue := dataLoadManager.Registry + "/" + "hwameistor/dataload-init" + ":" + dataLoadManager.Tag

			container.Env = append(container.Env, []corev1.EnvVar{
				{
					Name:  "WEBHOOK_NAMESPACE",
					Value: clusterInstance.Spec.TargetNamespace,
				},
				{
					Name:  "FAILURE_POLICY",
					Value: clusterInstance.Spec.AdmissionController.FailurePolicy,
				},
				{
					Name:  "DATALOADER_IMAGE",
					Value: dataloadImageValue,
				},
			}...)
		}
		resourceCreate.Spec.Template.Spec.Containers[i] = container
	}

	return resourceCreate
}

func getAdmissionControllerContainerImageStringFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) string {
	imageSpec := clusterInstance.Spec.AdmissionController.Controller.Image
	return imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
}

func getAdmissionControllerReplicasFromClusterInstance(clusterInstance *hwameistoriov1alpha1.Cluster) int32 {
	return clusterInstance.Spec.AdmissionController.Replicas
}

func needOrNotToUpdateAdmissionController(cluster *hwameistoriov1alpha1.Cluster, gottenAdmissionController appsv1.Deployment) (bool, *appsv1.Deployment) {
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
			for k, envVar := range container.Env {
				if envVar.Name == "FAILURE_POLICY" {
					if envVar.Value != cluster.Spec.AdmissionController.FailurePolicy {
						admissionControllerToUpdate.Spec.Template.Spec.Containers[i].Env[k].Value = cluster.Spec.AdmissionController.FailurePolicy
						needToUpdate = true
					}
				}
				if envVar.Name == "DATALOADER_IMAGE" {
					dataLoadManager := cluster.Spec.DataLoadManager.DataLoadManagerContainer.Image
					dataloadImageValue := dataLoadManager.Registry + "/" + "hwameistor/dataload-init" + ":" + dataLoadManager.Tag
					if envVar.Value != dataloadImageValue {
						admissionControllerToUpdate.Spec.Template.Spec.Containers[i].Env[k].Value = dataloadImageValue
						needToUpdate = true
					}
				}
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
	resourceCreate := SetAdmissionController(newClusterInstance)

	// ensure admission controller CA is generated
	if err := m.ensureAdmissionCA(); err != nil {
		log.WithError(err).Error("failed to ensure admission controller CA")
		return newClusterInstance, err
	}
	key := types.NamespacedName{
		Namespace: resourceCreate.Namespace,
		Name:      resourceCreate.Name,
	}
	var gottenAdmissionController appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gottenAdmissionController); err != nil {
		if apierrors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), resourceCreate); errCreate != nil {
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
			Name:   pod.Name,
			Node:   pod.Spec.NodeName,
			Status: string(pod.Status.Phase),
		}
		podsStatus = append(podsStatus, podStatus)
	}

	instancesStatus := hwameistoriov1alpha1.DeployStatus{
		Pods:              podsStatus,
		DesiredPodCount:   gottenAdmissionController.Status.Replicas,
		AvailablePodCount: gottenAdmissionController.Status.AvailableReplicas,
		WorkloadType:      "Deployment",
		WorkloadName:      gottenAdmissionController.Name,
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

func (m *AdmissionControllerMaintainer) Uninstall() error {
	key := types.NamespacedName{
		Namespace: m.ClusterInstance.Spec.TargetNamespace,
		Name:      admissionController.Name,
	}
	var gottenAdmissionController appsv1.Deployment
	if err := m.Client.Get(context.TODO(), key, &gottenAdmissionController); err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err)
			return nil
		} else {
			log.Errorf("Get AdmissionController err: %v", err)
			return err
		}
	} else {
		for _, reference := range gottenAdmissionController.OwnerReferences {
			if reference.Name == m.ClusterInstance.Name {
				if err = m.Client.Delete(context.TODO(), &gottenAdmissionController); err != nil {
					return err
				} else {
					return nil
				}
			}
		}
	}

	log.Errorf("AdmissionController Owner is not %s, can't delete. ", m.ClusterInstance.Name)
	return nil
}

// ensure hwameistor-admission-ca is generated before the admission controller running
func (m *AdmissionControllerMaintainer) ensureAdmissionCA() error {
	log.Info("ensuring hwameistor-admission-ca secret...")
	m.lock.Lock()
	defer m.lock.Unlock()

	// skip generation if the ca already exists
	secret := corev1.Secret{}
	secret.Name = "hwameistor-admission-ca"
	secret.Namespace = m.ClusterInstance.Spec.TargetNamespace

	secretExist := false
	if err := m.Client.Get(context.Background(), client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, &secret); err != nil {
		if !apierrors.IsNotFound(err) {
			log.WithError(err).Error("failed to get admission ca secret")
			return err
		}
	} else {
		secretExist = true
		if secret.Data != nil && secret.Data[corev1.TLSPrivateKeyKey] != nil && secret.Data[corev1.TLSCertKey] != nil {
			log.Info("admission ca secret found, skip generation")
			return nil
		}
	}

	// generate self-signed certs
	var dnsNames = []string{webhookService, webhookService + "." + m.ClusterInstance.Spec.TargetNamespace, webhookService + "." + m.ClusterInstance.Spec.TargetNamespace + "." + "svc"}
	var commonName = webhookService + "." + m.ClusterInstance.Spec.TargetNamespace + "." + "svc"
	serverCertPEM, serverPrivateKeyPEM, err := certmanager.NewCertManager(
		[]string{organization},
		time.Until(time.Date(time.Now().Year()+defaultEffectiveYear, time.Now().Month(), time.Now().Day(), time.Now().Hour(), time.Now().Minute(), 0, 0, time.Now().Location())),
		dnsNames,
		commonName,
	).GenerateSelfSignedCerts()
	if err != nil {
		log.WithError(err).Error("failed to generate certs")
		return err
	}
	originalSecret := secret.DeepCopy()
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[corev1.TLSCertKey] = serverCertPEM.Bytes()
	secret.Data[corev1.TLSPrivateKeyKey] = serverPrivateKeyPEM.Bytes()

	if !secretExist {
		err = m.Client.Create(context.Background(), &secret)
		log.Info("creating hwameistor-admission-ca secret...")
	} else {
		err = m.Client.Patch(context.Background(), &secret, client.MergeFrom(originalSecret))
		log.Info("updating hwameistor-admission-ca secret...")
	}

	if err != nil {
		log.WithError(err).Error("failed to update admission ca secret")
	}
	return err
}

// StartCertificateWatcher starts a goroutine to continuously watch the certificate expiry and trigger rolling update
func (m *AdmissionControllerMaintainer) StartCertificateWatcher(ctx context.Context) {
	log.Info("Starting certificate watcher...")
	go func(ctx context.Context) {
		retryRollingUpdateCertificate := func() {
			retry.OnError(DefaultBackoff, func(err error) bool {
				if err != nil {
					log.WithError(err).Error("failed to rolling update certificate, retrying...")
				}
				return true
			}, func() error {
				err := m.rollingUpdateCertificate()
				if err == nil {
					go m.scheduleNextCertificateCheck()
				}
				return err
			})
		}

		// Calculate and send the next certificate check time at startup
		m.scheduleNextCertificateCheck()
		for {
			select {
			case <-ctx.Done():
				log.Info("Certificate watcher stopped")
				return
			case expireTime := <-m.watchChan:
				log.WithField("expire_time", expireTime).Info("Received certificate expiry signal")
				delay := time.Until(expireTime)
				log.WithField("delay(hours)", delay.Hours()).Info("Scheduling certificate update")
				time.AfterFunc(delay, retryRollingUpdateCertificate)
			}
		}
	}(ctx)
}

// scheduleNextCertificateCheck calculates the next certificate check time and sends it to watchChan
func (m *AdmissionControllerMaintainer) scheduleNextCertificateCheck() {
	// Get the expiry time of the current certificate
	expiryTime, err := m.getCertificateExpiryTime()
	if err != nil {
		log.WithError(err).Error("Failed to get certificate expiry time for scheduling")
		// If failed to get expiry time, set a default check time (e.g., 1 hour later)
		nextCheckTime := time.Now().Add(1 * time.Hour)
		select {
		case m.watchChan <- nextCheckTime:
			log.WithField("next_check_time", nextCheckTime).Info("Scheduled next certificate check (fallback)")
		default:
			log.Warn("Failed to send next check time to watchChan (channel full)")
		}
		return
	}

	// Start updating 7 days before the certificate expires
	updateTime := expiryTime.Add(-7 * 24 * time.Hour)

	select {
	case m.watchChan <- updateTime:
		log.WithFields(log.Fields{
			"certificate_expiry": expiryTime,
			"update_time":        updateTime,
		}).Info("Scheduled next certificate update")
	default:
		log.Warn("Failed to send update time to watchChan (channel full)")
	}
}

// getCertificateExpiryTime gets the expiry time of the current certificate
func (m *AdmissionControllerMaintainer) getCertificateExpiryTime() (time.Time, error) {
	secret := corev1.Secret{}
	if err := m.Client.Get(context.Background(), client.ObjectKey{
		Name:      "hwameistor-admission-ca",
		Namespace: m.ClusterInstance.Spec.TargetNamespace,
	}, &secret); err != nil {
		return time.Time{}, err
	}

	if secret.Data == nil || secret.Data[corev1.TLSCertKey] == nil {
		return time.Time{}, errors.New("certificate data not found in secret")
	}

	expiryInfo, err := getCertificateExpiryInfo(secret.Data[corev1.TLSCertKey])
	if err != nil {
		return time.Time{}, err
	}

	return expiryInfo.Time, nil
}

// rollingUpdateCertificate performs a rolling update of the admission certificate
// Steps:
// - clear the existing secret data to force regeneration
// - recreate the AdmissionController Pod to pick up the new certs
func (m *AdmissionControllerMaintainer) rollingUpdateCertificate() error {
	secret := corev1.Secret{}
	if err := m.Client.Get(context.Background(), client.ObjectKey{Name: "hwameistor-admission-ca",
		Namespace: m.ClusterInstance.Spec.TargetNamespace}, &secret); err != nil {
		if !apierrors.IsNotFound(err) {
			log.WithError(err).Error("failed to get admission ca secret")
			return err
		}
	} else {
		if secret.Data == nil && secret.Data[corev1.TLSCertKey] == nil {
			expireTime, err := getCertificateExpiryInfo(secret.Data[corev1.TLSCertKey])
			if err != nil {
				log.WithError(err).Error("failed to get certificate expiry info")
				return err
			}
			log.Infof("secret hwameistor-admission-ca expire time: %v", expireTime)
		}
		originalSecret := secret.DeepCopy()

		// clear the secret data to force regeneration
		secret.Data = nil
		if err := m.Client.Patch(context.Background(), &secret, client.MergeFrom(originalSecret)); err != nil {
			log.WithError(err).Error("failed to clear admission ca secret data")
			return err
		}
		log.Info("cleared admission ca secret data to force regeneration")
	}

	log.Info("regenerating admission ca secret...")
	if err := m.ensureAdmissionCA(); err != nil {
		log.WithError(err).Error("failed to regenerate admission ca secret")
		return err
	}

	// delete the existing AdmissionController Pod to force restart
	var pod corev1.Pod
	if err := m.Client.DeleteAllOf(
		context.TODO(),
		&pod,
		client.InNamespace(m.ClusterInstance.Spec.TargetNamespace),
		client.MatchingLabels(map[string]string{
			admissionControllerLabelSelectorKey: admissionControllerLabelSelectorValue,
		}),
	); err != nil {
		log.WithError(err).Error("failed to delete existing AdmissionController pods")
		return err
	}

	log.Info("deleted existing AdmissionController pods to trigger restart")
	return nil
}

func getCertificateExpiryInfo(certData []byte) (*metav1.Time, error) {
	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	expiryTime := metav1.NewTime(cert.NotAfter)
	return &expiryTime, nil
}

func FulfillAdmissionControllerSpec(clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
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
