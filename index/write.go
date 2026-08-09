package index

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
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
	return os.Rename(name, abs)
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
// The edit is surgical. It finds the `---` fence, replaces exactly the lines
// belonging to each key, and leaves the rest of the file untouched — never
// parsing the frontmatter into a map and re-serializing it, which would silently
// drop everything the YAML subset does not model (nested maps, anchors,
// comments, quoting style).
func (e *Entry) SetFields(fields map[string]any) error {
	if len(fields) == 0 {
		return errors.New("no fields to set")
	}
	// Validate every key and value before writing anything, so a rejected batch
	// applies nothing rather than the prefix that happened to be valid.
	for key, v := range fields {
		if err := validFieldKey(key); err != nil {
			return err
		}
		switch v.(type) {
		case string, []string:
		default:
			return fmt.Errorf("field %q: value must be a string or []string, got %T", key, v)
		}
	}
	raw, err := e.Raw()
	if err != nil {
		return err
	}
	out := raw
	// Sorted, so the same set of fields always produces the same file rather
	// than one whose new-key order depends on map iteration.
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		switch v := fields[key].(type) {
		case string:
			out, err = setFrontmatterValue(out, key, v, e.reserved())
		case []string:
			out, err = setFrontmatterList(out, key, v, e.reserved())
		}
		if err != nil {
			return err
		}
	}
	return e.commit(raw, out)
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

// UnsetField removes a key from the entry's frontmatter, including a block list
// belonging to it.
func (e *Entry) UnsetField(key string) error {
	if err := validFieldKey(key); err != nil {
		return err
	}
	raw, err := e.Raw()
	if err != nil {
		return err
	}
	return e.commit(raw, unsetFrontmatterValue(raw, key))
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
	return nil
}

// checkboxMark matches a GFM checkbox's mark, capturing the single character
// between the brackets so exactly that character can be replaced.
var checkboxMark = regexp.MustCompile(`^\s*[-*+] \[([ xX])\]`)

// validFieldKey rejects keys the engine must not write: the reserved underscore
// namespace (`_path` is the index's, not the user's) and anything that would not
// survive as a YAML key.
func validFieldKey(key string) error {
	switch {
	case strings.TrimSpace(key) == "":
		return errors.New("key must not be empty")
	case strings.HasPrefix(key, "_"):
		return fmt.Errorf("key %q is reserved (leading underscore)", key)
	case strings.ContainsAny(key, ":\n\r"):
		return fmt.Errorf("key %q must not contain ':' or a newline", key)
	}
	return nil
}

// reserved reports whether the entry is one of the two filenames the format
// defines as carrying no frontmatter.
func (e *Entry) reserved() bool {
	// The bundle-root index.md is the documented exception: it carries okf_version.
	if e.Path == "/index.md" {
		return false
	}
	base := e.base()
	return base == "index.md" || base == "log.md"
}

// formatField renders a `key: value` line, quoting only when the value needs it
// and otherwise matching whatever style the line already used.
func formatField(key, value, existing string) string {
	if needsQuote(value) {
		return fmt.Sprintf("%s: %q", key, value)
	}
	if e := strings.TrimSpace(existing); len(e) >= 2 && (e[0] == '\'' || e[0] == '"') && e[len(e)-1] == e[0] {
		return fmt.Sprintf("%s: %c%s%c", key, e[0], value, e[0]) // keep the author's quoting
	}
	return key + ": " + value
}

// needsQuote reports whether a value would not survive unquoted. Deliberately
// conservative: a wrongly-bare value corrupts the file, while a needlessly
// quoted one is merely noisy.
//
// Numbers are quoted, which matters more than it looks: a bare `0.1` is a float,
// so a version or an identifier written bare would stop being the string it was.
func needsQuote(v string) bool {
	if v == "" || v != strings.TrimSpace(v) {
		return true
	}
	if strings.ContainsAny(v, ":#[]{},&*!|>'\"%@`\n\r\t") {
		return true
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return true
	}
	switch strings.ToLower(v) {
	case "true", "false", "null", "yes", "no", "on", "off", "~":
		return true
	}
	return v[0] == '-' || v[0] == '?'
}

// setFrontmatterValue returns content with the frontmatter key set to value,
// changing that key's lines and nothing else.
//
// A file with no frontmatter gets a block, unless it is a reserved file, which by
// definition has none. Replacing a key that held a block list takes the list's
// items with it, or they would orphan into whatever key came next.
func setFrontmatterValue(content, key, value string, reserved bool) (string, error) {
	lines := strings.Split(content, "\n")
	open, closed := frontmatterFence(lines)
	if open < 0 {
		if reserved {
			return "", ErrNoFrontmatter
		}
		return fmt.Sprintf("---\n%s\n---\n%s", formatField(key, value, ""), content), nil
	}
	if closed < 0 {
		return "", errors.New("unterminated frontmatter")
	}

	if i, end, existing, ok := findKey(lines, open, closed, key); ok {
		out := append([]string{}, lines[:i]...)
		out = append(out, formatField(key, value, existing)+carriage(lines[i]))
		return strings.Join(append(out, lines[end+1:]...), "\n"), nil
	}
	// Absent: insert before the closing fence, so the new key joins the block
	// rather than displacing anything already in it.
	out := append([]string{}, lines[:closed]...)
	out = append(out, formatField(key, value, "")+carriage(lines[closed]))
	return strings.Join(append(out, lines[closed:]...), "\n"), nil
}

