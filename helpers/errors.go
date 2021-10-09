package helpers

import "fmt"

// Must panics is the error is not nil.
func Must(predicate interface{}, msg string, args ...interface{}) {
	switch p := predicate.(type) {
	case error:
		if p == nil {
			return
		}
		panic(fmt.Errorf("%s: %w", fmt.Sprintf(msg, args...), p))
	case bool:
		if p {
			return
		}
		panic(fmt.Errorf(msg, args...))
	case nil:
		return
	default:
		panic(fmt.Errorf("unknown predicate type %T: %v", p, p))
	}
}
