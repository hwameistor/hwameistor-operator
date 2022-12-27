package installhwamei

import (
	"os"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func InstallCRDs(cli client.Client, targetNamespace string) error {
	dir, set := os.LookupEnv("HwameistorCRDS")
	if !set {
		dir = "/hwameistorcrds"
	}

	resources, err := ReadResourcesFromDir(dir)
	if err != nil {
		log.Errorf("Read resources err: %v", err)
		return err
	}

	for _, resource := range resources {
		if err := Install(cli, resource, targetNamespace); err != nil {
		log.Errorf("Install err: %v", err)
		return err
		}
	}

	return nil
}

