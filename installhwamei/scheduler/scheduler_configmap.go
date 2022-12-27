package scheduler

import (
	"os"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/hwameistor/hwameistor-operator/installhwamei"
)

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