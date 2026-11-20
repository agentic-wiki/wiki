package index

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/agentic-wiki/wiki/parse"
)

// ErrNoFrontmatter is returned when asked to set a field on a reserved file that
// carries none. `index.md` and `log.md` are defined as having no frontmatter, so
// giving them a block would break the format rather than annotate the file.
var ErrNoFrontmatter = errors.New("reserved file carries no frontmatter")

// ErrNotInBundle refuses a write to an entry Read outside a bundle: it has no
// file to write.
var ErrNotInBundle = errors.New("entry is not a file in a bundle")

// ErrOutsideBundle refuses a file whose real path, symlinks resolved, is not
// the bundle's content: outside the bundle (a symlinked entry pointing at
// ~/.bashrc, which a cloned repo can ship), or on a dot path inside it (a link
// to .git/config). Writing through it would let whoever edits the wiki rewrite
// that file; reading it would show it.
var ErrOutsideBundle = errors.New("file is outside the bundle's content")

// errDotPath tells, within ErrOutsideBundle, a real path on a dot path inside
// the bundle from one outside it.
var errDotPath = errors.New("on a dot path")

// writeFile replaces a file's contents atomically: a temp file in the same
// directory, then a rename over the target.
//
// os.WriteFile opens with O_TRUNC, so the file is emptied before the new content
// lands, and anything reading it in that window sees an empty or partial entry.
// The window is small but real: an editor, an agent, or a watcher may be reading
// the same file, and a process killed mid-write leaves the entry truncated on
// disk. A rename within one filesystem is atomic, so a reader sees either the
// whole old file or the whole new one, and a crash leaves the original intact.
//
// Atomicity is **per file**. A command rewriting several of them can still be
// interrupted between two writes; that is a separate problem and this does not
// pretend to solve it.
//
// A rename breaks a hardlink, and deliberately so. Two names for one inode are
// two entries in the index, at two paths, so a relative link is normalized
// differently for each; sharing the inode meant the second write clobbered the
// first and left one of them pointing nowhere. Each name now gets the content
// that is correct where it sits.
//
// No fsync: the risk being closed here is a concurrent reader seeing a torn
// file, not power loss. Durability across a crash would need the temp file and
// its directory synced, which costs real time on every write.
//
// Only a file inside the bundle is written: root is the bundle's folder, and
// a file whose real path (symlinks resolved) is outside it is refused with
// ErrOutsideBundle. A link that stays inside the bundle is written through.
func writeFile(root, abs string, content []byte) error {
	// Resolve first: a rename replaces whatever name it is given, so writing to
	// a symlinked entry would swap the link for a regular file and silently
	// fork it into two diverging copies. Writing through to the target is what
	// a plain write did, as long as the target is in the bundle.
	abs, err := inBundle(root, abs)
	if err != nil {
		return err
	}
	dir := filepath.Dir(abs)
	tmp, err := os.CreateTemp(dir, ".wiki-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // a no-op once the rename has succeeded

	perm := os.FileMode(0o644)
	if fi, err := os.Stat(abs); err == nil {
		perm = fi.Mode().Perm() // keep whatever the user set
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, perm); err != nil {
		return err
	}
	return rename(name, abs)
}

// createFile writes content to a new file at abs, the way writeFile replaces
// one: through a temp file, so a reader never sees it half written. It is
// linked into place rather than renamed, because a link fails where a file
// already is (fs.ErrExist), even one that appeared a moment ago: it never
// overwrites.
//
// A filesystem without hard links (exFAT, some FUSE and SMB mounts) refuses
// the link; then the file is created exclusively and written in place, which
// still never overwrites, though a reader may see it half written.
func createFile(root, abs string, content []byte) error {
	if _, err := inBundle(root, abs); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".wiki-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // the link stays, the temp name goes
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	err = link(name, abs)
	if !errors.Is(err, fs.ErrPermission) && !errors.Is(err, errors.ErrUnsupported) {
		return err
	}
	f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(abs)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(abs)
		return err
	}
	return nil
}

