package helpers

import "github.com/sethvargo/go-githubactions"

// Group wraps output of f() into a collapsible block in action execution log.
func Group(name string, f func()) {
	githubactions.Group(name)
	defer githubactions.EndGroup()

	f()
}
