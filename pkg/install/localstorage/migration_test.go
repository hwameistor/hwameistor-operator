package localstorage

import (
	"reflect"
	"strings"
	"testing"

	api "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func migrationCluster() *api.Cluster {
	return FulfillLSDaemonsetSpec(&api.Cluster{Spec: api.ClusterSpec{
		TargetNamespace: "test",
		RBAC:            &api.RBACSpec{ServiceAccountName: "test"},
	}})
}

func assertMigrationConfig(t *testing.T, container corev1.Container, version, check string) {
	t.Helper()
	versions, checks := 0, 0
	for _, env := range container.Env {
		if env.Name == juiceFSVersionEnv {
			versions++
			if env.Value != version || env.ValueFrom != nil {
				t.Fatalf("version env: %+v", env)
			}
		}
	}
	for _, arg := range container.Args {
		if strings.HasPrefix(arg, "--migrate-check") {
			checks++
			if arg != "--migrate-check="+check {
				t.Fatalf("check arg: %q", arg)
			}
		}
	}
	if versions != 1 || checks != 1 {
		t.Fatalf("version entries=%d check flags=%d", versions, checks)
	}
}

func TestMigrationDefaultsAndOverrides(t *testing.T) {
	for _, enabled := range []string{"", "true", "false"} {
		t.Setenv(juiceFSVersionEnv, "")
		t.Setenv(migrateCheckEnv, enabled)
		cluster := migrationCluster()
		check := "true"
		if enabled == "false" {
			check = "false"
		}
		ds, err := SetLSDaemonSet(cluster)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == memberContainerName {
				assertMigrationConfig(t, c, "1.2.0", check)
			}
		}
		t.Setenv(juiceFSVersionEnv, "1.4.1")
		changed, updated, err := needOrNotToUpdateLSDaemonset(cluster, *ds)
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("version change did not update existing DaemonSet")
		}
		for _, c := range updated.Spec.Template.Spec.Containers {
			if c.Name == memberContainerName {
				assertMigrationConfig(t, c, "1.4.1", check)
			}
		}
		changed, _, err = needOrNotToUpdateLSDaemonset(cluster, *updated)
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			t.Fatal("unchanged configuration is not idempotent")
		}
	}
}

func TestMigrationReconcileOldDaemonSet(t *testing.T) {
	t.Setenv(juiceFSVersionEnv, "")
	t.Setenv(migrateCheckEnv, "")
	cluster := migrationCluster()
	setMemberExtraEnvAnnotation(t, cluster, []corev1.EnvVar{{Name: "CUSTOM", Value: "keep"}})
	ds, err := SetLSDaemonSet(cluster)
	if err != nil {
		t.Fatal(err)
	}
	for i := range ds.Spec.Template.Spec.Containers {
		c := &ds.Spec.Template.Spec.Containers[i]
		if c.Name != memberContainerName {
			continue
		}
		c.Args = []string{"--v=5", "--migrate-check=false", "--migrate-check", "false", "--custom=keep"}
		c.Env = []corev1.EnvVar{{Name: juiceFSVersionEnv, Value: "1.4.1"}, {Name: juiceFSVersionEnv, ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}}, {Name: "CUSTOM", Value: "keep"}}
	}
	before := ds.DeepCopy()
	changed, updated, err := needOrNotToUpdateLSDaemonset(cluster, *ds)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("old check=false was not updated to the default")
	}
	if !reflect.DeepEqual(ds, before) {
		t.Fatal("mutated input DaemonSet")
	}
	if changed, _, err := needOrNotToUpdateLSDaemonset(cluster, *updated); changed || err != nil {
		t.Fatal("repairing a missing image env requires another update")
	}
	for _, c := range updated.Spec.Template.Spec.Containers {
		if c.Name != memberContainerName {
			continue
		}
		assertMigrationConfig(t, c, "1.2.0", "true")
		if !reflect.DeepEqual(c.Args, []string{"--v=5", "--custom=keep", "--migrate-check=true"}) {
			t.Fatalf("unmanaged arguments changed: %v", c.Args)
		}
		found := false
		for _, env := range c.Env {
			if env.Name == "CUSTOM" && env.Value == "keep" {
				found = true
			}
		}
		if !found {
			t.Fatal("unmanaged environment removed")
		}
	}
	t.Setenv(migrateCheckEnv, "false")
	changed, updated, err = needOrNotToUpdateLSDaemonset(cluster, *updated)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("explicitly disabling check did not reconcile")
	}
	for _, c := range updated.Spec.Template.Spec.Containers {
		if c.Name == memberContainerName {
			assertMigrationConfig(t, c, "1.2.0", "false")
		}
	}
}

