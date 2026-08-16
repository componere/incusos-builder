package update

import "github.com/componere/incusos-builder/internal/errdefs"

// ErrFetch is returned when acquiring or reading an update-server
// artifact fails. Every acquisition failure in this adapter wraps it.
// Callers map it to process exit code 5. It is the §6 name for
// [errdefs.ErrFetch]; [errors.Is] matches through either.
var ErrFetch = errdefs.ErrFetch
