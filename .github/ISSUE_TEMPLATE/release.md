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

- [ ] Create a new `release-notes-v0.4x.0` branch and add a new release notes file using the available [template](/grafana/k6/tree/master/release%20notes/template.md) to the [repository's `release notes` folder](/grafana/k6/tree/master/release%20notes).
- [ ] Go through the potential [dependencies updates](Dependencies.md) and create a dedicated PR if any of them is relevant to this release.

### Release Preparation

#### ~ 1 week before the release date.

- [ ] Ensure all PRs in the k6-docs repository, related to new or modified functionalities introduced by the new version have been created.
- [ ] Ensure all PRs in the k6 repository, part of the current [milestone](https://github.com/grafana/k6/milestones), have been merged.
- [ ] Open a PR with the release notes for the new version, and ask teams who might have contributed to the release (@k6-browser, @k6-chaos, @devrel teams, etc.) to contribute their notes and review the existing ones.
- [ ] Share the release notes PR with the k6 open-source teams. Request contributions from all affected teams (browser, chaos, devrel, docs, etc.) and any other stakeholders involved in the new release.
- [ ] Open a separate PR for bumping [the k6 Go project's version](https://github.com/grafana/k6/blob/9fa50b2d1f259cdccff5cc7bc18a236d31c345ac/lib/consts/consts.go#L11).
- [ ] Create a dedicated branch for the upcoming version in the grafana/k6-DefinitelyTyped fork repository.
- [ ] Open a PR in the DefinitelyTyped/DefinitelyTyped repository, using the branch created in the grafana/k6-DefinitelyTyped fork, to update the k6 type definitions for the new release.


#### ~ 1 day before the release date.

- [ ] Open a PR in the k6-docs repository, archiving the current k6's JavaScript API version as per the following [instructions](https://github.com/grafana/k6-docs/wiki/Add-version-for-Javascript-API-documentation).
- [ ] Ensure the [existing k6-docs PRs](https://github.com/grafana/k6-docs/pulls), related to the new functionalities and changes, are reviewed, up to date with the latest state of the `master` branch, and based upon the branch containing the k6 archived JavaScript API documentation (as created in the previous step).

### Release Day

#### Documentation

- [ ] Merge the k6-docs repository's Javascript API archiving PR and rebase the rest of the branches meant for the release on top of the new state of the `master` branch.
- [ ] Merge all the k6-docs repository's branches containing changes related to the release.
- [ ] Ensure the last resulting k6-docs GitHub action targetting the `main` branch sees its "Check broken links" job pass.
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