func TestMigrationDefaultsPreserveCustomImage(t *testing.T) {
	cluster := migrationCluster()
	if got := getJuicesyncEnvFromClusterInstance(cluster); !strings.HasSuffix(got, "/hwameistor/hwameistor-juicesync:v1.2.0-v1.4.1-01") {
		t.Fatalf("default image: %s", got)
	}
	member := cluster.Spec.LocalStorage.Member
	member.JuicesyncImage = &api.ImageSpec{Registry: "example.com", Repository: "custom/sync", Tag: "test"}
	FulfillLSDaemonsetSpec(cluster)
	if getJuicesyncEnvFromClusterInstance(cluster) != "example.com/custom/sync:test" {
		t.Fatal("custom image overwritten")
	}
}

func TestInvalidMigrationEnvironmentRejected(t *testing.T) {
	for _, env := range []string{juiceFSVersionEnv, migrateCheckEnv} {
		t.Run(env, func(t *testing.T) {
			t.Setenv(juiceFSVersionEnv, "1.2.0")
			t.Setenv(migrateCheckEnv, "true")
			t.Setenv(env, "invalid")
			cluster := migrationCluster()
			// A nil client ensures invalid configuration fails before any API operation.
			if _, err := NewMaintainer(nil, cluster).Ensure(); err == nil || !strings.Contains(err.Error(), env) {
				t.Fatalf("invalid %s: %v", env, err)
			}
		})
	}
}

func TestMigrationVersionAnnotationOverridesOperatorDefault(t *testing.T) {
	t.Setenv(juiceFSVersionEnv, "1.2.0")
	t.Setenv(migrateCheckEnv, "true")
	cluster := migrationCluster()
	setMemberExtraEnvAnnotation(t, cluster, []corev1.EnvVar{
		{Name: juiceFSVersionEnv, Value: "1.2.0"},
		{Name: juiceFSVersionEnv, Value: "1.4.1"},
	})
	ds, err := SetLSDaemonSet(cluster)
	if err != nil {
		t.Fatal(err)
	}
	assertMigrationConfig(t, findMemberContainer(t, ds), "1.4.1", "true")
	delete(cluster.Annotations, memberExtraEnvAnnotationName)
	changed, updated, err := needOrNotToUpdateLSDaemonset(cluster, *ds)
	if err != nil || !changed {
		t.Fatalf("removing override: changed=%v err=%v", changed, err)
	}
	assertMigrationConfig(t, findMemberContainer(t, updated), "1.2.0", "true")
	setMemberExtraEnvAnnotation(t, cluster, []corev1.EnvVar{{Name: juiceFSVersionEnv, Value: "latest"}})
	if _, err := SetLSDaemonSet(cluster); err == nil {
		t.Fatal("invalid version annotation accepted")
	}
}

func TestMigrationVersionValueFromPreserved(t *testing.T) {
	t.Setenv(juiceFSVersionEnv, "1.2.0")
	t.Setenv(migrateCheckEnv, "true")
	cluster := migrationCluster()
	version := corev1.EnvVar{Name: juiceFSVersionEnv, ValueFrom: &corev1.EnvVarSource{
		ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "migration"}, Key: "version"},
	}}
	setMemberExtraEnvAnnotation(t, cluster, []corev1.EnvVar{version})
	ds, err := SetLSDaemonSet(cluster)
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range findMemberContainer(t, ds).Env {
		if env.Name == juiceFSVersionEnv {
			if !reflect.DeepEqual(env, version) {
				t.Fatalf("valueFrom changed: %+v", env)
			}
			return
		}
	}
	t.Fatal("version environment missing")
}
