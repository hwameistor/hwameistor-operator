package install

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const csiAttacherCompatibilityTag = "v4.13.0"

const attacherCompatibilityArg = "--feature-gates=MutableCSINodeAllocatableCount=false"

func SetCSIAttacherImage(container *corev1.Container, image string) bool {
	changed := container.Image != image
	wasCompatibilityVersion := strings.HasSuffix(container.Image, ":"+csiAttacherCompatibilityTag)
	container.Image = image
	if !strings.HasSuffix(image, ":"+csiAttacherCompatibilityTag) {
		if wasCompatibilityVersion {
			for idx, arg := range container.Args {
				if arg == attacherCompatibilityArg {
					container.Args = append(container.Args[:idx:idx], container.Args[idx+1:]...)
					break
				}
			}
		}
		return changed
	}
	for _, arg := range container.Args {
		if strings.Contains(arg, "MutableCSINodeAllocatableCount=") {
			return changed
		}
	}
	container.Args = append(container.Args, attacherCompatibilityArg)
	return true
}
