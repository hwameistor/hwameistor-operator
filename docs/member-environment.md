# Local Storage member environment variables

Configure member environment variables through Helm `localStorage.member.extraEnv`
or the Cluster annotation `hwameistor.io/local-storage-member-extra-env`, which
contains a JSON array of Kubernetes `EnvVar` objects. Both literal values and
`valueFrom` references are supported.

Reconciliation updates the Operator's built-in variables and declared `extraEnv`
entries. Variables added directly to the member DaemonSet are preserved unless
their names are managed by the Operator. For a managed name, the Cluster
configuration takes precedence over direct DaemonSet edits. Removing a previously
declared extra variable removes it from the DaemonSet; removing an override of a
built-in variable restores the Operator's default.

Environment declaration order matters for `$(VAR)` expansion. Declared entries
retain their configured order, including duplicate names. Directly added entries
retain their relative order and stay interleaved with surviving managed entries
when the managed order is unchanged. When reordering declarations, check any
references between declared and directly added variables. Prefer declaring
dependent variables together in `extraEnv`.

The Operator records managed variable names, without values, in the DaemonSet
metadata annotation `hwameistor.io/local-storage-member-managed-env`. This does not
add a CRD field. Initializing this annotation alone does not change the Pod
template or restart Pods. Malformed ownership metadata stops reconciliation with
an error rather than guessing which variables to remove.

On the first reconciliation of an older DaemonSet without this annotation,
unknown variables are preserved and only currently configured names are adopted.
An extra variable removed from the Cluster before ownership was recorded cannot
be distinguished from a direct user addition and must be removed manually if no
longer needed. Variables already deleted by an older Operator are not restored
automatically. Direct patches are not restored if the DaemonSet is deleted and
recreated; use `extraEnv` for persistent configuration.
