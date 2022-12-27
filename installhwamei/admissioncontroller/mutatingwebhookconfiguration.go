package admissioncontroller

import (
	"context"

	"k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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