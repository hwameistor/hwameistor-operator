package install

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/hwameistor/hwameistor-operator/pkg/kubeconfig"
	log "github.com/sirupsen/logrus"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
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
		if err := createOrUpdateCRD(cli, resource); err != nil {
			log.Errorf("Ensure crd err: %v", err)
			return err
		}
	}

	return nil
}

func createOrUpdateCRD(cli client.Client, resourceBytes []byte) error {
	extensionCli, err := clientset.NewForConfig(kubeconfig.Get())
	if err != nil {
		log.Errorf("Generate Clientset err: %v", err)
		return err
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(resourceBytes), len(resourceBytes))
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

		unstructuredObj, err := RawExtensionToUnstructured(rawObj)
		if err != nil {
			log.Errorf("RawExtensionToUnstructured err: %v", err)
			return err
		}

		crd, err := extensionCli.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), unstructuredObj.GetName(), v1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				creatingErr := cli.Create(context.TODO(), unstructuredObj, &client.CreateOptions{})
				if creatingErr != nil {
					log.Errorf("Create crd err :%v", creatingErr)
					return creatingErr
				}
				return nil
			}
			log.Errorf("Get crd err: %v", err)
			return err
		}
		if crd != nil {
			unstructuredObj.SetResourceVersion(crd.ResourceVersion)
			if updatingErr := cli.Update(context.TODO(), unstructuredObj, &client.UpdateOptions{}); err != nil {
				log.Errorf("Update crd err: %v", updatingErr)
				return updatingErr
			}
			log.Infof("Successfully updated crd: %v", unstructuredObj.GetName())
			return nil
		}
	}
	return nil
}
