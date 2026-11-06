package index

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/agentic-wiki/wiki/parse"
)

// ErrNoFrontmatter is returned when asked to set a field on a reserved file that
// carries none. `index.md` and `log.md` are defined as having no frontmatter, so
// giving them a block would break the format rather than annotate the file.
var ErrNoFrontmatter = errors.New("reserved file carries no frontmatter")

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
func writeFile(abs string, content []byte) error {
	// Resolve first: a rename replaces whatever name it is given, so writing to
	// a symlinked entry would swap the link for a regular file and silently
	// fork it into two diverging copies. Writing through to the target is what
	// a plain write did. A broken or unresolvable link falls back to the path
	// as given, which then fails the same way a plain write would.
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
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
	if after == before {
		return nil
	}
	if err := writeFile(e.abs, []byte(after)); err != nil {
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
