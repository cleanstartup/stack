package activity

import "errors"

// ErrNotFound signals that the requested activity does not exist for the
// caller. Transports render this using their standard "not found"
// representation: web renders an HTTP 404, CLI returns exit code 1.
var ErrNotFound = errors.New("not found")
