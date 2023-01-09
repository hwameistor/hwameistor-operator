package scheduler

import (
	"bytes"
	"context"
	"io"
	"os"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"github.com/hwameistor/hwameistor-operator/installhwamei"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SchedulerConfigMapMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewSchedulerConfigMapMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *SchedulerConfigMapMaintainer {
	return &SchedulerConfigMapMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

func InstallSchedulerConfigMap(cli client.Client, targetNamespace string) error {
	filePath, set := os.LookupEnv("SchedulerConfigMap")
	if !set {
		filePath = "/scheduler-config.yaml"
	}

	fileBytes, err := installhwamei.GetFileBytes(filePath)
	if err != nil {
		log.Errorf("GetFileBytes err: %v", err)
		return err
	}

	if err := installhwamei.Install(cli, fileBytes, targetNamespace); err != nil {
		log.Errorf("Create scheduler configmap err: %v", err)
		return err
	}

	return nil
}

func (m *SchedulerConfigMapMaintainer) Ensure() error {
	filePath, set := os.LookupEnv("SchedulerConfigMap")
	if !set {
		filePath = "/scheduler-config.yaml"
	}

	fileBytes, err := installhwamei.GetFileBytes(filePath)
	if err != nil {
		log.Errorf("GetFileBytes err: %v", err)
		return err
	}

	var cmUnstructured *unstructured.Unstructured

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(fileBytes), len(fileBytes))
	for {
		var rawObj runtime.RawExtension
		err := decoder.Decode(&rawObj)
		if err != nil {
			if err == io.EOF {
				break
			} else {
        log.Errorf("decode err: %v", err)
				return err
			}
		}

		cmUnstructured, err = installhwamei.RawExtensionToUnstructured(rawObj)
		if err != nil {
			log.Errorf("RawExtensionToUnstructured err: %v", err)
			return err
		}

		if cmUnstructured.GetKind() == "ConfigMap" {
			cmUnstructured.SetNamespace(m.ClusterInstance.Spec.TargetNamespace)
		}
	}

	key := types.NamespacedName{
		Namespace: cmUnstructured.GetNamespace(),
		Name: cmUnstructured.GetName(),
	}
	var gotten corev1.ConfigMap
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), cmUnstructured); errCreate != nil {
				log.Errorf("Create Scheduler ConfigMap err: %v", err)
				return errCreate
			}
		} else {
			log.Errorf("Get Scheduler ConfigMap err: %v", err)
			return err
		}
	}

	return nil
}