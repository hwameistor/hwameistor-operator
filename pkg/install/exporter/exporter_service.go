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
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewExporterServiceMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *ExporterServiceMaintainer {
	return &ExporterServiceMaintainer{
		Client:          cli,
		ClusterInstance: clusterInstance,
	}
}

var exporterService = corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-exporter",
		Labels: map[string]string{
			exporterLabelSelectorKey: exporterLabelSelectorValue,
		},
	},
	Spec: corev1.ServiceSpec{
		Selector: map[string]string{
			exporterLabelSelectorKey: exporterLabelSelectorValue,
		},
		Ports: []corev1.ServicePort{
			{
				TargetPort: intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "metrics",
				},
				Port: 80,
				Name: "metrics",
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
		Name:      exporterService.Name,
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

func (m *ExporterServiceMaintainer) Uninstall() error {
	key := types.NamespacedName{
		Namespace: m.ClusterInstance.Spec.TargetNamespace,
		Name:      exporterService.Name,
	}
	var gotten corev1.Service
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if errors.IsNotFound(err) {
			return nil
		} else {
			log.Errorf("get Exporter Service err: %v", err)
			return err
		}
	} else {
		if err = m.Client.Delete(context.TODO(), &gotten); err != nil {
			return err
		}
	}

	return nil
}
