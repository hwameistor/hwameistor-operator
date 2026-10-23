# Release Process

This is the process to follow to make a new release. Similar to [Semantic](https://semver.org/) Version. 
The project refers to the respective components of this triple as < major >.< minor >.< patch >.

The Hwameistor Operator Project's release process is as follows:
- [Minor Release Process](#Minor)
- [Patch Release Process](#Patch)
- [Backporting fixes](#backporting-fixes)

<span id="Minor"> </span>
## Minor Release Process

A minor release is e.g. v0.1.0.

A minor release requires 3 steps are as follows: 

### Before release

1. Ensure all issues of `priority/critical-urgent` or `priority/important-soon` label have been resolved.
2. Ensure all PRs about feature which should be included in this version have been merged.  
3. Check the latest PeriodCheck based on main branch has passed.
4. SmokeTest (optional)

### Core release

1. An issue is proposing a new release with a changelog since the latest release.
2. Check out master, ensure it's upto date, and ensure you have a clean working directory.
   1. Update `CHANGELOG.md` and create changelog under `changelogs/released/<version>` directory
3. Create a new local release branch from master.
4. Edit file `helm/operator/Chart.yaml`
   1. Modify version field to release version 
   2. Modify appVersion field to release version
   > NOTE: version and appVersion are consistent by default
5. Commit all changes, push the branch, and PR it into master.

### Post release

1. Checkout release packages(includes helm packages, images) are generated correctly.
2. Write release notes to explain the changes of this release corresponding to the changelog.

<span id="Patch"> </span>
## Patch Release Process

A minor release is e.g. v0.1.1.

A patch release requires 3 steps are as follows:

### Before release

1. Ensure all issues of `priority/critical-urgent` or `priority/important-soon` label have been resolved.
2. Check the latest PeriodCheck based on main branch has passed.
3. SmokeTest (optional)

### Core release

1. An issue is proposing a new release with a changelog since the latest release.
2. Check out master, ensure it's upto date, and ensure you have a clean working directory.
   1. Update `CHANGELOG.md` and create changelog under `changelogs/released/<version>` directory
3. Create a new local release branch from master.
4. Edit file `helm/operator/Chart.yaml`
   1. Modify version field to release version
   2. Modify appVersion field to release version
   > NOTE: version and appVersion are consistent by default
5. Commit all changes, push the branch, and PR it into master.

### Post release

1. Checkout release packages(includes helm packages, images) are generated correctly.
2. Write release notes to explain the changes of this release corresponding to the changelog.

## Backporting fixes

Maintenance branches use `release/vX.Y` and track the corresponding HwameiStor
major/minor version. `release/v1.0` starts from Operator `v1.0.2`, and
`release/v1.1` starts from Operator `v1.1.1`.

Apply the following labels to a pull request targeting `main`:

| Label | Target branch |
| --- | --- |
| `cherry-pick-v1.0` | `release/v1.0` |
| `cherry-pick-v1.1` | `release/v1.1` |
| `cherry-pick-v1` (legacy alias) | `release/v1.0` |

After the pull request is merged, the `Cherry pick` workflow opens one backport
pull request per selected target. Labels can also be added after merging. Adding
both version labels selects both branches; the legacy alias does not create a
second v1.0 backport. Backport pull requests are not merged automatically.

The workflow reads current labels and skips targets that already have an open or
merged backport. Concurrent runs for the same source pull request are serialized.
Conflicts fail the affected target without committing conflict markers; inspect
the action summary before preparing a manual backport. Start a manual backport
description with `Backport of #<source PR number>` so it can be recognized on retries.

To retry, run `Cherry pick` from the Actions tab with the workflow branch set to
`main` and `pull_number` set to the merged source pull request. The source pull
request's current labels select the targets. Closed, unmerged backport pull
requests do not prevent a retry.

Repository administrators must configure `BACKPORT_TOKEN` (or the existing
`MY_TOKEN` alias) as a repository secret. Use a PAT or GitHub App token with access
to this repository and write permission for contents and pull requests; backports
that change workflows also need permission to update workflow files. The default
`GITHUB_TOKEN` is not used to create backports because its pull request events do
not start normal PR CI. A missing token fails before creating backport branches.

The `pr.yml` and `release-image.yaml` branch filters must include the maintained
release branches on each branch itself. A change to the workflows on `main`
alone does not update an existing release branch. The image/chart release
workflow runs when `helm/operator/Chart.yaml` changes on a supported branch or
when manually dispatched; adding a backport label does not publish a release.
