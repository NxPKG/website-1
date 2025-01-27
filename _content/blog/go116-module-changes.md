---
title: New module changes in Go 1.16
date: 2021-02-18
by:
- Jay Conrod
tags:
- modules
- versioning
summary: Go 1.16 enables modules by default, provides a new way to install executables, and lets module authors retract published versions.
---


We hope you're enjoying Go 1.16!
This release has a lot of new features, especially for modules.
The [release notes](/doc/go1.16) describe these changes briefly, but let's explore a few of them in depth.

## Modules on by default

The `go` command now builds packages in module-aware mode by default, even when no `go.mod` is present.
This is a big step toward using modules in all projects.

It's still possible to build packages in GOPATH mode by setting the `GO111MODULE` environment variable to `off`.
You can also set `GO111MODULE` to `auto` to enable module-aware mode only when a go.mod file is present in the current directory or any parent directory.
This was previously the default.
Note that you can set `GO111MODULE` and other variables permanently with `go env -w`:

    go env -w GO111MODULE=auto

We plan to drop support for GOPATH mode in Go 1.17.
In other words, Go 1.17 will ignore `GO111MODULE`.
If you have projects that do not build in module-aware mode, now is the time to migrate.
If there is a problem preventing you from migrating, please consider filing an [issue](/issue/new) or an [experience report](/wiki/ExperienceReports).

## No automatic changes to go.mod and go.sum

Previously, when the `go` command found a problem with `go.mod` or `go.sum` like a missing `require` directive or a missing sum, it would attempt to fix the problem automatically.
We received a lot of feedback that this behavior was surprising, especially for commands like `go list` that don't normally have side effects.
The automatic fixes weren't always desirable: if an imported package wasn't provided by any required module, the `go` command would add a new dependency, possibly triggering upgrades of common dependencies.
Even a misspelled import path would result in a (failed) network lookup.

In Go 1.16, module-aware commands report an error after discovering a problem in `go.mod` or `go.sum` instead of attempting to fix the problem automatically.
In most cases, the error message recommends a command to fix the problem.


    $ go build
    example.go:3:8: no required module provides package github.com/khulnasoft-lab/godep/net/html; to add it:
        go get github.com/khulnasoft-lab/godep/net/html
    $ go get github.com/khulnasoft-lab/godep/net/html
    $ go build

As before, the `go` command may use the `vendor` directory if it's present (see [Vendoring](/ref/mod#vendoring) for details).
Commands like `go get` and `go mod tidy` still modify `go.mod` and `go.sum`, since their main purpose is to manage dependencies.

## Installing an executable at a specific version

The `go install` command can now install an executable at a specific version by specifying an `@version` suffix.

    go install golang.org/x/tools/gopls@v0.6.5

When using this syntax, `go install` installs the command from that exact module version, ignoring any `go.mod` files in the current directory and parent directories.
(Without the `@version` suffix, `go install` continues to operate as it always has, building the program using the version requirements and replacements listed in the current module’s `go.mod`.)

We used to recommend `go get -u program` to install an executable, but this use caused too much confusion with the meaning of `go get` for adding or changing module version requirements in `go.mod`.
And to avoid accidentally modifying `go.mod`, people started suggesting more complex commands like:

    cd $HOME; GO111MODULE=on go get program@latest

Now we can all use `go install program@latest` instead.
See [`go install`](/ref/mod#go-install) for details.

In order to eliminate ambiguity about which versions are used, there are several restrictions on what directives may be present in the program's `go.mod` file when using this install syntax.
In particular, `replace` and `exclude` directives are not allowed, at least for now.
In the long term, once the new `go install program@version` is working well for enough use cases, we plan to make `go get` stop installing command binaries.
See [issue 43684](/issue/43684) for details.

## Module retraction

Have you ever accidentally published a module version before it was ready?
Or have you discovered a problem right after a version was published that needed to be fixed quickly?
Mistakes in published versions are difficult to correct.
To keep module builds deterministic, a version cannot be modified after it is published.
Even if you delete or change a version tag, [`proxy.golang.org`](https://proxy.golang.org) and other proxies probably already have the original cached.

Module authors can now *retract* module versions using the `retract` directive in `go.mod`.
A retracted version still exists and can be downloaded (so builds that depend on it won't break), but the `go` command won’t select it automatically when resolving versions like `@latest`.
`go get` and `go list -m -u` will print warnings about existing uses.

For example, suppose the author of a popular library `example.com/lib` releases `v1.0.5`, then discovers a new security issue.
They can add a directive to their `go.mod` file like the one below:

    // Remote-triggered crash in package foo. See CVE-2021-01234.
    retract v1.0.5


Next, the author can tag and push version `v1.0.6`, the new highest version.
After this, users that already depend on `v1.0.5` will be notified of the retraction when they check for updates or when they upgrade a dependent package.
The notification message may include text from the comment above the `retract` directive.

    $ go list -m -u all
    example.com/lib v1.0.0 (retracted)
    $ go get .
    go: warning: example.com/lib@v1.0.5: retracted by module author:
        Remote-triggered crash in package foo. See CVE-2021-01234.
    go: to switch to the latest unretracted version, run:
        go get example.com/lib@latest

For an interactive, browser-based guide, check out [Retract Module Versions](https://play-with-go.dev/retract-module-versions_go116_en/) on [play-with-go.dev](https://play-with-go.dev/).
See the [`retract` directive docs](/ref/mod#go-mod-file-retract) for syntax details.

## Controlling version control tools with GOVCS

The `go` command can download module source code from a mirror like [proxy.golang.org](https://proxy.golang.org) or directly from a version control repository using `git`, `hg`, `svn`, `bzr`, or `fossil`.
Direct version control access is important, especially for private modules that aren't available on proxies, but it's also potentially a security problem: a bug in a version control tool may be exploited by a malicious server to run unintended code.

Go 1.16 introduces a new configuration variable, `GOVCS`, which lets the user specify which modules are allowed to use specific version control tools.
`GOVCS` accepts a comma-separated list of `pattern:vcslist` rules.
The `pattern` is a [`path.Match`](/pkg/path#Match) pattern matching one or more leading elements of a module path.
The special patterns `public` and `private` match public and private modules (`private` is defined as modules matched by patterns in `GOPRIVATE`; `public` is everything else).
The `vcslist` is a pipe-separated list of allowed version control commands or the keyword `all` or `off`.

For example:

    GOVCS=github.com:git,evil.com:off,*:git|hg

With this setting, modules with paths on `github.com` can be downloaded using `git`; paths on `evil.com` cannot be downloaded using any version control command, and all other paths (`*` matches everything) can be downloaded using `git` or `hg`.

If `GOVCS` is not set, or if a module does not match any pattern, the `go` command uses this default: `git` and `hg` are allowed for public modules, and all tools are allowed for private modules.
The rationale behind allowing only Git and Mercurial is that these two systems have had the most attention to issues of being run as clients of untrusted servers.
In contrast, Bazaar, Fossil, and Subversion have primarily been used in trusted, authenticated environments and are not as well scrutinized as attack surfaces.
That is, the default setting is:

    GOVCS=public:git|hg,private:all

See [Controlling version control tools with `GOVCS`](/ref/mod#vcs-govcs) for more details.

## What's next?

We hope you find these features useful. We're already hard at work on the next set of module features for Go 1.17, particularly [lazy module loading](/issue/36460), which should make the module loading process faster and more stable.
As always, if you run into new bugs, please let us know on the [issue tracker](https://github.com/golang/go/issues). Happy coding!
