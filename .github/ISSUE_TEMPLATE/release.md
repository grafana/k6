---
name: k6 Release
about: k6 release accommodates activities and a checklist with the k6 open-source release process.
title: 'k6 release v0.5x.0'
labels: ["release"]
---

**Release Date**:

25th May 2024 <<  TODO: WRITE HERE THE UPDATED RELEASE DATE

## Release Activities

### At the beginning of the cycle

- [ ] Obtain the Release coordinator's contact from the `@k6-browser` team and co-assign the issue to them.
- [ ] Create a new `release-v0.5x.0` long-lived branch and add a new release notes file using the available [template](/grafana/k6/tree/master/release%20notes/template.md) to the [repository's `release notes` folder](/grafana/k6/tree/master/release%20notes).
- [ ] Go through the potential [dependencies updates](Dependencies.md) and create a dedicated PR if any of them is relevant to this release.
- [ ] Create a new `release-v0.5x.0` long-lived branch on the [grafana/k6-DefinitelyTyped](https://github.com/grafana/k6-DefinitelyTyped) fork repository.
    - [ ] Bump the version in [types/k6/package.json](https://github.com/grafana/k6-DefinitelyTyped/blob/master/types/k6/package.json#L4) to the next one.

### Release Preparation

#### ~ 2 weeks before the release date

- [ ] Ensure that all pull-requests from a release milestone are merged.
- [ ] Ensure that browser and other experimental modules (if needed) have been updated to their latest version.

#### ~ 1 week before the release date

- [ ] Ensure that all merged pull-requests referenced in the release notes are linked to the release milestone.
- [ ] Ensure all pull-requests in the `k6-docs` repository, related to new or modified functionalities introduced by the new version have been created.
- [ ] Ensure all PRs in the k6 repository, part of the current [milestone](https://github.com/grafana/k6/milestones), have been merged.
- [ ] Open a PR with the release notes for the new version, and ask teams who might have contributed to the release (k6-browser, k6-ecosystem, k6-docs, k6-devrel teams, etc.) to contribute their notes and review the existing ones.
- [ ] Share the release notes PR with the k6 open-source teams. Request contributions from all affected teams (k6-browser, k6-chaos, k6-docs, k6-devrel, etc.) and any other stakeholders involved in the new release.
- [ ] Open a separate pull-request for bumping [the k6 Go project's version](https://github.com/grafana/k6/blob/master/lib/consts/consts.go#L11).
- [ ] Open a PR in the `DefinitelyTyped/DefinitelyTyped` repository, using the release branch created in the grafana/k6-DefinitelyTyped fork, to update the k6 type definitions for the new release.

#### ~ 1 day before the release date

- [ ] Ensure all pull-requests in the `k6-docs` repository, related to new or modified functionalities introduced by the new version have been merged.

### Release Day

#### Documentation

- [ ] Open and merge a pull-request from `main` in the `k6-docs` repository, copying the current k6's `next` to a folder named with the k6 version (e.g. `v0.48.x`).
- [ ] Ensure the k6 repository release notes PR contains the correct links to the docs.

#### In k6 repository

- [ ] Merge the PR bumping [the k6 Go project's version](https://github.com/grafana/k6/blob/9fa50b2d1f259cdccff5cc7bc18a236d31c345ac/lib/consts/consts.go#L11).
- [ ] Merge the release notes PR.
- [ ] Create and push a new tag of the form `vX.Y.Z` using git: `git tag v0.5x.0 -m "v0.5x.0"`.

#### Announcements

- [ ] Publish a link to the new GitHub release in the #k6-changelog channel.
- [ ] Notify the larger team in the #k6 channel, letting them know that the release is published.
- [ ] Close the release's milestone.

## Wrapping Release

- [ ] Ensure the `DefinitelyTyped/DefinitelyTyped` PR(s) are merged.
- [ ] Update the k6 repository's `.github/ISSUE_TEMPLATE/release.md` in the event steps from this checklist were incorrect or missing.