// pagePath checks a bundle path to write a page at, and the page: a .md path
// with no empty, `.` or `..` segment, none starting with `.`, and no `\`, `:`
// or control character in a segment (ErrBadPath), and no more than
// parse.MaxBlocks blocks (parse.ErrRefused). It returns the path without its
// leading slash.
func pagePath(p string, content []byte) (string, error) {
	rel := strings.TrimPrefix(p, "/")
	if !strings.HasSuffix(rel, ".md") || path.Base(rel) == ".md" {
		return "", fmt.Errorf("%w: %q is not a .md path", ErrBadPath, p)
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.HasPrefix(seg, ".") {
			return "", fmt.Errorf("%w: %q has an empty, dot or dot-dot segment", ErrBadPath, p)
		}
		if strings.ContainsFunc(seg, func(r rune) bool { return r == '\\' || r == ':' || unicode.IsControl(r) }) {
			return "", fmt.Errorf("%w: %q has a \\, : or control character", ErrBadPath, p)
		}
	}
	if n := len(parse.Blocks(parse.Body(string(content)))); n > parse.MaxBlocks {
		return "", fmt.Errorf("%w: the page has %d blocks, more than %d", parse.ErrRefused, n, parse.MaxBlocks)
	}
	return rel, nil
}

// ErrChanged refuses a guarded write: the file no longer has the version
// the caller read (or is gone).
var ErrChanged = errors.New("the file changed since it was read")

// WriteIf replaces the page at the bundle path p ("/notes/x.md") with
// content, the whole file, only if it still has version: the bytes the caller
// read and worked the new ones out from. Otherwise it is ErrChanged and
// nothing is written. So a write built from one read is all or nothing, where
// ApplyBlocks would merge. The path follows Create's rules (ErrBadPath, and
// ErrOutsideBundle where the file really is), except that a page `ignore`
// leaves out can be written, as can one inside a nested bundle (SkipsFolder
// isn't checked): it is still a page of this bundle. The write is
// writeFile's: through a temp file, keeping the file's permissions.
//
// As for every write in wiki, a write landing between WriteIf's read and its
// rename is lost (the editor ADR's known limit L-2): the version check is not
// atomic with the write.
//
// The index isn't changed: rebuild it.
func (idx *Index) WriteIf(p, version string, content []byte) error {
	rel, err := pagePath(p, content)
	if err != nil {
		return err
	}
	abs := filepath.Join(idx.Bundle.Dir, filepath.FromSlash(rel))
	real, err := inBundle(idx.Bundle.Dir, abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s is gone", ErrChanged, p)
		}
		return err
	}
	b, err := os.ReadFile(real)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %s is gone", ErrChanged, p)
	case err != nil:
		return err
	case FileVersion(b) != version:
		return fmt.Errorf("%w: %s", ErrChanged, p)
	}
	return writeFile(idx.Bundle.Dir, abs, content)
}

// ErrExists refuses to create a file where one already is.
var ErrExists = errors.New("a file exists at that path")

// ErrBadPath refuses a path to create: not a .md bundle path, or one with an
// empty, `.` or `..` segment, a segment starting with `.`, or a `\`, `:` or
// control character in a segment (not portable: a Windows stream name).
var ErrBadPath = errors.New("not a path for a new entry")

// Create writes a new entry at the bundle path p ("/notes/x.md") with content,
// the whole file, making the folders it needs. It never overwrites: a file
// already there is ErrExists. p must be a .md path with no empty, `.` or `..`
// segment and none starting with `.` (ErrBadPath), and is judged where the
// file would really land: through a folder linked out of the bundle it is
// ErrOutsideBundle. A path the bundle's `ignore` leaves out is ErrBadPath too,
// since the file would not be an entry.
//
// The index isn't changed (rebuild it to list the entry, as after Move): the
// entry is returned, read from the new file.
func (idx *Index) Create(p string, content []byte) (*Entry, error) {
	rel, err := pagePath(p, content)
	if err != nil {
		return nil, err
	}
	if matchAnyGlob(idx.ignoreIn, "/"+rel) || idx.SkipsFolder(path.Dir(rel)) {
		return nil, fmt.Errorf("%w: wiki.toml's ignore leaves %q out", ErrBadPath, p)
	}
	abs := filepath.Join(idx.Bundle.Dir, filepath.FromSlash(rel))
	if _, err := inBundle(idx.Bundle.Dir, abs); err != nil {
		return nil, err
	}
	made := missingDirs(filepath.Dir(abs))
	undo := func() {
		for _, d := range made {
			os.Remove(d) // only an empty folder goes
		}
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		undo()
		return nil, err
	}
	if err := createFile(idx.Bundle.Dir, abs, content); err != nil {
		undo()
		if errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("%w: %s", ErrExists, p)
		}
		return nil, err
	}
	return parseEntry(idx.Bundle.Dir, abs)
}

