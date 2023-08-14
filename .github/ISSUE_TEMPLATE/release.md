---
name: k6 Release 
about: k6 release accommodates activities and a checklist that has the k6 open-source release process.
title: 'k6 release v0.4x.0'
---

**Release Date**: 

25th May 2023 <<  TODO: WRITE HERE THE UPDATED RELEASE DATE

## Release Activities

### At the beginning of the cycle

- [ ] Create a new branch `release-notes-v04x` and add a new related [release notes file](release-template/release%20notes) using the available [template](release-template/release%20notes/template.md).
- [ ] Go through the potential [dependencies updates](Dependencies.md) then create a PR if any is relevant. 

### Release Preparation

~ 1 week before the release date.

- [ ] k6-docs PRs for all new or changed functionality have been created.
- [ ] All PRs to the k6 repository in the current [milestone](https://github.com/grafana/k6/milestones) have been merged.
- [ ] A pull request with the release notes and request the final reviews (including the @k6-browser, devrels folks).
- [ ] Open a separate PR for bumping [the version](https://github.com/grafana/k6/blob/9fa50b2d1f259cdccff5cc7bc18a236d31c345ac/lib/consts/consts.go#L11).
- [ ] The release notes PR shared in the `#k6-oss-dev` internal channel mentioning all the impacted teams (@k6-browser, @k6-chaos, @k6 devrel and any other potential stackholder of the new release).
- [ ] `DefinitelyTyped/DefinitelyTyped` PR(s) is ready.

~ 1 day before the release date.

- [ ] PR for archiving the current k6's JavaScript [API version](https://github.com/grafana/k6-docs/wiki/Add-version-for-Javascript-API-documentation).
- [ ] Check that the [Existing k6-docs PRs](https://github.com/grafana/k6-docs/pulls) related to the new functionality are reviewed and rebased and pointing to the branch with k6's JavaScript API archived.
 
### Release Day

#### Documentation

- [ ] The PR with archiving the old version JS API merged first and rebase the rest on top.
- [ ] PRs with changes related to the release merged.
- [ ] After merging all k6-docs' PRs ensure that we have no broken links by checking "Check broken links" job in GitHub actions. 
- [ ] [The new Docs Release vX.Y.Z](https://github.com/grafana/k6-docs/releases/new) published.
- [ ] Release Notes PR contains the right links to the docs.

#### In k6 repository

- [ ] PR for bumping [the version](https://github.com/grafana/k6/blob/9fa50b2d1f259cdccff5cc7bc18a236d31c345ac/lib/consts/consts.go#L11) merged.
- [ ] Release notes PR merged.
- [ ] A new tag from the CLI `vX.Y.Z` created (`git tag v0.4x.0 -m "v0.4x.0"`) & pushed.

#### Announcements

- [ ] A GitHub's link to the new release published in #k6-changelog.
- [ ] DevRel team is notified in #k6-devrel that release is published.
- [ ] The release's milestone closed.

## Wrapping Release

- [ ] `DefinitelyTyped/DefinitelyTyped` PR(s) merged.
- [ ] Update the k6's `.github/ISSUE_TEMPLATE/release.md` if new repeated steps appear.
