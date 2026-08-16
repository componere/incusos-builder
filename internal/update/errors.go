package update

import "errors"

// ErrFetch is returned when acquiring or reading an update-server
// artifact fails. Every acquisition failure wraps it. Callers map it to
// process exit code 5.
var ErrFetch = errors.New("acquisition failed")