// link is os.Link, a variable so a test can play a filesystem without hard
// links.
var link = os.Link

// inBundle returns abs's real path (realPath), or ErrOutsideBundle when that
// path is not the bundle's content: outside the bundle's real root, or hidden
// relative to it (a segment starting with ., as Build skips). The one
// containment rule: writes and moves refuse such a path, and Build leaves it
// out.
func inBundle(root, abs string) (string, error) {
	real, err := realPath(abs)
	if err != nil {
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrOutsideBundle, real)
	}
	if hidden(filepath.ToSlash(rel)) {
		return "", fmt.Errorf("%w: %s is %w", ErrOutsideBundle, real, errDotPath)
	}
	return real, nil
}

// realPath is abs with every symlink resolved. A path that can't be resolved
// (not there yet, or a broken link) is its deepest existing folder resolved,
// plus the rest as written: a write then replaces that name itself, never a
// file it points to, and a move's new folders are judged where they would
// really be made.
func realPath(abs string) (string, error) {
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real, nil
	}
	dir, rest := filepath.Dir(abs), filepath.Base(abs)
	for {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(real, rest), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no existing folder above %s", abs)
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
}

// SetField writes key: value into the entry's frontmatter, preserving every
// other byte of the file, and refreshes the entry in the index.
func (e *Entry) SetField(key, value string) error {
	return e.SetFields(map[string]any{key: value})
}

// SetFields writes several frontmatter fields in one pass. Values may be a
// string or a []string; anything else is an error naming the key and its type.
//
// map[string]any rather than map[string]string so this mirrors Frontmatter,
// which returns the same shape: a consumer can read the frontmatter, edit it,
// and write it back. With separate scalar and list calls, setting a status and a
// tag list together took two writes, which is exactly the half-updated entry one
// pass exists to prevent. Frontmatter is genuinely heterogeneous, so a
// map[string]string was never the honest type for it.
//
// One pass rather than a loop of SetField: two writes can leave an entry
// half-updated if the second fails, and a consumer changing two related fields
// (a status and an assignee, say) means them as one change.
//
// The edit changes only the lines each key owns (parse.SetField,
// parse.SetFieldList) and leaves the rest of the file as it is. It never parses
// the frontmatter into a map and re-serializes it, which would silently drop
// everything the YAML subset does not model (anchors, comments, quoting style).
//
// The new content is built in memory and written once, so an invalid key or a
// key that appears twice fails the whole batch before anything is written.
func (e *Entry) SetFields(fields map[string]any) error {
	if len(fields) == 0 {
		return errors.New("no fields to set")
	}
	for key, v := range fields {
		switch v.(type) {
		case string, []string:
		default:
			return fmt.Errorf("field %q: value must be a string or []string, got %T", key, v)
		}
	}
	raw, err := e.frontmatterRaw()
	if err != nil {
		return err
	}
	out := raw
	// Sorted, so the same set of fields always produces the same file rather
	// than one whose new-key order depends on map iteration.
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		switch v := fields[key].(type) {
		case string:
			out, err = parse.SetField(out, key, v)
		case []string:
			out, err = parse.SetFieldList(out, key, v)
		}
		if err != nil {
			return e.named(err)
		}
	}
	return e.commit(raw, out)
}

// frontmatterRaw reads the entry for a frontmatter write. A reserved file has no
// header by definition, so it gets none: ErrNoFrontmatter.
func (e *Entry) frontmatterRaw() (string, error) {
	raw, err := e.Raw()
	if err != nil {
		return "", err
	}
	// A reserved file has no header (Body is all of it): it gets none. An
	// unterminated one counts as none there, and elsewhere goes on to the write,
	// which refuses it with the file named (parse.EditError).
	if e.reserved() && parse.Body(raw) == raw {
		return "", ErrNoFrontmatter
	}
	return raw, nil
}

