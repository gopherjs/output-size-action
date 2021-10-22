# 📦📉 GopherJS reference app output size tracker.

This is a GitHub Action the GopherJS project uses to keep track of how changes to the compiler or runtime affect the size of the generated output. Large output size has been a [long-standing pain point](https://github.com/gopherjs/gopherjs/issues/136) for GopherJS users, and while reducing it significantly is not easy, we can at least try to make sure it doesn't creep up too much.

This check consists of two workflows:

1. Check out a reference GopherJS app and build it with the GopherJS version from the pull request and its target branch as a comparison baseline, generating a report in JSON and Markdown formats.
1. Publish the generated report as a comment in the pull request.

Note that the first workflow executes potentially untrusted code contained in the pull request, and therefore [must not be run](https://securitylab.github.com/research/github-actions-preventing-pwn-requests/) in a context that has write access to the main repository. However, write access is required to post a comment in the pull request, which we achieve by generating the report in an untrusted context of a `pull_request` trigger and handing it off ready to a trusted workflow that simply posts it.

The below are reference workflow configurations:

<details>
<summary>.github/workflows/measure-size.yml</summary>

```yml
name: Measure canonical app size

on: ["pull_request"]

jobs:
  measure:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v2
        with:
          go-version: "^1.17.2"
      - uses: gopherjs/output-size-action/measure@main
        with:
          name: jQuery TodoMVC
          repo: https://github.com/gopherjs/todomvc
          go-package: github.com/gopherjs/todomvc
          report_json: /tmp/report.json
          report_md: /tmp/report.md
      - uses: actions/upload-artifact@v2
        with:
          name: size_report
          path: |
            /tmp/report.json
            /tmp/report.md
```

</details>

<details>
<summary>.github/workflows/publish-size.yml</summary>

```yml
name: Publish canonical app size

on:
  workflow_run:
    workflows: ["Measure canonical app size"]
    types: ["completed"]

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: gopherjs/output-size-action/publish@main
        with:
          report_artifact: size_report
```

</details>

## How it works

### Measure

This step is written in Go and compiled by GopherJS so that it can be run by GitHub as a [native JavaScript action](https://docs.github.com/en/actions/creating-actions/creating-a-javascript-action). This eliminates a need of using Docker and/or publishing container images, thus minimizing setup time for the action. The main package is located under [`./measure`](measure/main.go) directory and compiled code is placed in `_dist`.

For development convenience, `git-hooks.sh` script ensures that the complied code is updated with each commit and you don't have to do it manually. To set up hooks, simply run `./git-hooks.sh` from the root of the repository.

At the high level, the action works as follows:

1. Check out the reference app in a temporary directory.
2. Check out GopherJS repository at the PR commit.
3. Install Go version required by GopherJS and compile GopherJS.
4. Build the reference app and measure default, minified and gzipped sizes.
5. Check out GopherJS at the PR target branch.
6. Install Go version, compile GopherJS and repeat the measurements.
7. If the target branch is not master, repeat the same at master.
8. Generate the report in JSON and Markdown formats and upload them as artifacts.

### Publish

This step heavily relies on GitHub SDK and is written in JavaScript, since using external NPM modules in GopherJS is unfortunately not very easy at this time. The code is located under [`./publish`](publish/publish.js) and relies heavily on `actions/github-script` to provide access to the GitHub API.

This step performs the following:

1. Determine which PR the workflow run is associated with.
2. Find any previous report comments in the PR and collapse them to minimize confusion.
3. Post the Markdown version of the report as a new comment.
