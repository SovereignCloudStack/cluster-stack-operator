# Contributing Guidelines
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Contributing Guidelines](#contributing-guidelines)
  - [Finding Things That Need Help](#finding-things-that-need-help)
  - [Branches](#branches)
  - [Contributing a Patch](#contributing-a-patch)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

Read the following guide if you're interested in contributing to cluster-stack-operator.

## Finding Things That Need Help

If you're new to the project and want to help, but don't know where to start, we have a semi-curated list of issues that should not need deep knowledge of the system. [Have a look and see if anything sounds interesting].
Before starting to work on the issue, make sure that it doesn't have a [lifecycle/active] label. If the issue has been assigned, reach out to the assignee.
Alternatively, read some of the docs on other controllers and try to write your own, file and fix any/all issues that come up, including gaps in documentation!

If you're a more experienced contributor, looking at unassigned issues in the next release milestone is a good way to find work that has been prioritized. For example, if the latest minor release is `v1.0`, the next release milestone is `v1.1`.

Help and contributions are very welcome in the form of code contributions but also in helping to moderate office hours, triaging issues, fixing/investigating flaky tests, cutting releases, helping new contributors with their questions, reviewing proposals, etc.


## Branches

Cluster Stack Operator has two types of branches: the *main* branch and
*release-X* branches.

The *main* branch is where development happens. All the latest and
greatest code, including breaking changes, happens on main.

The *release-X* branches contain stable, backwards compatible code. On every
major or minor release, a new branch is created. It is from these
branches that minor and patch releases are tagged. In some cases, it may
be necessary to open PRs for bugfixes directly against stable branches, but
this should generally not be the case.

## Contributing a Patch

1. If working on an issue, signal other contributors that you are actively working on it using `/lifecycle active`.
2. Fork the desired repo, develop and test your code changes.
3. Submit a pull request.
    1.  All code PR must be have a title starting with one of
        - ⚠️ (`:warning:`, major or breaking changes)
        - ✨ (`:sparkles:`, feature additions)
        - 🐛 (`:bug:`, patch and bugfixes)
        - 📖 (`:book:`, documentation or proposals)
        - 🌱 (`:seedling:`, minor or other)
    2. If the PR requires additional action from users switching to a new release, include the string "action required" in the PR release-notes.
    3. All code changes must be covered by unit tests and E2E tests.
    4. All new features should come with user documentation.
4. Once the PR has been reviewed and is ready to be merged, commits should be.
    1. Ensure that commit message(s) are be meaningful and commit history is readable.

All changes must be code reviewed. Coding conventions and standards are explained in the official [developer docs](https://github.com/kubernetes/community/tree/master/contributors/devel). Expect reviewers to request that you avoid common [go style mistakes](https://github.com/golang/go/wiki/CodeReviewComments) in your PRs.
