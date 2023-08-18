package storageclass

import (
	"context"
	"strings"

	hwameistoroperatorv1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	hwameistorclient "github.com/hwameistor/hwameistor/pkg/apis/client/clientset/versioned"
	hwameistorinformer "github.com/hwameistor/hwameistor/pkg/apis/client/informers/externalversions/hwameistor/v1alpha1"
	hwameistorv1alpha1 "github.com/hwameistor/hwameistor/pkg/apis/hwameistor/v1alpha1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	// "k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StorageClassMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoroperatorv1alpha1.Cluster
}

func NewMaintainer(cli client.Client, clusterInstance *hwameistoroperatorv1alpha1.Cluster) *StorageClassMaintainer {
	return &StorageClassMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var volumeBindingWaitForFirstConsumer = storagev1.VolumeBindingWaitForFirstConsumer
var defaultAllowVolumeExpansionBooleanValue = true
var defaultReclaimPolicy = corev1.PersistentVolumeReclaimDelete
var defaultDiskType = "HDD"
var defaultFSType = "xfs"

var scTemplate = storagev1.StorageClass{
	Provisioner: "lvm.hwameistor.io",
	VolumeBindingMode: &volumeBindingWaitForFirstConsumer,
	Parameters: map[string]string{
		"replicaNumber": "1",
		"poolType": "REGULAR",
		"volumeKind": "LVM",
		"striped": "true",
		"convertible": "false",
	},
}

func SetStorageClassTemplate(clusterInstance *hwameistoroperatorv1alpha1.Cluster) {
	scTemplate.AllowVolumeExpansion =  &clusterInstance.Spec.StorageClass.AllowVolumeExpansion
	scTemplate.ReclaimPolicy = &clusterInstance.Spec.StorageClass.ReclaimPolicy
	scTemplate.Parameters["csi.storage.k8s.io/fstype"] = clusterInstance.Spec.StorageClass.FSType
}

func (m *StorageClassMaintainer) Ensure() error {
	localStorageNodes, err := listLocalStorageNode(m.Client)
	if err != nil {
		log.Errorf("List StorageNodes err: %v", err)
		return err
	}
	log.Infof("StorageNodes list: %+v", localStorageNodes)
	storageClassNameToCreate := generateStorageClassNameToCreateAccordingToLocalStorageNodes(localStorageNodes)
	log.Infof("Generate storageClassName to create: %+v", storageClassNameToCreate)
	existingStorageClass, err := listStorageClass(m.Client)
	if err != nil {
		log.Errorf("List StorageClass err: %v", err)
		return err
	}
	log.Infof("Existing storageClass list: %+v", existingStorageClass)
	// storageClassNameToCreate = deleteExistingStorageClassNameFromMapOfStorageClassNameToCreate(existingStorageClass, storageClassNameToCreate)
	log.Infof("StorageClassName to create finally: %+v", storageClassNameToCreate)

	SetStorageClassTemplate(m.ClusterInstance)

	var needConvertibleStorageClass bool
	if m.ClusterInstance.Status.DRBDAdapterCreatedJobNum >= 1 {
		needConvertibleStorageClass = true
	} else {
		needConvertibleStorageClass = false
	}
	var needHAStorageClass bool
	if m.ClusterInstance.Status.DRBDAdapterCreatedJobNum >= 2 {
		needHAStorageClass = true
	} else {
		needHAStorageClass = false
	}

	log.Infof("Going to generate storageClass to create, needConvertibleStorageClass: %v, needHAStorageClass: %v", needConvertibleStorageClass, needHAStorageClass)
	storageClassesToCreate := generateStorageClass(storageClassNameToCreate, needConvertibleStorageClass, needHAStorageClass)
	log.Infof("Generated storageClass to create: %+v", storageClassesToCreate)

	for _, storageClassToCreate := range storageClassesToCreate {
		if err := m.Client.Create(context.TODO(), &storageClassToCreate); err != nil {
			log.Errorf("Create StorageClass err: %v", err)
			// return err
			continue
		}
	}
	
	return nil
}

func FulfillStorageClassSpec(clusterInstance *hwameistoroperatorv1alpha1.Cluster) *hwameistoroperatorv1alpha1.Cluster {
	if clusterInstance.Spec.StorageClass == nil {
		clusterInstance.Spec.StorageClass = &hwameistoroperatorv1alpha1.StorageClassSpec{}
	}
	if clusterInstance.Spec.StorageClass.ReclaimPolicy == "" {
		clusterInstance.Spec.StorageClass.ReclaimPolicy = defaultReclaimPolicy
	}
	if clusterInstance.Spec.StorageClass.FSType == "" {
		clusterInstance.Spec.StorageClass.FSType = defaultFSType
	}

	return clusterInstance
}

func listLocalStorageNode(cli client.Client) ([]hwameistorv1alpha1.LocalStorageNode, error) {
	localStorageNodeList := hwameistorv1alpha1.LocalStorageNodeList{}
	err := cli.List(context.TODO(), &localStorageNodeList)
	return localStorageNodeList.Items, err
}

func listStorageClass(cli client.Client) ([]storagev1.StorageClass, error) {
	storageClassList := storagev1.StorageClassList{}
	err := cli.List(context.TODO(), &storageClassList)
	return storageClassList.Items, err
}

func generateStorageClassNameToCreateAccordingToLocalStorageNodes(localStorageNodes []hwameistorv1alpha1.LocalStorageNode) map[string]string {
	m := make(map[string]string)
	for _, localStorageNode := range localStorageNodes {
		for _, pool := range localStorageNode.Status.Pools {
			storageClassName := "hwameistor-storage-lvm-" + strings.ToLower(pool.Class)
			m[storageClassName] = pool.Class
		}
	}

	return m
}

func deleteExistingStorageClassNameFromMapOfStorageClassNameToCreate(existingStorageClass []storagev1.StorageClass, storageClassNameToCreate map[string]string) map[string]string {
	for _, existingStorageClass := range existingStorageClass {
		delete(storageClassNameToCreate, existingStorageClass.Name)
	}

	return storageClassNameToCreate
}

func generateStorageClass(storageClassNameToCreate map[string]string, needConvertibleStorageClass bool, needHAStorageClass bool) []storagev1.StorageClass {
	storageClasses := make([]storagev1.StorageClass, 0)
	for name, poolClass := range storageClassNameToCreate {
		storageClass := scTemplate.DeepCopy()
		storageClass.Name = name
		storageClass.Parameters["poolClass"] = poolClass
		storageClasses = append(storageClasses, *storageClass)

		if needConvertibleStorageClass {
			convertibleStorageClass := scTemplate.DeepCopy()
			convertibleStorageClass.Name = name + "-convertible"
			convertibleStorageClass.Parameters["poolClass"] = strings.ToUpper(poolClass)
			convertibleStorageClass.Parameters["convertible"] = "true"
			convertibleStorageClass.Parameters["replicaNumber"] = "1"
			storageClasses = append(storageClasses, *convertibleStorageClass)
		}
		
		if needHAStorageClass {
			haStorageClass := scTemplate.DeepCopy()
			haStorageClass.Name = name + "-ha"
			haStorageClass.Parameters["poolClass"] = strings.ToUpper(poolClass)
			haStorageClass.Parameters["convertible"] = "true"
			haStorageClass.Parameters["replicaNumber"] = "2"
			storageClasses = append(storageClasses, *haStorageClass)
		}
	}

	return storageClasses
}

func WatchLocalStorageNodes(cli client.Client, clusterKey types.NamespacedName, stopCh <-chan struct{}) {
	fcs := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Infof("OnAdd: %+v", obj)
			clusterInstance := &hwameistoroperatorv1alpha1.Cluster{}
			err := cli.Get(context.TODO(), clusterKey, clusterInstance)
			if err != nil {
				if errors.IsNotFound(err) {
					log.WithError(err).Error("Cluster instance not found")
					return
				}
				log.Errorf("Get instance err: %v", err)
				return
			}
			NewMaintainer(cli, clusterInstance).Ensure()
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			log.Infof("OnUpdate: old: %+v", oldObj)
			log.Infof("OnUpdate: new: %+v", newObj)
			clusterInstance := &hwameistoroperatorv1alpha1.Cluster{}
			err := cli.Get(context.TODO(), clusterKey, clusterInstance)
			if err != nil {
				if errors.IsNotFound(err) {
					log.WithError(err).Error("Cluster instance not found")
					return
				}
				log.Errorf("Get instance err: %v", err)
				return
			}
			NewMaintainer(cli, clusterInstance).Ensure()
		},
		DeleteFunc: func(obj interface{}) {
			log.Infof("OnDelete: %+v", obj)
		},
	}
	config, err := rest.InClusterConfig()
	// config, err := clientcmd.BuildConfigFromFlags("", "/Users/home/.kube/config")
	if err != nil {
		log.WithError(err).Error("Failed to build kubernetes config")
		return
		// return err
	}
	clientset, err := hwameistorclient.NewForConfig(config)
	if err != nil {
		log.WithError(err).Error("Failed to build clientset")
		return
		// return err
	}
	lsnInformer := hwameistorinformer.NewLocalStorageNodeInformer(clientset, 0, cache.Indexers{})
	lsnInformer.AddEventHandler(fcs)
	log.Infof("Going to run informer")
	lsnInformer.Run(stopCh)
	log.Infof("Informer run over")

	// return nil
	// return
}