package install

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const DefaultCSIAttacherTag = "v4.13.0"

const attacherCompatibilityArg = "--feature-gates=MutableCSINodeAllocatableCount=false"

func SetCSIAttacherImage(container *corev1.Container, image string) bool {
	changed := container.Image != image
	wasDefaultVersion := strings.HasSuffix(container.Image, ":"+DefaultCSIAttacherTag)
	container.Image = image
	if !strings.HasSuffix(image, ":"+DefaultCSIAttacherTag) {
		if wasDefaultVersion {
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
