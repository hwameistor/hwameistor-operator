package exporter

import (
	"context"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ExporterServiceMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewExporterServiceMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *ExporterServiceMaintainer {
	return &ExporterServiceMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var exporterService = corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-exporter",
	},
	Spec: corev1.ServiceSpec{
		Selector: map[string]string{
			exporterLabelSelectorKey: exporterLabelSelectorValue,
		},
		Ports: []corev1.ServicePort{
			{
				TargetPort: intstr.IntOrString{
					Type: intstr.String,
					StrVal: "exporter-apis",
				},
				Port: 8080,
			},
		},
	},
}

func SetExporterService(clusterInstance *hwameistoriov1alpha1.Cluster) {
	exporterService.Namespace = clusterInstance.Spec.TargetNamespace
}

func (m *ExporterServiceMaintainer) Ensure() error {
	SetExporterService(m.ClusterInstance)
	key := types.NamespacedName{
		Namespace: exporterService.Namespace,
		Name: exporterService.Name,
	}
	var gottenService corev1.Service
	if err := m.Client.Get(context.TODO(), key, &gottenService); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &exporterService); errCreate != nil {
				log.Errorf("Create Exporter Service err: %v", err)
				return errCreate
			}
		} else {
			log.Errorf("Get Exporters Service err: %v", err)
			return err
		}
	}

	return nil
}