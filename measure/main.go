package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	h "github.com/gopherjs/output-size-action/helpers"
	"github.com/gopherjs/output-size-action/report"
	"github.com/sethvargo/go-githubactions"
)

// env contains GitHub Action environment information.
type env struct {
	Name       string
	Repo       string
	GoPackage  string
	TempDir    string
	ReportJSON string
	ReportMD   string

	GHWorkspace string
	GHEventName string
	GHCommit    string
	GHBranch    string
	GHPRHead    string
	GHPRBase    string

	Event struct {
		Before      string `json:"before"`
		After       string `json:"after"`
		PullRequest struct {
			HTMLURL string `json:"html_url"`
		} `json:"pull_request"`
	}
}

// newEnv initialized github action input parameters.
func newEnv() *env {
	e := &env{
		Name:        githubactions.GetInput("name"),
		Repo:        githubactions.GetInput("repo"),
		GoPackage:   githubactions.GetInput("go-package"),
		ReportJSON:  githubactions.GetInput("report_json"),
		ReportMD:    githubactions.GetInput("report_md"),
		TempDir:     h.TempDir(),
		GHWorkspace: os.Getenv("GITHUB_WORKSPACE"),
		GHEventName: os.Getenv("GITHUB_EVENT_NAME"),
		GHCommit:    os.Getenv("GITHUB_SHA"),
		GHBranch:    os.Getenv("GITHUB_REF"),
		GHPRHead:    os.Getenv("GITHUB_HEAD_REF"),
		GHPRBase:    os.Getenv("GITHUB_BASE_REF"),
	}

	if eventPath := os.Getenv("GITHUB_EVENT_PATH"); eventPath != "" {
		rawEvent, err := os.ReadFile(eventPath)
		h.Must(err, "read event payload from %s", eventPath)
		h.Must(json.Unmarshal(rawEvent, &e.Event), "parse event payload: %s", string(rawEvent))
	}
	return e
}

func (e *env) String() string {
	return fmt.Sprintf("%#v", e)
}

// goVersion determines which Go version the given GopherJS branch is targeting.
func goVersion(repoRoot string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(repoRoot, "compiler", "version_check.go"), nil, 0)
	h.Must(err, "load and parse compiler/version_check.go")

	obj := f.Scope.Lookup("Version")
	h.Must(obj != nil, "failed to loop up Version const in AST.")
	spec := obj.Decl.(*ast.ValueSpec)
	h.Must(len(spec.Values) == 1, "expected exactly one value for Version const, got: %v", spec)
	lit, ok := spec.Values[0].(*ast.BasicLit)
	h.Must(ok && lit.Kind == token.STRING, "Version const must be initialized with a string literal, got: %v", spec.Values[0])
	version, err := strconv.Unquote(lit.Value)
	h.Must(err, "be quoted string")

	parts := strings.Split(version, "+")
	h.Must(len(parts) == 2, "expected '+' separator to be present in the version string, got: %s", version)
	h.Must(strings.HasPrefix(parts[1], "go1"), "expected a valid go version after '+' separator, got: %s", parts[1])
	return parts[1]
}

// installGo installs the given Go version and returns the appropriate Go command to use.
func installGo(version string) string {
	h.Must(
		h.Exec(h.Vars{}, "go", "install", fmt.Sprintf("golang.org/dl/%s@latest", version)),
		"install %s wrapper", version)
	h.Must(h.Exec(h.Vars{}, version, "download"), "download %s", version)
	return version
}

func main() {
	e := newEnv()

	h.Group("Run parameters...", func() { fmt.Println(e) })

	r := report.Report{}
	r.App.Name = e.Name
	r.App.Repo = e.Repo

	if e.GHEventName == "pull_request" {
		r.Trigger = e.Event.PullRequest.HTMLURL
		r.Measurements = []*report.DataPoint{
			{Name: fmt.Sprintf("Pull request (%s)", e.GHPRHead), Commit: e.GHCommit},
			{Name: fmt.Sprintf("Target branch (%s)", e.GHPRBase), Commit: e.GHPRBase},
		}
		if e.GHPRBase != "master" {
			r.Measurements = append(r.Measurements, &report.DataPoint{Name: "Master", Commit: "master"})
		}
	} else if e.GHEventName == "pull_request_target" {
		githubactions.Fatalf("This action executes untrusted code from the pull request and must never be used with the `pull_request_target` trigger. See https://securitylab.github.com/research/github-actions-preventing-pwn-requests/ for details.")
	} else {
		githubactions.Fatalf("Unsupported event type %q.", e.GHEventName)
	}

	if len(r.Measurements) == 0 {
		githubactions.Infof("No comparisons to make, exiting early.")
		return
	}

	h.Group("Cloning reference app...", func() {
		h.Cd(e.TempDir, func() {
			h.Must(h.Exec(h.Vars{}, "git", "clone", e.Repo, "app"), "clone reference app from %s", e.Repo)
			h.Cd("./app", func() {
				commit, err := h.Capture(h.Vars{}, "git", "rev-parse", "HEAD")
				h.Must(err, "obtain HEAD commit")
				r.App.Commit = commit
			})
		})
	})

	for _, dp := range r.Measurements {
		fmt.Printf("🧪 Measuring size for %s\n", dp.Name)
		var goTool string
		h.Group("Building GopherJS", func() {
			h.Cd(e.GHWorkspace, func() {
				h.Must(h.Exec(h.Vars{}, "git", "checkout", dp.Commit), "checkout gopherjs at %s", dp.Commit)
				h.Must(h.Exec(h.Vars{}, "git", "log", "--oneline", "-1", "--no-merges", "--abbrev-commit"), "show checked out revision")
				goTool = installGo(goVersion("."))
				h.Must(h.Exec(h.Vars{}, goTool, "install", "-v", "."), "install gopherjs at %s", dp.Commit)
			})
		})

		h.Group("Building reference app", func() {
			h.Cd(filepath.Join(e.TempDir, "app"), func() {
				goroot, err := h.Capture(h.Vars{}, goTool, "env", "GOROOT")
				h.Must(err, "determine GOROOT")
				env := h.Vars{"GOPHERJS_GOROOT": goroot}

				out := filepath.Join(e.TempDir, "app.js")
				h.Must(h.Exec(env, "gopherjs", "build", "-v", "-o", out, "."), "build reference app")
				dp.Size = h.FileSize(out)
				h.Must(os.Remove(out), "delete compiled app")

				h.Must(h.Exec(env, "gopherjs", "build", "-v", "-m", "-o", out, "."), "build reference app")
				dp.Minified = h.FileSize(out)

				h.Exec(h.Vars{}, "gzip", out)
				dp.Compressed = h.FileSize(out + ".gz")
				h.Must(os.Remove(out+".gz"), "delete compiled app")
			})
		})
	}

	fmt.Println(r) // Log the report.

	if e.ReportJSON != "" {
		h.Must(r.SaveJSON(e.ReportJSON), "save json report to %q", e.ReportJSON)
	}

	if e.ReportMD != "" {
		h.Must(r.SaveMarkdown(e.ReportMD), "save markdown report to %q", e.ReportMD)
	}
}