// setFrontmatterList is setFrontmatterValue for a list value, emitting one or
// more lines in place of the key's existing ones.
func setFrontmatterList(content, key string, values []string, reserved bool) (string, error) {
	lines := strings.Split(content, "\n")
	open, closed := frontmatterFence(lines)
	if open < 0 {
		if reserved {
			return "", ErrNoFrontmatter
		}
		return fmt.Sprintf("---\n%s\n---\n%s", flowList(key, values), content), nil
	}
	if closed < 0 {
		return "", errors.New("unterminated frontmatter")
	}

	if i, end, _, ok := findKey(lines, open, closed, key); ok {
		// end > i means the key owned `- item` lines, so it was written as a
		// block. Keep it that way.
		var repl []string
		if end > i && len(values) > 0 {
			repl = blockList(key, values)
		} else {
			repl = []string{flowList(key, values)}
		}
		cr := carriage(lines[i])
		for j := range repl {
			repl[j] += cr
		}
		out := append([]string{}, lines[:i]...)
		out = append(out, repl...)
		return strings.Join(append(out, lines[end+1:]...), "\n"), nil
	}
	out := append([]string{}, lines[:closed]...)
	out = append(out, flowList(key, values)+carriage(lines[closed]))
	return strings.Join(append(out, lines[closed:]...), "\n"), nil
}

// flowList renders `key: [a, b]`. An empty list is written `key: []`, which is
// a declared-but-empty field, not the same thing as an absent one.
func flowList(key string, values []string) string {
	items := make([]string, len(values))
	for i, v := range values {
		items[i] = quoteItem(v)
	}
	return key + ": [" + strings.Join(items, ", ") + "]"
}

// blockList renders a key over several lines, one `- item` each.
func blockList(key string, values []string) []string {
	out := make([]string, 0, len(values)+1)
	out = append(out, key+":")
	for _, v := range values {
		out = append(out, "  - "+quoteItem(v))
	}
	return out
}

// quoteItem quotes a list item that would not survive bare. A comma or a
// bracket ends the item in flow style, so the rule is stricter than for a
// scalar, where those characters are harmless.
func quoteItem(v string) string {
	if needsQuote(v) || strings.ContainsAny(v, ",[]") {
		return fmt.Sprintf("%q", v)
	}
	return v
}

// unsetFrontmatterValue removes a key and any block list belonging to it,
// returning content unchanged when the key is absent.
func unsetFrontmatterValue(content, key string) string {
	lines := strings.Split(content, "\n")
	open, closed := frontmatterFence(lines)
	if open < 0 || closed < 0 {
		return content
	}
	i, end, _, ok := findKey(lines, open, closed, key)
	if !ok {
		return content
	}
	return strings.Join(append(append([]string{}, lines[:i]...), lines[end+1:]...), "\n")
}

// findKey locates a key's line within a frontmatter block, along with the last
// line belonging to it: a block list's `- item` lines belong to the key above
// them, so replacing or removing the key has to take them too.
func findKey(lines []string, open, closed int, key string) (start, end int, existing string, ok bool) {
	for i := open + 1; i < closed; i++ {
		line := strings.TrimRight(lines[i], "\r")
		if line == "" || line[0] == ' ' || line[0] == '\t' || strings.HasPrefix(strings.TrimSpace(line), "- ") {
			continue // a nested or block-list line belongs to the key above it
		}
		k, v, cut := strings.Cut(line, ":")
		if !cut || strings.TrimSpace(k) != key {
			continue
		}
		end = i
		if strings.TrimSpace(v) == "" { // a block list follows
			for j := i + 1; j < closed; j++ {
				if strings.HasPrefix(strings.TrimSpace(strings.TrimRight(lines[j], "\r")), "- ") {
					end = j
					continue
				}
				break
			}
		}
		return i, end, v, true
	}
	return 0, 0, "", false
}

// frontmatterFence locates the opening and closing `---` lines, or -1 when the
// content has no frontmatter block at all.
func frontmatterFence(lines []string) (open, closed int) {
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return -1, -1
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			return 0, i
		}
	}
	return 0, -1
}

func carriage(line string) string {
	if strings.HasSuffix(line, "\r") {
		return "\r"
	}
	return ""
}
