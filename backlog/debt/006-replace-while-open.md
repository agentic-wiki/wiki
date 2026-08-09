---
type: task
title: "replacing an open file: a retry, not proper semantics"
status: todo
priority: medium
tags: [debt, platform, mutation]
---

`writeFile` replaces an entry with a temp file and a rename. On Unix that is atomic and works regardless of who has the file open. On Windows it is neither.

Replacing a file there means deleting the old one, and Go opens files for reading without `FILE_SHARE_DELETE`, so `os.Rename` over a file another handle has open fails with `ERROR_ACCESS_DENIED` or `ERROR_SHARING_VIOLATION`. The plain `os.WriteFile` this replaced needed only write sharing and succeeded, so atomic writes introduced a failure mode on Windows that did not exist before.

**What shipped is a mitigation:** `rename` retries for ~110ms, but *only* while the error is one of the contention codes (`index/rename_windows.go`; the predicate is `false` on every other platform, so there is no retry and no cost off Windows). That covers the common cause, which is transient and not the user's doing — an antivirus scanner or the search indexer opening a file it just saw change. It does not cover a handle held open for longer, where the write simply fails.

**The proper fix is POSIX rename semantics.** `SetFileInformationByHandle` with `FILE_RENAME_INFO_EX` and `FILE_RENAME_FLAG_POSIX_SEMANTICS` replaces a file even with open handles, exactly as Unix does; the existing handles keep referring to the now-unlinked file. Go's own standard library already uses the equivalent `FILE_DISPOSITION_POSIX_SEMANTICS` for *delete* (`os.Remove`), so the pattern is established — it just is not applied to rename, which still goes through `MoveFileEx(MOVEFILE_REPLACE_EXISTING)` as of Go 1.26.

**Why it was not done now:**

- It needs `golang.org/x/sys/windows`, a second dependency, on a tool that took its first one this release and only to fix silent config bugs.
- It needs a fallback path: `FILE_RENAME_INFO_EX` is Windows 10 1709+, and filesystems without POSIX semantics (FAT32) reject it, so the `MoveFileEx` route has to stay anyway.
- It cannot be verified where this was written. Shipping untested platform-specific syscall code in a release is a worse trade than a retry whose failure mode is "returns the error it would have returned".

**Also unverified:** the concurrent-reader atomicity test (`TestCommandRewritesAreAtomic`) is skipped on Windows, because a reader looping that tightly holds the target permanently open and the test would measure contention rather than tearing. The property it checks is Unix-shaped; the Windows risk is the write failing, not a torn read.

**Acceptance:** a rename over an entry with an open read handle succeeds on Windows 10 1709+ without retrying; older Windows and non-POSIX filesystems fall back to the current path; the atomicity test runs on Windows rather than skipping; verified on a real Windows runner, not cross-compiled.
