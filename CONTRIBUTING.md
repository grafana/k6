Contributing to k6
==================

Thank you for your interest in contributing to k6!

(ﾉ◕ヮ◕)ﾉ*:・ﾟ✧

Before you begin, make sure to familiarize yourself with the [Code of Conduct](CODE_OF_CONDUCT.md). If you've previously contributed to other open source project, you may recognize it as the classic [contributor covenant](http://contributor-covenant.org/).

If you want to chat with the team or the community, you can join our Slack team: [LINK GOES HERE].

Filing issues
-------------

Don't be afraid to file issues! Nobody can fix a bug we don't know exists, or add a feature we didn't think of.

The worst that can happen is that someone closes it right away and points you in the right direction.  

That said, "how do I..."-type questions are often more suited for Slack.

Contributing code
-----------------

If you'd like to contribute code to k6, this is the basic procedure. Make sure to follow the **style guide** described below!

1. Find an issue you'd like to fix. If there is none already, or you'd like to add a feature, please open one and we can talk about how to do it.
   
   Remember, there's more to software development than code; if it's not properly planned, stuff gets messy real fast.

2. Create a fork and open a feature branch based off develop - `feature/my-cool-feature` is the classic way to name these, but it really doesn't matter, as long as you don't hack directly on develop.

3. Create a pull request - make sure you make it from your feature branch to develop!

4. We well discuss implementation details until it's polished and perfect, then a maintainer will merge it.

We use [git flow](http://nvie.com/posts/a-successful-git-branching-model/), so you may recognize our branching structure from, well, every other project that does this. If not, have a look at that post and you'll feel right at home in no time.

Style guide
-----------

In order to keep the project manageable, consistency is very important. Most of this is enforced automatically by various bots.

**Code style**

As you'd expect, please adhere to good ol' `gofmt` (there are plugins for most editors that can autocorrect this), but also `gofmt -s` (code simplification), and don't leave unused functions laying around.

Continous integration will catch all of this if you don't, and it's fine to just fix linter complaints with another commit, but you can also run the linter yourself:

```
gometalinter --config gometalinter.json --deadline 10m ./...
```

**Commit format**

In order to keep the changelog easy to read, all commits must have one of the following prefixes:

* `[feat]` - new features
* `[fix]` - bug fixes
* `[change]` - changed behavior
* `[removed]` - something was removed
* `[refactor]` - nothing added or removed
* `[lint]` - fixed linter complaints
* `[test]` - added or improved tests
* `[test/fix]` - fixed broken tests
* `[docs]` - docs and sample code
* `[docs/fix]` - fixed doc or sample errors

If your commit closes an issue, please [close it with your commit message](https://help.github.com/articles/closing-issues-via-commit-messages/), for example:

```
[feat] Added this really rad feature

Closes #420
```

**Language and text formatting**

Any human-readable text you add must be non-gendered (if applicable), and should be fairly concise without devolving into grammatical horrors, dropped words and shorthands. This isn't Twitter, but don't write a novel where a single sentence would suffice.

If you're writing a longer block of text to a terminal, wrap it at 80 characters.

**License**

If you make a new source file, you must copy the license preamble from an existing file to the top of it. We can't merge a PR with unlicensed source files.

This doesn't apply to documentation or sample code; only files that make up the application itself.
