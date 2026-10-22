# Migration binary selection and default content verification

The migration image bundles JuiceFS 1.2.0 and 1.4.1. Operator reads `JUICEFS_VERSION` from its own Deployment environment and passes it to the Local Storage member. The default is `1.2.0`; `1.4.1` is the only alternative. Operator also reads `MIGRATE_CHECK` (default `true`) and reconciles the member's `--migrate-check` argument. Unsupported values fail reconciliation before updating the DaemonSet. No Cluster API or CRD fields are added.

Operator Helm values configure these Deployment environment variables:

```yaml
localStorage:
  migrate:
    check: true
    juicesync:
      imageRepository: hwameistor/hwameistor-juicesync
      tag: v1.2.0-v1.4.1-01
      version: "1.2.0"
```

To select the other bundled binary without rebuilding the image, set `localStorage.migrate.juicesync.version` to `"1.4.1"` in your Helm values. For a deployment managed without Helm, set the Operator environment directly:

```sh
kubectl -n <operator-namespace> set env deployment/hwameistor-operator JUICEFS_VERSION=1.4.1
# Optional: explicitly disable copied-content verification.
kubectl -n <operator-namespace> set env deployment/hwameistor-operator MIGRATE_CHECK=false
```

Operator environment variables provide defaults for its managed Clusters. The existing `localStorage.member.extraEnv` Helm value (stored in the `hwameistor.io/local-storage-member-extra-env` Cluster annotation) can override `JUICEFS_VERSION` for an individual Cluster. The last version entry wins; removing it restores the Operator default. `MIGRATE_CHECK` is configured on the Operator Deployment. Referenced version values (`valueFrom`) are resolved and validated by Local Storage when creating a Job. Changing the Deployment environment restarts Operator; reconciliation then updates the Local Storage DaemonSet. Edit Operator configuration rather than its managed DaemonSet, whose migration settings are reconciled. The existing `spec.localStorage.member.juicesyncImage` field still controls the image reference and preserves custom values. The image tag is a release identifier, not a binary selector.

Coordinate the Operator, Local Storage and migration-image releases; no CRD upgrade is required for this change. Local Storage must be upgraded to forward `JUICEFS_VERSION` into new migration Jobs. The image release target is `hwameistor/hwameistor-juicesync:v1.2.0-v1.4.1-01`; this source change does not publish it. Existing Jobs and their retry Pods keep their original configuration. Wait for the Local Storage rollout to finish before creating a new migration Job.

Verification uses `--check-all` to check copied files and existing files that would otherwise be skipped based on size and modification time, alongside the existing `--links --dirs --perms --force-update` flags. Empty or unset environment variables select defaults; explicit `MIGRATE_CHECK=false` disables verification. Version 1.2.0 fails on tested special files. Version 1.4.1 skips FIFO, socket and device nodes with warnings, even with checks enabled; it must be explicitly selected and is never an automatic failure fallback. Content checks do not preserve hardlink relationships or xattrs/ACLs and do not establish complete rootfs fidelity or a consistent snapshot of an actively written source.
