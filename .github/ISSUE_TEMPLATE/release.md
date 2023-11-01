---
name: k6 Release
about: k6 release accommodates activities and a checklist with the k6 open-source release process.
title: 'k6 release v0.4x.0'
labels: ["release"]
---

**Release Date**:

25th May 2023 <<  TODO: WRITE HERE THE UPDATED RELEASE DATE

## Release Activities

### At the beginning of the cycle

- [ ] Obtain the Release coordinator's contact from the `@k6-browser` team and co-assign the issue to them.
- [ ] Create a new `release-v0.4x.0` long-lived branch and add a new release notes file using the available [template](/grafana/k6/tree/master/release%20notes/template.md) to the [repository's `release notes` folder](/grafana/k6/tree/master/release%20notes).
- [ ] Go through the potential [dependencies updates](Dependencies.md) and create a dedicated PR if any of them is relevant to this release.
- [ ] Create a new `release-v0.4x.0` long-lived branch on the [k6-docs](https://github.com/grafana/k6-docs) repository. Try to keep it synchronized with `main` throughout the development cycle to prevent it from becoming outdated and to avoid numerous conflicts.
- [ ] Create a new `release-v0.4x.0` long-lived branch on the [grafana/k6-DefinitelyTyped](https://github.com/grafana/k6-DefinitelyTyped) fork repository.

### Release Preparation

#### ~ 1 week before the release date.

- [ ] Ensure all PRs in the `k6-docs` repository, related to new or modified functionalities introduced by the new version have been created and merged to the release branch.
- [ ] Ensure all PRs in the k6 repository, part of the current [milestone](https://github.com/grafana/k6/milestones), have been merged.
- [ ] Open a PR with the release notes for the new version, and ask teams who might have contributed to the release (k6-browser, k6-chaos, k6-docs, k6-devrel teams, etc.) to contribute their notes and review the existing ones.
- [ ] Share the release notes PR with the k6 open-source teams. Request contributions from all affected teams (k6-browser, k6-chaos, k6-docs, k6-devrel, etc.) and any other stakeholders involved in the new release.
- [ ] Open a separate PR for bumping [the k6 Go project's version](https://github.com/grafana/k6/blob/9fa50b2d1f259cdccff5cc7bc18a236d31c345ac/lib/consts/consts.go#L11).
- [ ] Open a PR in the `DefinitelyTyped/DefinitelyTyped` repository, using the release branch created in the grafana/k6-DefinitelyTyped fork, to update the k6 type definitions for the new release.

#### ~ 1 day before the release date.

- [ ] Open a PR from `main` in the `k6-docs` repository, archiving the current k6's JavaScript API version as per the following [instructions](https://github.com/grafana/k6-docs/wiki/Add-version-for-Javascript-API-documentation).
- [ ] Open a PR in `k6-docs` repository for the release branch containing all the merged PRs for the upcoming version. If any, resolve the conflicts with the `main` branch.

### Release Day

#### Documentation

- [ ] Merge the `k6-docs` repository's JavaScript API archiving PR and rebase the rest of the branches meant for the release on top of the new state of the `main` branch.
- [ ] Merge the release PR for `k6-docs` containing all the changes related to the release. The order in which this PR is merged relative to the previous one is crucial; otherwise, the new changes might inadvertently be incorporated into previous versions.
- [ ] Ensure the last resulting k6-docs GitHub action targeting the `main` branch sees its "Check broken links" job pass.
- [ ] Publish the new [vX.Y.Z version of docs](https://github.com/grafana/k6-docs/releases/new).
- [ ] Ensure the k6 repository release notes PR contains the correct links to the docs.

#### In k6 repository

- [ ] Merge the PR bumping [the k6 Go project's version](https://github.com/grafana/k6/blob/9fa50b2d1f259cdccff5cc7bc18a236d31c345ac/lib/consts/consts.go#L11).
- [ ] Merge the release notes PR.
- [ ] Create and push a new tag of the form `vX.Y.Z` using git: `git tag v0.4x.0 -m "v0.4x.0"`.

#### Announcements

- [ ] Publish a link to the new GitHub release in the #k6-changelog channel.
- [ ] Notify the larger team in the #k6 channel, letting them know that the release is published.
- [ ] Close the release's milestone.

## Wrapping Release

- [ ] Ensure the `DefinitelyTyped/DefinitelyTyped` PR(s) are merged.
- [ ] Update the k6 repository's `.github/ISSUE_TEMPLATE/release.md` in the event steps from this checklist were incorrect or missing.
