package storageclass

import (
	"context"
	"strings"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	log "github.com/sirupsen/logrus"
)

var volumeBindingWaitForFirstConsumer = storagev1.VolumeBindingWaitForFirstConsumer

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

func InstallStorageClass(cli client.Client) error {
	if err := cli.Create(context.TODO(), &sc); err != nil {
		log.Errorf("Create StorageClass err: %v", err)
		return err
	}

	return nil
}