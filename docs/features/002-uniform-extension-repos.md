# Uniform repository setup for Grafana-owned k6 extensions

## Problem

Grafana owns around thirty k6 extension repositories, and no two are set up the same way.
A maintainer moving between them has to relearn how CI is defined, which build commands
exist, how to build the extension locally, and how a release is cut; some repos have no
linting at all, and half have no release automation. Changing a repo-owned convention
means a coordinated sweep, and advancing a pinned shared workflow needs an update in every
caller, so the repos sit on several versions of it at once.

## Significance

A handful of people maintain all of these repos and move between them constantly, so
every difference costs time again and again.

- **A standard already exists and almost nothing enforces it.** xk6 defines a compliance
  preset for official Grafana extensions, covering the README, examples, type definitions,
  and code ownership. Two repos check against it in CI. Ten more can run the check locally
  but never do so automatically, so drift usually goes unreported.
- **CI is half-shared and not reproducible.** Just over half call the shared CI workflow,
  and those that do sit on two different versions of it. Almost none pin the lint
  configuration to match, so the linter silently follows a moving default while the repo
  looks pinned. The rest still have hand-written pipelines, the oldest years out of date.
- **Linting is uneven, and local development does not match CI.** Several repos run no
  linter. More than a third have no `lint` task, so reproducing a CI failure can mean
  reading the workflow and rebuilding the command by hand. Three incompatible Makefile
  families are in use.
- **Releasing is inconsistent, and missing from half.** About half call a shared release
  workflow, and a couple have their own. The rest have none, so cutting a version depends
  on whoever does it remembering the steps, and what a release produces differs by repo.
- **Documentation is missing, inconsistent, or unmaintainable.** Fewer than half the repos
  have a `CONTRIBUTING.md`, and each one runs to hundreds of lines whose supposedly
  identical sections have already been reworded repo by repo. Several different code of
  conduct texts are in use, and a third of repos have none. READMEs cover different
  things, so how to install an extension is in a different place, or nowhere.
- **No extension documents which k6 versions it supports.** Twenty-four repos depend
  directly on k6, at five different versions, one a pre-release commit from 2024. A
  dependency is not a support policy, so a user cannot tell whether the k6 they run is
  covered.
- **Some settings cannot be checked from the repo at all.** Only one repo has issue or pull
  request templates, and labels and branch protection exist only in GitHub, so nothing in
  the repo defines them.
- **Tooling the maintainers do not use still costs.** Many repos carry a development
  container pinning many tool versions. The core team does not use them, but Renovate
  still raises pull requests needing review.

External contributors feel this most. What guidance they find, and where, depends on which
repo they landed in. Users are affected too, less visibly: the
checks that would catch a broken build, a failing test, or a known vulnerability run
unevenly, so how well an extension is verified depends on which one they picked.

## Cost of inaction

Repository structure does not converge on its own. The variants that exist stay, and every
standard the repos own keeps needing a coordinated update across all of them. Renovate
moves dependencies forward, but one repo at a time: the same upgrade produces a separate
update in each repository, and lands at a different moment in each.

## Desired state

A maintainer or contributor who knows one Grafana-owned k6 extension knows all of them.

- Every extension is verified to the same standard before anything ships: the same
  dependency, lint, test, and build checks. Each repo shows which version of the standard
  verified it, and moving to a new version is a mechanical, reviewable update, not per-repo
  design work.
- Any extension can be linted, tested, and built locally with the same commands, and the
  results match what CI reports. Every repo states what it needs to build.
- Releasing is predictable and repeatable everywhere, with nothing depending on
  undocumented steps. A release publishes what consumers use and nothing they do not.
- Contributor and user documentation is consistent and self-contained, and shared changes
  reach every repo through the same mechanical update.
- Conformance pays for itself. A repo that keeps to the standard takes later improvements
  as a routine update, and a repo that diverges visibly gives that up.
- There is one definition of a conforming extension repository, and one place to find out
  whether a repo meets it.
- Every tool that stays supports a documented workflow whose benefit is worth its upkeep.

Success is measurable: a change to the standard reaches every repository as one
reviewed change plus a mechanical update, and a new extension is uniform from its first
commit.

## Out of scope

Whether any given repository keeps its development container is not decided here. The
brief only asks that a tool which stays serve a documented workflow. Whether the standard
extends to community extensions is a separate question.

---

*Filed here because the brief process lives in this repository. It should move to
`grafana/xk6` once that repo has a `docs/features/` folder.*
