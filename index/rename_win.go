//go:build windows

package index

import (
	"errors"
	"syscall"
)

// Go's syscall package exports ERROR_ACCESS_DENIED but not the sharing codes.
const (
	errSharingViolation = syscall.Errno(32) // ERROR_SHARING_VIOLATION
	errLockViolation    = syscall.Errno(33) // ERROR_LOCK_VIOLATION
)

// contendedReplace reports whether a failed replace is worth retrying.
//
// Here a replace deletes the file it is replacing, and files opened for reading
// do not permit that, so the operation fails while anything else holds the entry
// open. The usual cause is transient and not the user's doing: a scanner or an
// indexer opening a file it just saw change.
//
// A handle held longer than the retry window is a different problem, and it
// surfaces as the error rather than being hidden. The durable fix is a replace
// with POSIX semantics, which the standard library does not use — see the
// backlog task on replace semantics.
func contendedReplace(err error) bool {
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED) ||
		errors.Is(err, errSharingViolation) ||
		errors.Is(err, errLockViolation)
}
