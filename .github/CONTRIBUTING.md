# Contributing to CoreDNS

Welcome! Our community focuses on helping others and making CoreDNS the best it can be. We gladly
accept contributions and encourage you to get involved!

## Bug Reports

First, please [search this
repository](https://github.com/coredns/coredns/search?q=&type=Issues&utf8=%E2%9C%93) with a variety
of keywords to ensure your bug is not already reported.

If not, [open an issue](https://github.com/coredns/coredns/issues) and answer the questions so we
can understand and reproduce the problematic behavior.

The burden is on you to convince us that it is actually a bug in CoreDNS. This is easiest to do when
you write clear, concise instructions so we can reproduce the behavior (even if it seems obvious).
The more detailed and specific you are, the faster we will be able to help you. Check out [How to
Report Bugs Effectively](https://www.chiark.greenend.org.uk/~sgtatham/bugs.html).

Please be kind. :smile: Remember that CoreDNS comes at no cost to you, and you're getting free help.

## Minor Improvements and New Tests

Submit [pull requests](https://github.com/coredns/coredns/pulls) at any time. Make sure to write
tests to assert your change is working properly and is thoroughly covered.

## New Features

First, please [search](https://github.com/coredns/coredns/search?q=&type=Issues&utf8=%E2%9C%93) with
a variety of keywords to ensure your suggestion/proposal is new.

Please also check for existing pull requests to see if someone is already working on this. We want
to avoid duplication of effort.

If the proposal is new and no one has opened pull request yet, you may open either an issue or a
pull request for discussion and feedback.

If you are going to spend significant time implementing code for a pull request, best to open an
issue first and "claim" it and get feedback before you invest a lot of time.

**If someone already opened a pull request, but you think the pull request has stalled and you would
like to open another pull request for the same or similar feature, get some of the maintainers (see
[CODEOWNERS](CODEOWNERS)) involved to resolve the situation and move things forward.**

If possible make a pull request as small as possible, or submit multiple pull request to complete a
feature. Smaller means: easier to understand and review. This in turn means things can be merged
faster.

## New Plugins

A new plugin is (usually) about 1000 lines of Go. This includes tests and some plugin boiler plate.
This is a considerable amount of code and will take time to review. To prevent too much back and
forth it is advisable to start with the plugin's `README.md`; This will be its main documentation
and will help nail down the correct name of the plugin and its various config options.

From there it can work its way through the rest (`setup.go`, the `ServeDNS` handler function, etc.).
Doing this will help the reviewers, as each chunk of code is relatively small.

Also read [plugin.md](https://raw.githubusercontent.com/coredns/coredns/master/plugin.md) for
advice on how to write a plugin.

## Updating Dependencies

We use [Go Modules](https://github.com/golang/go/wiki/Modules) as the tool to manage vendor dependencies.

Use the following to update the version of all dependencies
```sh
$ go get -u
```

After the dependencies have been updated or added, you might run the following to
cleanup the go module files:
```sh
$ go mod tidy
```

Please refer to [Go Modules](https://github.com/golang/go/wiki/Modules) for more details.

## Developer Certificate of Origin

As required by the CNCF's [charter](https://github.com/cncf/foundation/blob/master/charter.md#11-ip-policy),
all new code contributions must be accompanied by a [Developer Certificate of Origin (DCO)](https://developercertificate.org/). CoreDNS uses [Probot](https://github.com/probot/dco#how-it-works) to enforce the DCO on pull requests.

You may use git option `-s` to append automatically to the `Sign-off-by` line to your commit messages:

```
$ git commit -s -m 'This is my commit message'
```

# Thank You

Thanks for your help! CoreDNS would not be what it is today without your contributions.
