package index

import (
	"os"
	"time"
)

// rename moves src onto dst, retrying only while the failure is contention:
// something else holding one of them open at that instant.
//
// Not every system lets a file be renamed or replaced while another handle is
// open on it. Where that is so, the failure is usually transient and not the
// user's doing, and a short wait clears it; contendedReplace says whether an
// error is that case, and is false on systems where a rename does not care who
// is reading.
//
// Any other error returns immediately, since retrying a real permission error
// would only delay reporting it.
//
// This is a mitigation rather than a cure: contention held longer than the
// window still fails. See the backlog task on replace semantics.
//
// This file is the only sanctioned home for os.Rename, and a test enforces
// that: every rename in the engine has to inherit this handling, or a caller
// that reached for the raw call would silently lose it.
func rename(src, dst string) error {
	err := os.Rename(src, dst)
	for i := 0; err != nil && contendedReplace(err) && i < 10; i++ {
		time.Sleep(time.Duration(i+1) * 2 * time.Millisecond) // ~110ms over ten tries
		err = os.Rename(src, dst)
	}
	return err
}
