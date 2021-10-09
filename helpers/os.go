package helpers

import (
	"fmt"
	"os"
	"strings"

	"github.com/gopherjs/gopherjs/js"
)

var childProcess = js.Global.Call("require", "child_process")

// Exec a command with the given args as a subprocess, redirecting all output to
// the stdout/stderr. Blocks until execution is completed.
func Exec(name string, args ...string) (exitErr error) {
	defer func() {
		if err := recover(); err != nil {
			exitErr = err.(error)
		}
	}()
	fmt.Println("$", name, strings.Join(args, " "))
	childProcess.Call("execFileSync", name, args, map[string]interface{}{
		"stdio": "inherit",
	})
	return nil
}

// Capture command stdout and return it as a string.
func Capture(name string, args ...string) (_ string, exitErr error) {
	defer func() {
		if err := recover(); err != nil {
			exitErr = err.(error)
		}
	}()

	fmt.Println("$", name, strings.Join(args, " "))
	out := childProcess.Call("execFileSync", name, args, map[string]interface{}{
		"encoding": "utf-8",
	}).String()
	return strings.TrimSpace(out), nil
}

// Cd into the given directory and execute f(). Restores the original working
// directory upon return.
func Cd(dir string, f func()) {
	cwd, err := os.Getwd()
	Must(err, "determine current directory")
	Must(os.Chdir(dir), "change working directory to %q", dir)

	defer func() {
		Must(os.Chdir(cwd), "restore current directory to %q", cwd)
	}()

	f()
}

// TempDir create and return a temporary directory. Panics if fails.
func TempDir() string {
	dir, err := os.MkdirTemp(os.TempDir(), "output-size")
	Must(err, "create temporary directory")
	return dir
}

// FileSize returns size of the given file.
func FileSize(f string) int64 {
	info, err := os.Stat(f)
	Must(err, "stat compiled app")
	return info.Size()
}
