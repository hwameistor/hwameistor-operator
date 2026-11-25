package install

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestSetCSIAttacherImage(t *testing.T) {
	const repository = "registry.k8s.io/sig-storage/csi-attacher:"
	older := repository + "v3.0.1"
	current := repository + DefaultCSIAttacherTag
	container := corev1.Container{Image: older, Args: []string{"--v=5"}}
	if !SetCSIAttacherImage(&container, current) || container.Image != current || !reflect.DeepEqual(container.Args, []string{"--v=5", attacherCompatibilityArg}) {
		t.Fatalf("upgrade did not apply the compatibility flag: image=%s args=%v", container.Image, container.Args)
	}
	if SetCSIAttacherImage(&container, current) {
		t.Fatal("reconciling the same image must not change the args")
	}
	if !SetCSIAttacherImage(&container, older) || !reflect.DeepEqual(container.Args, []string{"--v=5"}) {
		t.Fatalf("rollback did not restore the old arguments: %v", container.Args)
	}
	custom := []string{"--feature-gates=MutableCSINodeAllocatableCount=true"}
	container = corev1.Container{Args: append([]string(nil), custom...)}
	if !SetCSIAttacherImage(&container, current) || !reflect.DeepEqual(container.Args, custom) {
		t.Fatalf("explicit feature gate was changed: %v", container.Args)
	}
}
