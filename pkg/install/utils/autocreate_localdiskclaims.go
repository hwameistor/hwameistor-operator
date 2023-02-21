package utils

import (
	"context"
	"strings"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	hwameistorv1alpha1 "github.com/hwameistor/hwameistor/pkg/apis/hwameistor/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SiftAvailableAndUnreservedDisks(localDisks []hwameistorv1alpha1.LocalDisk) []hwameistorv1alpha1.LocalDisk {
	sifted := make([]hwameistorv1alpha1.LocalDisk, 0)
	for _, localDisk := range localDisks {
		if (localDisk.Status.State == hwameistorv1alpha1.LocalDiskAvailable) && (!localDisk.Spec.Reserved) {
			sifted = append(sifted, localDisk)
		}
	}

	return sifted
}

func GenerateLocalDiskClaimsToCreateAccordingToLocalDisks(localDisks []hwameistorv1alpha1.LocalDisk) []hwameistorv1alpha1.LocalDiskClaim {
	localDiskClaims := make([]hwameistorv1alpha1.LocalDiskClaim, 0)
	m := make(map[string]bool)
	for _, localDisk := range localDisks {
		key := localDisk.Spec.NodeName + ":" + localDisk.Spec.DiskAttributes.Type
		if m[key] {
			//already constructed this ldc
			continue
		}
		claimName := localDisk.Spec.NodeName + "-" + strings.ToLower(localDisk.Spec.DiskAttributes.Type)  + "-" + "claim"
		localDiskClaim := hwameistorv1alpha1.LocalDiskClaim{
			ObjectMeta: v1.ObjectMeta{
				Name: claimName,
			},
			Spec: hwameistorv1alpha1.LocalDiskClaimSpec{
				NodeName: localDisk.Spec.NodeName,
				Description: hwameistorv1alpha1.DiskClaimDescription{
					DiskType: localDisk.Spec.DiskAttributes.Type,
				},
			},
		}
		localDiskClaims = append(localDiskClaims, localDiskClaim)
		m[key] = true
	}

	return localDiskClaims
}

func CreateLocalDiskClaims (cli client.Client, localDiskClaims []hwameistorv1alpha1.LocalDiskClaim) error {
	for _, localDiskClaim := range localDiskClaims {
		if err := cli.Create(context.TODO(), &localDiskClaim); err != nil {
			return err
		}
	}

	return nil
}