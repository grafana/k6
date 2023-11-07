# Updating dependencies in k6

k6 has a not small amount of dependencies, some of which are used more than others which affects how often if at all they should be updated.

Some original discussions can be found in [this issue](https://github.com/grafana/k6/issues/1933).

## General rule

The general rule is to update *direct* dependencies just after the release has been made.

This is mostly so that we keep up with dependencies for which we want new features or bug fixes and can be skipped on case by case basis if there is no bandwidth to be done or any other good reason.

For example some dependencies that were in the middle of big updates were skipped as they didn't add anything of value at the time, but were being updated day after day seemingly fixing bugs added the previous day. As in that case that wasn't a dependency we wanted to be on top off - we skipped it.

Through the rest of the development cycle dependencies shouldn't be updated unless:
1. required for development
2. a particular bug fix/feature is really important


The last part predominantly goes for `golang.org/x/*` and particularly `golang.org/x/net` which usually have updates through the development of Go itself.
[Goja](https://github.com/dop251/goja) has special considerations as it's heavily used and bug fixes there or new features usually have high impact on k6. Which means that we usually try to update it whenever something new lands there.

As the stability of any k6 release is pretty essential, this should be done only when adequate testing can be done, and in general, the changelog for each dependency should be consulted on what has changed.

The latter also serves as a time to open/close issues that are related to the updates. There might be a bug fix for an open issue - we should test it and close the issue. Or there might be new functionality that can be used - probably an issue should be open.

## Go versions

We aim to support a building of the k6 binary with the two latest versions of golang, which reflects the support [policy](https://go.dev/doc/devel/release#policy) of the Go team.

## Exceptions

There are some dependencies that we really don't use all that much, intend on removing and as a general note don't need anything else from them. Given that we currently have no problems that updates will fix - we prefer to not update them as not to introduce bugs. Also, for some they bring additional dependencies that we do not want, which is just one more reason not to update them.

List (as of March 2022):
- github.com/DataDog/datadog-go  - newer versions have a lot more dependencies for functionality we don't need. Also in general a different library is probably going to be better in this case as it only supports UDP and no TCP.
- github.com/andybalholm/cascadia - a dependency of `github.com/PuerkitoBio/goquery`
- github.com/sirupsen/logrus - it's in maintenance mode and we want to remove it - also no update for a long time, but also no bugs.
- github.com/spf13/afero - there are plans to be [replaced by io/fs](https://github.com/grafana/k6/issues/1079) and we don't need anything from it. We have already worked around some bugs so updating might break something
- github.com/spf13/cobra - none of the newer features are particularly needed, but adds a bunch of new dependencies.
- github.com/spf13/pflag - similar to above
- gopkg.in/guregu/null.v3 - no new interesting features and we probably want to drop it in favor of [croconf](https://github.com/grafana/croconf)
- gopkg.in/yaml.v3 - no new features wanted - actually used directly in only one place to output yaml to stdout.

## How to do it

For updating dependencies we recommend to use [modtools](https://github.com/dop251/modtools).

Running `modtools check --direct-only` will give you a list of packages that aren't frozen (the ones above in the exceptions). Alternatively just running `go get <dependency>` for each direct dependency, which also will tell you if there was an update.

Then take a look at the changelog between the versions.

You can use the command `modtools check --direct-only` provided you, to update it. Run tests and if relevant check that bugs are fixed or any other verification that is appropriate.

Commit dependencies one by one with a message like `Update <dependency> from vX.Y.Z to vX.Y.Z` and a relevant changelog for k6. Sometimes that means "nothing of relevance for k6", sometimes it means a list of bug fixes or new features.

It's preferable to make multiple PRs - in most cases you can split them in three:
- update for goja - which usually needs to happen.
- update for `golang.org/x/*` - also again happen literally every release
- everything else - this in general doesn't include more than 5-6 small updates.

Further splitting is recommended if PRs become too big.

When updating goja it's recommended to run the tc39 tests in `js/tc39`. And if needed, update the breaking ones as explained in an [Introduction to a k6's TC39 testing](./js/tc39/README.md).
