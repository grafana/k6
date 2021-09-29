Contributing to k6
==================

Thank you for your interest in contributing to k6!

(ﾉ◕ヮ◕)ﾉ*:・ﾟ✧

Before you begin, make sure to familiarize yourself with the [Code of Conduct](CODE_OF_CONDUCT.md). If you've previously contributed to other open source project, you may recognize it as the classic [Contributor Covenant](https://contributor-covenant.org/).

If you want to chat with the team or the community, you can [join our Slack team](https://k6.io/slack/).

Filing issues
-------------

Don't be afraid to file issues! Nobody can fix a bug we don't know exists, or add a feature we didn't think of.

The worst that can happen is that someone closes it and points you in the right direction.

That said, "how do I..."-type questions are often more suited for Slack.

Contributing code
-----------------

If you'd like to contribute code to k6, this is the basic procedure. Make sure to follow the [style guide](#style-guide) described below.

1. Find an issue you'd like to fix. If there is none already, or you'd like to add a feature, please open one and we can talk about how to do it.  Out of respect for your time, please start a discussion regarding any bigger contributions either in a GitHub Issue, in the community forums or in the `#contributors` channel of the k6 slack **before** you get started on the implementation.
  
   
   Remember, there's more to software development than code; if it's not properly planned, stuff gets messy real fast. 

2. Create a fork and open a feature branch - `feature/my-cool-feature` is the classic way to name these, but it really doesn't matter.

3. Create a pull request!

4. Sign the [Contributor License Agreement](https://cla-assistant.io/loadimpact/k6) (the process is integrated with the pull request flow through cla-assistant.io)

5. We will discuss implementation details until everyone is happy, then a maintainer will merge it.

Development setup
-----------------

To get a basic development environment for Go and k6 up and running, first make sure you have **[Git](https://git-scm.com/downloads)** and **[Go](https://golang.org/doc/install)** (1.10 or newer) installed and working properly.

Once that's done, you can get the k6 source into your Go workspace (`$GOPATH/src`) by running:
```bash
go get go.k6.io/k6
```
This will also build a `k6` binary and put it in `$GOPATH/bin`.

**Building from source**:

Standing in the repo root (`$GOPATH/src/go.k6.io/k6`) you can build a k6 binary from source by running:
```bash
cd $GOPATH/src/go.k6.io/k6
go build
```

**Running the linter**:

We make use of the [golangci-lint](https://github.com/golangci/golangci-lint) tool to lint the code in CI. To run it locally, first install it:
```bash
go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
```
then run:
```
golangci-lint run --out-format=tab --new-from-rev master ./...
```

If you've added new dependencies you might also want to check and make sure all dependencies exists in `vendor/` folder by running:
```bash
go get -u github.com/FiloSottile/vendorcheck
vendorcheck ./...
```

**Running the test suite**:

To exercise the entire test suite:
```bash
go test -race ./...
```

To run the tests of a specific package:
```bash
go test -race go.k6.io/k6/core
```

To run just a specific test case use `-run` and pass in a regex that matches the name of the test:
```bash
go test -race ./... -run ^TestEngineRun$
```

Combining the two above we can run a specific test case in a specific package:
```bash
go test -race go.k6.io/k6/core -run ^TestEngineRun$
```

Style guide
-----------

In order to keep the project manageable, consistency is very important. Most of this is enforced automatically by various bots.

**Code style**

As you'd expect, please adhere to good ol' `gofmt` (there are plugins for most editors that can autocorrect this), but also `gofmt -s` (code simplification), and don't leave unused functions laying around.

Continuous integration will catch all of this if you don't, and it's fine to just fix linter complaints with another commit, but you can also run the linter yourself:

```
golangci-lint run --out-format=tab --new-from-rev master ./...
```

Comments in the source should wrap at 100 characters, but there's no maximum length or need to be brief here - please include anything one might need to know in order to understand the code, that you could reasonably expect any reader to not already know (you probably don't need to explain what a goroutine is).

**Commit format**

We don't have any explicit rules about commit message formatting, but try to write something that could be included as-is in a changelog.

If your commit closes an issue, please [close it with your commit message](https://help.github.com/articles/closing-issues-via-commit-messages/), for example:

```
Added this really rad feature

Closes #420
```

**Language and text formatting**

Any human-readable text you add must be non-gendered, and should be fairly concise without devolving into grammatical horrors, dropped words and shorthands. This isn't Twitter, you don't have a character cap, but don't write a novel where a single sentence would suffice.

If you're writing a longer block of text to a terminal, wrap it at 80 characters - this ensures it will display properly at the de facto default terminal size of 80x25. As an example, this is the help text of the `scale` subcommand:

```
   Scale will change the number of active VUs of a running test.

   It is an error to scale a test beyond vus-max; this is because instantiating
   new VUs is a very expensive operation, which may skew test results if done
   during a running test. To raise vus-max, use --max/-m.

   Endpoint: /v1/status
```


**License**

If you make a new source file, you must copy the license preamble from an existing file to the top of it. We can't merge a PR with unlicensed source files. We also can't merge PRs unless all authors have signed the [Contributor License Agreement](https://cla-assistant.io/loadimpact/k6).

This doesn't apply to documentation or sample code; only files that make up the application itself.
