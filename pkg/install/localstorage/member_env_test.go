package localstorage

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestReconcilePreservesManualMemberEnv(t *testing.T) {
	for _, changeConfig := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "updated configuration"}[changeConfig], func(t *testing.T) {
			cluster := newTestCluster()
			setMemberExtraEnvAnnotation(t, cluster, []corev1.EnvVar{
				{Name: "DECLARED", Value: "old"},
				{Name: "REMOVED", Value: "old"},
			})
			ds, err := SetLSDaemonSet(cluster)
			if err != nil {
				t.Fatal(err)
			}
			manual := []corev1.EnvVar{
				{Name: "CUSTOM_BIN", Value: "/custom/bin"},
				{Name: "PATH", Value: "$(CUSTOM_BIN):/usr/bin:/bin"},
				{Name: "TOKEN", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "member-secrets"}, Key: "token",
				}}},
			}
			for i := range ds.Spec.Template.Spec.Containers {
				c := &ds.Spec.Template.Spec.Containers[i]
				if c.Name == memberContainerName {
					c.Env = append(c.Env, manual...)
					c.EnvFrom = []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "member-config"},
					}}}
				}
			}
			ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, corev1.Container{
				Name: "sidecar", Env: []corev1.EnvVar{{Name: "CUSTOM", Value: "keep"}},
			})
			if changeConfig {
				cluster.Spec.LocalStorage.Member.Image.Tag = "updated"
				setMemberExtraEnvAnnotation(t, cluster, []corev1.EnvVar{
					{Name: "DECLARED", Value: "new"},
					{Name: "ADDED", Value: "new"},
				})
			}
			before := ds.DeepCopy()
			changed, updated, err := needOrNotToUpdateLSDaemonset(cluster, *ds)
			if err != nil || changed != changeConfig {
				t.Fatalf("changed=%v, want %v; error=%v", changed, changeConfig, err)
			}
			if !reflect.DeepEqual(ds, before) {
				t.Fatal("reconciliation mutated its input")
			}
			member := findMemberContainer(t, updated)
			var kept []corev1.EnvVar
			for _, env := range member.Env {
				if env.Name == "CUSTOM_BIN" || env.Name == "PATH" || env.Name == "TOKEN" {
					kept = append(kept, env)
				}
			}
			if !reflect.DeepEqual(kept, manual) {
				t.Fatalf("manual env changed: got %+v, want %+v", kept, manual)
			}
			if changeConfig {
				assertEnvValue(t, member.Env, "DECLARED", "new")
				assertEnvValue(t, member.Env, "ADDED", "new")
				assertEnvValue(t, member.Env, "REMOVED", "old")
			}
			if !reflect.DeepEqual(member.EnvFrom, findMemberContainer(t, before).EnvFrom) ||
				!reflect.DeepEqual(updated.Spec.Template.Spec.Containers[2], before.Spec.Template.Spec.Containers[2]) {
				t.Fatal("changed envFrom or sidecar")
			}
			if changed, _, err := needOrNotToUpdateLSDaemonset(cluster, *updated); err != nil || changed {
				t.Fatalf("reconciliation is not idempotent: changed=%v error=%v", changed, err)
			}
		})
	}
}

func TestReconcileRetainsEnvRemovedFromExtraEnv(t *testing.T) {
	for _, removeAnnotation := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty extraEnv", true: "removed annotation"}[removeAnnotation], func(t *testing.T) {
			cluster := newTestCluster()
			setMemberExtraEnvAnnotation(t, cluster, []corev1.EnvVar{{Name: "OLD_EXTRA", Value: "keep"}})
			ds, err := SetLSDaemonSet(cluster)
			if err != nil {
				t.Fatal(err)
			}
			ds.Annotations = map[string]string{"example.com/custom": "keep"}
			if removeAnnotation {
				delete(cluster.Annotations, memberExtraEnvAnnotationName)
			} else {
				setMemberExtraEnvAnnotation(t, cluster, []corev1.EnvVar{})
			}

			before := ds.DeepCopy()
			changed, updated, err := needOrNotToUpdateLSDaemonset(cluster, *ds)
			if err != nil || changed {
				t.Fatalf("removing extraEnv should leave the DaemonSet unchanged: changed=%v error=%v", changed, err)
			}
			assertEnvValue(t, findMemberContainer(t, updated).Env, "OLD_EXTRA", "keep")
			if !reflect.DeepEqual(ds, before) || !reflect.DeepEqual(updated, before) {
				t.Fatal("removing extraEnv changed the DaemonSet")
			}
		})
	}
}

func TestMergeMemberEnvOrder(t *testing.T) {
	for _, tt := range []struct {
		name     string
		current  []corev1.EnvVar
		wanted   []corev1.EnvVar
		expected []corev1.EnvVar
	}{
		{
			name: "interleaved references and duplicate declarations",
			current: []corev1.EnvVar{
				{Name: "MANUAL", Value: "custom"},
				{Name: "VERSION", Value: "old"},
				{Name: "FIRST", Value: "$(VERSION)"},
				{Name: "VERSION", Value: "old-override"},
				{Name: "LAST", Value: "$(VERSION)"},
			},
			wanted: []corev1.EnvVar{{Name: "VERSION", Value: "$(MANUAL)"}, {Name: "VERSION", Value: "override"}},
			expected: []corev1.EnvVar{
				{Name: "MANUAL", Value: "custom"},
				{Name: "VERSION", Value: "$(MANUAL)"},
				{Name: "FIRST", Value: "$(VERSION)"},
				{Name: "VERSION", Value: "override"},
				{Name: "LAST", Value: "$(VERSION)"},
			},
		},
		{
			name: "reordered declarations preserve undeclared entry",
			current: []corev1.EnvVar{
				{Name: "A", Value: "old"}, {Name: "MANUAL", Value: "keep"},
				{Name: "B", Value: "old"}, {Name: "REMOVED", Value: "old"},
			},
			wanted:   []corev1.EnvVar{{Name: "B", Value: "new"}, {Name: "A", Value: "$(B)"}},
			expected: []corev1.EnvVar{{Name: "B", Value: "new"}, {Name: "A", Value: "$(B)"}, {Name: "MANUAL", Value: "keep"}, {Name: "REMOVED", Value: "old"}},
		},
		{
			name: "duplicate manual entries remain ordered",
			current: []corev1.EnvVar{
				{Name: "MANUAL", Value: "first"}, {Name: "A", Value: "old"},
				{Name: "MANUAL", Value: "$(MANUAL):second"},
			},
			wanted: []corev1.EnvVar{{Name: "A", Value: "new"}},
			expected: []corev1.EnvVar{
				{Name: "MANUAL", Value: "first"}, {Name: "A", Value: "new"},
				{Name: "MANUAL", Value: "$(MANUAL):second"},
			},
		},
		{
			name:     "new declarations can refer to manual entries",
			current:  []corev1.EnvVar{{Name: "MANUAL", Value: "keep"}},
			wanted:   []corev1.EnvVar{{Name: "NEW", Value: "$(MANUAL)"}},
			expected: []corev1.EnvVar{{Name: "MANUAL", Value: "keep"}, {Name: "NEW", Value: "$(MANUAL)"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			merged := mergeMemberEnv(tt.current, tt.wanted)
			if !reflect.DeepEqual(merged, tt.expected) {
				t.Fatalf("got %v, want %v", merged, tt.expected)
			}
			if again := mergeMemberEnv(merged, tt.wanted); !reflect.DeepEqual(again, merged) {
				t.Fatalf("merge is not idempotent: %v", again)
			}
		})
	}
}
