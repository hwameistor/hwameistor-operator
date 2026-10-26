package localstorage

import (
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const memberManagedEnvAnnotationName = "hwameistor.io/local-storage-member-managed-env"

func getManagedMemberEnv(ds *appsv1.DaemonSet) ([]string, error) {
	raw := ds.Annotations[memberManagedEnvAnnotationName]
	if raw == "" {
		// Older DaemonSets have no ownership history. Unknown variables must be
		// preserved because they may have been added directly by the user.
		return nil, nil
	}
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return nil, fmt.Errorf("parse DaemonSet annotation %q: %w", memberManagedEnvAnnotationName, err)
	}
	return names, nil
}

func setManagedMemberEnv(ds *appsv1.DaemonSet, env []corev1.EnvVar) bool {
	names := make([]string, 0, len(env))
	seen := make(map[string]bool, len(env))
	for _, item := range env {
		if !seen[item.Name] {
			names = append(names, item.Name)
			seen[item.Name] = true
		}
	}
	// Marshaling a string slice cannot fail. Store names only, never values.
	raw, _ := json.Marshal(names)
	if ds.Annotations[memberManagedEnvAnnotationName] == string(raw) {
		return false
	}
	if ds.Annotations == nil {
		ds.Annotations = make(map[string]string)
	}
	// DaemonSet metadata avoids a rollout when only ownership is initialized.
	ds.Annotations[memberManagedEnvAnnotationName] = string(raw)
	return true
}

// mergeMemberEnv replaces current and previously managed names while retaining
// user-added entries. Desired entries keep their declaration order, including
// duplicates, because Kubernetes expands $(VAR) references in that order.
func mergeMemberEnv(current, wanted []corev1.EnvVar, previouslyManaged []string) []corev1.EnvVar {
	managed := make(map[string]bool, len(previouslyManaged)+len(wanted))
	for _, name := range previouslyManaged {
		managed[name] = true
	}
	for _, item := range wanted {
		managed[item.Name] = true
	}

	merged := make([]corev1.EnvVar, 0, len(current)+len(wanted))
	next := 0
	for _, item := range current {
		if !managed[item.Name] {
			merged = append(merged, item)
			continue
		}
		// Keep manual entries interleaved with surviving managed entries.
		// If managed entries were reordered, the desired order takes priority.
		for i := next; i < len(wanted); i++ {
			if wanted[i].Name == item.Name {
				merged = append(merged, wanted[next:i+1]...)
				next = i + 1
				break
			}
		}
	}
	return append(merged, wanted[next:]...)
}