// SetFieldList writes a list-valued frontmatter field: `tags`, `blockers`, and
// anything else the bundle spells as a list.
//
// Separate from SetField because a list is not a string that happens to contain
// brackets. Passing "[a, b]" to SetField writes `key: "[a, b]"` — correctly
// quoted for a scalar, and a one-element list when read back.
//
// The existing shape is kept: a key already written as a block list stays one,
// so the API does not reformat frontmatter it was only asked to change. A new
// key is written flow-style, which is what the scaffolds use.
func (e *Entry) SetFieldList(key string, values []string) error {
	return e.SetFields(map[string]any{key: values})
}

// UnsetField removes a key from the entry's frontmatter, with every line it owns.
func (e *Entry) UnsetField(key string) error {
	raw, err := e.Raw()
	if err != nil {
		return err
	}
	out, err := parse.UnsetField(raw, key)
	if err != nil {
		return e.named(err)
	}
	return e.commit(raw, out)
}

// SetCheckbox sets the done state of the `- [ ]` item at the given line.
//
// Keyed by line because that is the only stable identity a checkbox has: its
// text may repeat within an entry, and parse.Checkbox already carries the line.
// Exactly one character changes.
func (e *Entry) SetCheckbox(line int, done bool) error {
	raw, err := e.Raw()
	if err != nil {
		return err
	}
	lines := strings.Split(raw, "\n")
	if line < 1 || line > len(lines) {
		return fmt.Errorf("no line %d in %s", line, e.Path)
	}
	i := line - 1
	m := checkboxMark.FindStringSubmatchIndex(lines[i])
	if m == nil {
		return fmt.Errorf("no checkbox on line %d of %s", line, e.Path)
	}
	mark := " "
	if done {
		mark = "x"
	}
	lines[i] = lines[i][:m[2]] + mark + lines[i][m[3]:]
	return e.commit(raw, strings.Join(lines, "\n"))
}

// commit writes the new content when it differs and refreshes the entry, so a
// caller holding an index never reads back what it just overwrote.
//
// A no-op write is skipped deliberately: it would touch the mtime and wake every
// file watcher for a change that did not happen.
func (e *Entry) commit(before, after string) error {
	if !e.inBundle() {
		return ErrNotInBundle // checked first: an entry from Read can't be written, even to no change
	}
	if after == before {
		return nil
	}
	if err := writeFile(e.root, e.abs, []byte(after)); err != nil {
		return err
	}
	return e.refresh()
}

// refresh re-parses the entry in place, so a caller holding an index never reads
// back what it just overwrote.
//
// A full re-parse rather than patching the field that changed: setting a key
// that was absent inserts a line, and replacing a block list with a scalar
// removes several, either of which shifts every line number below it. Links,
// checkboxes, and headings all carry line numbers, so anything less would leave
// them quietly pointing at the wrong lines.
//
// The entry keeps its identity, so pointers the index and its callers already
// hold stay valid. Not refreshed: how *other* entries' wikilinks resolve, which
// depends on this entry's `aliases` and is settled by a Build pass. Editing
// aliases through this API therefore wants a rebuild.
func (e *Entry) refresh() error {
	fresh, err := parseEntry(e.root, e.abs)
	if err != nil {
		return err
	}
	e.Type = fresh.Type
	e.Links = fresh.Links
	e.SelfAnchors = fresh.SelfAnchors
	e.Outside = fresh.Outside
	e.Checkboxes = fresh.Checkboxes
	e.Headings = fresh.Headings
	e.wikilinks = fresh.wikilinks
	e.fm = fresh.fm
	e.typed = fresh.typed
	e.fmErr = fresh.fmErr
	return nil
}

// checkboxMark matches a GFM checkbox's mark, capturing the single character
// between the brackets so exactly that character can be replaced.
var checkboxMark = regexp.MustCompile(`^\s*[-*+] \[([ xX])\]`)

// reserved reports whether the entry is one of the two filenames the format
// defines as carrying no frontmatter.
func (e *Entry) reserved() bool {
	// The bundle-root index.md is the documented exception: it carries okf_version.
	if e.Path == "/index.md" {
		return false
	}
	return e.base() == "index.md" || e.IsLog()
}

// named puts the entry's path into a write's refusal (parse.EditError), so it
// reads "frontmatter of /x.md can't be edited safely: …". Other errors are
// returned as they are.
func (e *Entry) named(err error) error {
	var refused *parse.EditError
	if errors.As(err, &refused) {
		named := *refused
		named.Path = e.Path
		return &named
	}
	return err
}
