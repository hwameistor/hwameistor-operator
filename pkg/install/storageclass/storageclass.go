package storageclass

import (
	"context"
	"strings"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StorageClassMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *StorageClassMaintainer {
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

var sc = storagev1.StorageClass{
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

func SetStorageClass(clusterInstance *hwameistoriov1alpha1.Cluster) {
	sc.Name = "hwameistor-storage-lvm-" + strings.ToLower(clusterInstance.Spec.StorageClass.DiskType)
	sc.AllowVolumeExpansion =  &clusterInstance.Spec.StorageClass.AllowVolumeExpansion
	sc.ReclaimPolicy = &clusterInstance.Spec.StorageClass.ReclaimPolicy
	sc.Parameters["poolClass"] = clusterInstance.Spec.StorageClass.DiskType
	sc.Parameters["csi.storage.k8s.io/fstype"] = clusterInstance.Spec.StorageClass.FSType
}

func (m *StorageClassMaintainer) Ensure() error {
	SetStorageClass(m.ClusterInstance)
	key := types.NamespacedName{
		Name: sc.Name,
	}

	var gotten storagev1.StorageClass
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &sc); errCreate != nil {
				log.Errorf("Create StorageClass err: %v", err)
				return errCreate
			}
		} else {
			log.Errorf("Get StorageClass err: %v", err)
			return err
		}
	}
	
	return nil
}

func FulfillStorageClassSpec(clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.StorageClass == nil {
		clusterInstance.Spec.StorageClass = &hwameistoriov1alpha1.StorageClassSpec{}
	}
	// if clusterInstance.Spec.StorageClass.AllowVolumeExpansion == false {
	// 	clusterInstance.Spec.StorageClass.AllowVolumeExpansion = defaultAllowVolumeExpansionBooleanValue
	// }
	if clusterInstance.Spec.StorageClass.ReclaimPolicy == "" {
		clusterInstance.Spec.StorageClass.ReclaimPolicy = defaultReclaimPolicy
	}
	if clusterInstance.Spec.StorageClass.DiskType == "" {
		clusterInstance.Spec.StorageClass.DiskType = defaultDiskType
	}
	if clusterInstance.Spec.StorageClass.FSType == "" {
		clusterInstance.Spec.StorageClass.FSType = defaultFSType
	}

	return clusterInstance
}