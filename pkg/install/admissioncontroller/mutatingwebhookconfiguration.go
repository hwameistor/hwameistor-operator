package admissioncontroller

import (
	"context"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AdmissionControllerMutatingWebhookConfigurationMaintainer struct {
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewAdmissionControllerMutatingWebhookConfigurationMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *AdmissionControllerMutatingWebhookConfigurationMaintainer {
	return &AdmissionControllerMutatingWebhookConfigurationMaintainer{
		Client:          cli,
		ClusterInstance: clusterInstance,
	}
}

var mutatingWebhookConfiguration = v1.MutatingWebhookConfiguration{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-admission-mutate",
	},
}

func InstallAdmissionControllerMutatingWebhookConfiguration(cli client.Client) error {
	if err := cli.Create(context.TODO(), &mutatingWebhookConfiguration); err != nil {
		log.Errorf("Create Admission Controller MutatingWebhookConfiguration err: %v", err)
		return err
	}

	return nil
}

func (m *AdmissionControllerMutatingWebhookConfigurationMaintainer) Ensure() error {
	key := types.NamespacedName{
		Namespace: mutatingWebhookConfiguration.Namespace,
		Name:      mutatingWebhookConfiguration.Name,
	}
	var gotten v1.MutatingWebhookConfiguration
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &mutatingWebhookConfiguration); errCreate != nil {
				log.Errorf("Create AdmissionController MutatingWebhookConfiguration err: %v", err)
				return errCreate
			}
		} else {
			log.Errorf("Get AdmissionController MutatingWebhookConfiguration err: %v", err)
			return err
		}
	}

	return nil
}
