package localstorage

import corev1 "k8s.io/api/core/v1"

// mergeMemberEnv updates configured names and preserves all other existing
// entries, including variables removed from extraEnv. Desired entries keep their
// declaration order and duplicates because Kubernetes expands $(VAR) references
// in that order.
func mergeMemberEnv(current, wanted []corev1.EnvVar) []corev1.EnvVar {
	configured := make(map[string]bool, len(wanted))
	for _, item := range wanted {
		configured[item.Name] = true
	}

	merged := make([]corev1.EnvVar, 0, len(current)+len(wanted))
	next := 0
	for _, item := range current {
		if !configured[item.Name] {
			merged = append(merged, item)
			continue
		}
		// Keep other entries interleaved with configured entries.
		// If configured entries were reordered, the desired order takes priority.
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
