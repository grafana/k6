# Contributing to k6

Thank you for your interest in contributing to k6!

(ﾉ◕ヮ◕)ﾉ*:・ﾟ✧

Before you begin, make sure to familiarize yourself with the [Code of Conduct](CODE_OF_CONDUCT.md). If you've previously contributed to other open source project, you may recognize it as the classic [Contributor Covenant](https://contributor-covenant.org/).

If you want to chat with the team or the community, you can [join our community forums](https://community.k6.io/).

> **Note:** To disclose security issues, refer to [SECURITY.md](https://github.com/grafana/k6?tab=security-ov-file).

## Filing issues

Don't be afraid to file issues! Nobody can fix a bug we don't know exists, or add a feature we didn't think of.

The worst that can happen is that someone closes it and points you in the right direction.

That said, "how do I..."-type questions are often more suited for community forums.

## Contributing code

If you'd like to contribute code to k6, this is the basic procedure. Make sure to follow the [style guide](#code-style) described below.

1. Find an issue you'd like to fix. If there is none already, or you'd like to add a feature, please open one, and we can talk about how to do it.  Out of respect for your time, please start a discussion regarding any bigger contributions either in a GitHub Issue, in the community forums **before** you get started on the implementation.
  
   Remember, there's more to software development than code; if it's not properly planned, stuff gets messy real fast.

2. Create a fork and open a feature branch - `feature/my-cool-feature` is the classic way to name these, but it really doesn't matter.

3. Create a pull request!

4. Sign the [Contributor License Agreement](https://cla-assistant.io/grafana/k6) (the process is integrated with the pull request flow through cla-assistant.io).

5. We will discuss implementation details until everyone is happy, then a maintainer will merge it.

### Development setup

To get a basic development environment for Go and k6 up and running, first make sure you have **[Git](https://git-scm.com/downloads)** and **[Go](https://golang.org/doc/install)** (see our [go.mod](https://github.com/grafana/k6/blob/master/go.mod#L3) for minimum required version) installed and working properly.

We recommend using the Git command-line interface to download the source code for the k6:

* Open a terminal and run `git clone https://github.com/grafana/k6.git`. This command downloads k6's sources to a new `k6` directory in your current directory.
* Open the `k6` directory in your favorite code editor.

For alternative ways of cloning the k6 repository, please refer to [GitHub's cloning a repository](https://docs.github.com/en/github/creating-cloning-and-archiving-repositories/cloning-a-repository) documentation.

#### Running the linter

We make use of the [golangci-lint](https://github.com/golangci/golangci-lint) tool to lint the code in CI. The base configuration, including the golangci-lint version, lives in [grafana/k6-ci](https://github.com/grafana/k6-ci), and k6-specific changes to it are kept in [`.golangci.patch`](https://github.com/grafana/k6/blob/master/.golangci.patch). To run it locally with the same version and configuration as CI, run:

```bash
make lint
```

There is no need to install golangci-lint yourself. `make lint` downloads the configuration into `build/lint/`, applies the patch and runs the matching golangci-lint version with `go run`.

If you need to change the linter configuration, edit `build/lint/.golangci.yml` after running `make lint`, then regenerate the patch with:

```bash
make update-lint-patch
```

#### Running the test suite

To exercise the entire test suite, please run the following command

```bash
make tests
```

#### Dependencies

Consult [Dependencies.md](Dependencies.md) to find out more about how we manage k6's dependencies, and our policy regarding dependencies management and update.

#### Code style

As you'd expect, please adhere to good ol' `gofmt` (there are plugins for most editors that can autocorrect this), but also `gofmt -s` (code simplification), and don't leave unused functions laying around.

Continuous integration will catch all of this if you don't, and it's fine to just fix linter complaints with another commit, but you can also run the linter yourself:

```bash
make check
```

Comments in the source should wrap at 100 characters, but there's no maximum length or need to be brief here - please include anything one might need to know in order to understand the code, that you could reasonably expect any reader to not already know (you probably don't need to explain what a goroutine is).

#### Commit format

We don't have any explicit rules about commit message formatting, but try to write something that could be included as-is in a changelog.

If your commit closes an issue, please [close it with your commit message](https://help.github.com/articles/closing-issues-via-commit-messages/), for example:

```text
Added this really rad feature

Closes #420
```

#### Language and text formatting

Any human-readable text you add must be non-gendered and should be fairly concise without devolving into grammatical horrors, dropped words, and shorthands. This isn't Twitter, you don't have a character cap, but don't write a novel where a single sentence would suffice.

If you're writing a longer block of text to a terminal, wrap it at 80 characters - this ensures it will display properly at the de facto default terminal size of 80x25. As an example, this is the help text of the `scale` sub-command:

```text
   Scale will change the number of active VUs of a running test.

   It is an error to scale a test beyond vus-max; this is because instantiating
   new VUs is a very expensive operation, which may skew test results if done
   during a running test. To raise vus-max, use --max/-m.

   Endpoint: /v1/status
```

### Backporting changes (maintainers)

To request a backport from `master`, add a `backport <branch>` label to the original pull request;
for example, `backport v1.x` targets the previous major's maintenance branch. You can request multiple
destinations by adding a label for each branch. On merge, the automation processes all backport labels.
Adding a label after the pull request has merged requests only that label's destination.
Use **Squash and merge** for changes that need automatic backports. The automation opens a separate
pull request against each destination; review the changes and wait for its CI checks before merging it.

The [trigger workflow](.github/workflows/backport-trigger.yml) starts the
[backport workflow](.github/workflows/backport.yml). If a backport fails, inspect the workflow run
and any comment on the original pull request. Resolve conflicts manually on a new branch based on
the destination branch, cherry-pick the original change, and open a pull request against it that links
to the original pull request. A clean cherry-pick still needs review: an older major may need different APIs
or dependency versions.

Before retrying a failed run or reapplying the label, check whether a backport pull request already
exists. The shared action does not reliably skip backports it has already created.

The automatic path supports squash commits and regular, non-executable files. Changes involving
merge commits, executable files, symlinks, or submodules need a manual backport.
The worker also rejects requests when it cannot match the original PR to the triggering workflow
run, for example if the source fork has been deleted. Backport those changes manually.

#### Automation setup

Create a `backport <branch>` repository label for each intended destination and verify that the branch
exists; initially, use `backport v1.x` for `v1.x`. Verify that the `k6-core-bot` GitHub App is installed
on this repository and has permission to write repository contents and pull requests, plus workflows
permission when backporting changes under `.github/workflows/`. The app token must be allowed to
create the backport branch and pull request under the repository's rules.

The workflows use Grafana's shared GitHub App token action. Verify the app configuration and Vault
policy for `.github/workflows/backport.yml` on `master` with the `workflow_run` event, including its
OIDC identity and `id-token: write` permission.
The existing Renovate approval integration does not establish that these permissions or policy are
already configured for backporting. Use an app token so that generated pull requests trigger CI.

#### Moving to the next maintenance major

When `master` moves to v3 and v2 becomes the previous major, create the `v2.x` maintenance branch
and `backport v2.x` label. The backport workflows use the destination from each label, so adding a
destination does not require editing them. Review the CI push branch filters, which currently name
`v1.x`, and the new branch's protection rules. Keep the source branch as `master` while it remains
the development branch. Use explicit version labels so that a request made for v1 cannot silently
become a request for v2.
