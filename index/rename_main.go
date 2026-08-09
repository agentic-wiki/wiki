//go:build !windows

// The ordinary case, and the one worth reading first: a rename moves a file
// whatever handles are open on it. rename_win.go is the exception.

package index

// contendedReplace is always false here: a failed rename is a real error, so
// retrying it would only delay the report.
func contendedReplace(error) bool { return false }
