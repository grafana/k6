---
name: k6 Release
about: k6 release accommodates activities and a checklist with the k6 open-source release process.
title: 'k6 release v0.5x.0'
labels: ["release"]
---

**Release Date**:

25th May 2025 **<- WRITE HERE THE UPDATED RELEASE DATE**

## Release Activities

### At the beginning of the cycle

- [ ] Create a new `release-v{major}.{minor}.0` branch.
    - [ ] Add a new release notes file using the available [template](https://github.com/grafana/k6/blob/master/release%20notes/template.md) to the [repository's `release notes` folder](https://github.com/grafana/k6/blob/master/release%20notes).
    - [ ] Go through the potential [dependencies updates](https://github.com/grafana/k6/blob/master/Dependencies.md) and create a dedicated PR if any of them is relevant to this release.
- [ ] Create a new `release-v{major}.{minor}.0` branch on the [grafana/k6-DefinitelyTyped](https://github.com/grafana/k6-DefinitelyTyped) fork repository.
    - [ ] Bump the version in [types/k6/package.json](https://github.com/grafana/k6-DefinitelyTyped/blob/master/types/k6/package.json#L4) to the next one.

### Release Preparation

#### ~ 2 weeks before the release date

- [ ] Ensure that all PRs from a release milestone are merged.
- [ ] Ensure experimental modules (if needed) have been updated to their latest version.

#### ~ 1 week before the release date

- [ ] Ensure that all merged PRs referenced in the release notes are linked to the release milestone.
- [ ] Ensure all PRs in the `k6-docs` repository related to new or modified functionalities introduced by the new version have been created.
- [ ] Ensure all PRs in the `k6` repository, part of the current [milestone](https://github.com/grafana/k6/milestones), have been merged.
- [ ] Open a PR with the release notes for the new version
  - [ ] Ask teams that might have contributed to the release (e.g., k6-ecosystem, k6-docs, k6-devrel teams) to contribute their notes and review the existing ones.
  - [ ] Remember to mention and thank [external contributors](https://github.com/search?q=user%3Agrafana+repo%3Ak6+milestone%3A%22v0.51.0%22+-author%3Amstoykov+-author%3Aoleiade+-author%3Ana--+-author%3Acodebien+-author%3Aolegbespalov+-author%3Aandrewslotin+-author%3Ajoanlopez+-author%3Aankur22+-author%3Ainancgumus+-author%3Aszkiba+-author%3Adependabot%5Bbot%5D&type=pullrequests). (**<- Update the query with the correct milestone version**).
- [ ] Share the release notes PR with the k6 open-source teams. Request contributions from all affected teams (k6-chaos, k6-docs, k6-devrel, etc.) and any other stakeholders involved in the new release.
- [ ] Open a separate PR for bumping [the k6 Go project's version](https://github.com/grafana/k6/blob/master/internal/build/version.go#L6).
- [ ] Open a PR in the `DefinitelyTyped/DefinitelyTyped` repository using the release branch created in the grafana/k6-DefinitelyTyped fork to update the k6 type definitions for the new release.

#### ~ 1 day before the release date

- [ ] Ensure all PRs in the `k6-docs` repository related to new or modified functionalities introduced by the new version have been merged.

### Release Day

#### Documentation

- [ ] Open and merge a PR from `main` in the `k6-docs` repository, copying the current k6's `next` to a folder named with the k6 version (e.g., `v0.55.x`).
- [ ] Ensure the `k6` repository release notes PR contains the correct links to the docs.

#### In k6 repository

- [ ] Merge the PR bumping [the k6 Go project's version](https://github.com/grafana/k6/blob/master/lib/consts/consts.go#L11-L12).
- [ ] Merge the release notes PR.
- [ ] Pull locally the previously merged changes.
- [ ] Create a new long-lived `v{major}.{minor}.x` release branch from the `main` branch.
- [ ] Checkout the new `v{major}.{minor}.x` release branch, create and push a new tag of the form `v{major}.{minor}.0` using git: _e.g._ `git tag v0.55.0 -m "v0.55.0"`.

#### Announcements

- [ ] Publish a link to the new GitHub release in the #k6-changelog channel.
- [ ] Notify the larger team in the #k6 channel, letting them know that the release is published.
- [ ] Close the release's milestone.

## Wrapping Release

- [ ] Ensure the `DefinitelyTyped/DefinitelyTyped` PR(s) are merged.
- [ ] Update the k6 repository's `.github/ISSUE_TEMPLATE/release.md` in the event steps from this checklist were incorrect or missing.
