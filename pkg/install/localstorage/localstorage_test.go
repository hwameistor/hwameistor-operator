/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package localstorage

import (
	"encoding/json"
	"testing"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestSetLSDaemonSetIncludesMemberExtraEnv(t *testing.T) {
	cluster := newTestCluster()
	setMemberExtraEnvAnnotation(t, cluster, []corev1.EnvVar{
		{Name: "PATH", Value: "/usr/bin:/run/current-system/sw/bin"},
	})

	daemonSet, err := SetLSDaemonSet(cluster)
	if err != nil {
		t.Fatalf("SetLSDaemonSet returned an error: %v", err)
	}
	member := findMemberContainer(t, daemonSet)
	assertEnvValue(t, member.Env, "PATH", "/usr/bin:/run/current-system/sw/bin")
}

func TestNeedOrNotToUpdateLSDaemonsetReconcilesMemberExtraEnv(t *testing.T) {
	oldCluster := newTestCluster()
	setMemberExtraEnvAnnotation(t, oldCluster, []corev1.EnvVar{
		{Name: "OLD_ENV", Value: "old"},
	})
	oldDaemonSet, err := SetLSDaemonSet(oldCluster)
	if err != nil {
		t.Fatalf("SetLSDaemonSet returned an error: %v", err)
	}
	gotten := *oldDaemonSet

	wantedCluster := newTestCluster()
	setMemberExtraEnvAnnotation(t, wantedCluster, []corev1.EnvVar{
		{Name: "NEW_ENV", Value: "new"},
	})

	needToUpdate, updated, err := needOrNotToUpdateLSDaemonset(wantedCluster, gotten)
	if err != nil {
		t.Fatalf("needOrNotToUpdateLSDaemonset returned an error: %v", err)
	}
	if !needToUpdate {
		t.Fatal("expected DaemonSet update when member extraEnv changes")
	}

	member := findMemberContainer(t, updated)
	assertEnvValue(t, member.Env, "NEW_ENV", "new")
	assertEnvMissing(t, member.Env, "OLD_ENV")
}

func TestSetLSDaemonSetRejectsInvalidMemberExtraEnvAnnotation(t *testing.T) {
	cluster := newTestCluster()
	cluster.Annotations = map[string]string{
		memberExtraEnvAnnotationName: "not-json",
	}

	if _, err := SetLSDaemonSet(cluster); err == nil {
		t.Fatal("expected an error for an invalid member extraEnv annotation")
	}
}

func setMemberExtraEnvAnnotation(t *testing.T, cluster *hwameistoriov1alpha1.Cluster, env []corev1.EnvVar) {
	t.Helper()

	value, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal member extraEnv annotation: %v", err)
	}
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	cluster.Annotations[memberExtraEnvAnnotationName] = string(value)
}

func newTestCluster() *hwameistoriov1alpha1.Cluster {
	return &hwameistoriov1alpha1.Cluster{
		Spec: hwameistoriov1alpha1.ClusterSpec{
			TargetNamespace: "hwameistor",
			RBAC: &hwameistoriov1alpha1.RBACSpec{
				ServiceAccountName: "hwameistor-admin",
			},
			LocalStorage: &hwameistoriov1alpha1.LocalStorageSpec{
				KubeletRootDir: "/var/lib/kubelet",
				CSI: &hwameistoriov1alpha1.CSISpec{
					Registrar: &hwameistoriov1alpha1.ContainerCommonSpec{
						Image: &hwameistoriov1alpha1.ImageSpec{
							Registry:   "registry.k8s.io",
							Repository: "sig-storage/csi-node-driver-registrar",
							Tag:        "v2.5.0",
						},
					},
				},
				Member: &hwameistoriov1alpha1.MemberSpec{
					Image: &hwameistoriov1alpha1.ImageSpec{
						Registry:   "ghcr.io",
						Repository: "hwameistor/local-storage",
						Tag:        "latest",
					},
					JuicesyncImage: &hwameistoriov1alpha1.ImageSpec{
						Registry:   "ghcr.io",
						Repository: "hwameistor/hwameistor-juicesync",
						Tag:        "latest",
					},
					HostPathSSHDir:  "/root/.ssh",
					HostPathDRBDDir: "/etc/drbd.d",
				},
			},
		},
	}
}

func findMemberContainer(t *testing.T, daemonSet *appsv1.DaemonSet) corev1.Container {
	t.Helper()

	for _, container := range daemonSet.Spec.Template.Spec.Containers {
		if container.Name == memberContainerName {
			return container
		}
	}

	t.Fatalf("container %q not found", memberContainerName)
	return corev1.Container{}
}

func assertEnvValue(t *testing.T, env []corev1.EnvVar, name, value string) {
	t.Helper()

	for _, item := range env {
		if item.Name == name {
			if item.Value != value {
				t.Fatalf("environment variable %q has value %q, want %q", name, item.Value, value)
			}
			return
		}
	}

	t.Fatalf("environment variable %q not found", name)
}

func assertEnvMissing(t *testing.T, env []corev1.EnvVar, name string) {
	t.Helper()

	for _, item := range env {
		if item.Name == name {
			t.Fatalf("environment variable %q should not be present", name)
		}
	}
}
