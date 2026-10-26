package localstorage

import (
	"encoding/json"
	"reflect"
	"strings"
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
				assertEnvMissing(t, member.Env, "REMOVED")
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

func TestReconcileAdoptsLegacyMemberEnvWithoutRollout(t *testing.T) {
	cluster := newTestCluster()
	ds, err := SetLSDaemonSet(cluster)
	if err != nil {
		t.Fatal(err)
	}
	// No historical ownership is available, including for an extraEnv entry
	// removed before upgrading. Both kinds of unknown entry must survive.
	ds.Annotations = map[string]string{"example.com/custom": "keep"}
	for i := range ds.Spec.Template.Spec.Containers {
		if ds.Spec.Template.Spec.Containers[i].Name == memberContainerName {
			ds.Spec.Template.Spec.Containers[i].Env = append(ds.Spec.Template.Spec.Containers[i].Env,
				corev1.EnvVar{Name: "MANUAL", Value: "keep"},
				corev1.EnvVar{Name: "OLD_EXTRA", Value: "keep"})
		}
	}
	before := ds.DeepCopy()
	changed, updated, err := needOrNotToUpdateLSDaemonset(cluster, *ds)
	if err != nil || !changed {
		t.Fatalf("adopting legacy DaemonSet: changed=%v error=%v", changed, err)
	}
	if !reflect.DeepEqual(ds, before) || !reflect.DeepEqual(updated.Spec.Template, before.Spec.Template) {
		t.Fatal("adopting ownership mutated the input or changed the Pod template")
	}
	if updated.Annotations["example.com/custom"] != "keep" {
		t.Fatal("removed unrelated annotation")
	}
	var names []string
	if err := json.Unmarshal([]byte(updated.Annotations[memberManagedEnvAnnotationName]), &names); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if name == "MANUAL" || name == "OLD_EXTRA" {
			t.Fatalf("claimed unknown env %s", name)
		}
	}
	if changed, _, err := needOrNotToUpdateLSDaemonset(cluster, *updated); err != nil || changed {
		t.Fatalf("ownership adoption is not idempotent: changed=%v error=%v", changed, err)
	}

	// A later explicit declaration takes ownership. Removing it removes that
	// variable only; unrelated manual entries remain untouched.
	setMemberExtraEnvAnnotation(t, cluster, []corev1.EnvVar{{Name: "OLD_EXTRA", Value: "declared"}})
	_, updated, err = needOrNotToUpdateLSDaemonset(cluster, *updated)
	if err != nil {
		t.Fatal(err)
	}
	assertEnvValue(t, findMemberContainer(t, updated).Env, "OLD_EXTRA", "declared")
	delete(cluster.Annotations, memberExtraEnvAnnotationName)
	_, updated, err = needOrNotToUpdateLSDaemonset(cluster, *updated)
	if err != nil {
		t.Fatal(err)
	}
	assertEnvMissing(t, findMemberContainer(t, updated).Env, "OLD_EXTRA")
	assertEnvValue(t, findMemberContainer(t, updated).Env, "MANUAL", "keep")
}

func TestReconcileRejectsInvalidMemberEnvOwnership(t *testing.T) {
	for _, raw := range []string{"not-json", `{"CUSTOM":true}`, `["CUSTOM",42]`} {
		t.Run(raw, func(t *testing.T) {
			cluster := newTestCluster()
			ds, err := SetLSDaemonSet(cluster)
			if err != nil {
				t.Fatal(err)
			}
			ds.Annotations[memberManagedEnvAnnotationName] = raw
			before := ds.DeepCopy()
			changed, updated, err := needOrNotToUpdateLSDaemonset(cluster, *ds)
			if err == nil || !strings.Contains(err.Error(), memberManagedEnvAnnotationName) || changed {
				t.Fatalf("invalid ownership: changed=%v error=%v", changed, err)
			}
			if !reflect.DeepEqual(ds, before) || !reflect.DeepEqual(updated, before) {
				t.Fatal("invalid ownership changed the DaemonSet")
			}
		})
	}
}

func TestMergeMemberEnvOrder(t *testing.T) {
	for _, tt := range []struct {
		name     string
		current  []corev1.EnvVar
		wanted   []corev1.EnvVar
		previous []string
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
			name: "reordered managed entries and removed entry",
			current: []corev1.EnvVar{
				{Name: "A", Value: "old"}, {Name: "MANUAL", Value: "keep"},
				{Name: "B", Value: "old"}, {Name: "REMOVED", Value: "old"},
			},
			wanted:   []corev1.EnvVar{{Name: "B", Value: "new"}, {Name: "A", Value: "$(B)"}},
			previous: []string{"A", "B", "REMOVED"},
			expected: []corev1.EnvVar{{Name: "B", Value: "new"}, {Name: "A", Value: "$(B)"}, {Name: "MANUAL", Value: "keep"}},
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
			merged := mergeMemberEnv(tt.current, tt.wanted, tt.previous)
			if !reflect.DeepEqual(merged, tt.expected) {
				t.Fatalf("got %v, want %v", merged, tt.expected)
			}
			if again := mergeMemberEnv(merged, tt.wanted, tt.previous); !reflect.DeepEqual(again, merged) {
				t.Fatalf("merge is not idempotent: %v", again)
			}
		})
	}
}
