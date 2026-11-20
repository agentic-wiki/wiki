package index

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/agentic-wiki/wiki/parse"
)

// FileVersion is a file's version: the hex of the first 16 bytes of the
// SHA-256 of its content. 128 bits, so two files with one version are out of
// reach.
func FileVersion(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:16])
}

// Version is the entry's version, as it is on disk now.
func (e *Entry) Version() (string, error) {
	raw, err := e.Raw()
	if err != nil {
		return "", err
	}
	return FileVersion([]byte(raw)), nil
}

// Blocks splits the entry's body (the frontmatter is never a block: its fields
// have their own write) into blocks, as it is on disk now.
func (e *Entry) Blocks() ([]parse.Block, error) {
	body, err := e.Body()
	if err != nil {
		return nil, err
	}
	return parse.Blocks(body), nil
}

// Applied is what a guarded write did: the entry's new Version, the operations
// from the base to it (the disk's changes included), what it couldn't apply,
// and whether the page had been rewritten.
type Applied struct {
	Version   string           `json:"version"`
	Ops       []parse.Op       `json:"ops"`
	Conflicts []parse.Conflict `json:"conflicts"`
	Rewritten bool             `json:"rewritten"`
	// Fields are the keys SetFieldsIf didn't write, because they had changed
	// on disk, with their values there.
	Fields []FieldConflict `json:"fields,omitempty"`
}

// FieldConflict is a frontmatter key changed on disk since the caller read it:
// its value there (a string, a []string, or nil when it is gone).
type FieldConflict struct {
	Key    string `json:"key"`
	Theirs any    `json:"theirs"`
}

// ApplyBlocks writes an app's changes to the entry's body, block by block:
// base is the version they were made against, baseBlocks its block hashes,
// and ops the changes. It reads the file, merges three ways (parse.MergeBlocks)
// and writes once, so every byte the operations didn't change stays, and what
// the disk changed meanwhile is kept. A conflicting operation is not applied:
// it comes back in Applied.Conflicts with the disk's blocks.
//
// The header is copied as it is. Nothing is written when the merged body is
// the file's. It refuses (parse.ErrRefused, nothing written) operations that
// MergeBlocks refuses, a baseBlocks that isn't the file's blocks when base is
// its version, and a change that would start a header-less file with a
// frontmatter fence: the header is written only by SetFieldsIf. A file
// outside the bundle's content is refused with ErrOutsideBundle.
func (e *Entry) ApplyBlocks(base string, baseBlocks []string, ops []parse.Op) (Applied, error) {
	header, body, version, err := e.forWrite()
	if err != nil {
		return Applied{}, err
	}
	blocks := parse.Blocks(body)
	if base == version && !slices.Equal(baseBlocks, blockHashes(blocks)) {
		return Applied{}, fmt.Errorf("%w: base is the page's version, but baseBlocks aren't its blocks", parse.ErrRefused)
	}
	merged, _, conflicts, rewritten, err := parse.MergeBlocks(baseBlocks, ops, body)
	if err != nil {
		return Applied{}, err
	}
	// A byte order mark doesn't hide a fence: readers that strip it see a header.
	if bare := strings.TrimPrefix(merged, "\ufeff"); header == "" && parse.Body(bare) != bare {
		return Applied{}, fmt.Errorf("%w: the page would start with a frontmatter fence; the header is written by its fields", parse.ErrRefused)
	}
	if err := e.commit(header+body, header+merged); err != nil {
		return Applied{}, err
	}
	known := make([]parse.Block, len(baseBlocks))
	for i, h := range baseBlocks {
		known[i] = parse.Block{Hash: h}
	}
	return Applied{
		Version:   FileVersion([]byte(header + merged)),
		Ops:       nonNil(parse.DiffBlocks(known, parse.Blocks(merged))),
		Conflicts: nonNil(conflicts),
		Rewritten: rewritten,
	}, nil
}

// SetFieldsIf writes frontmatter fields guarded per key: each key of set and
// unset is written only if it still has its value in old (a key missing from
// old, or nil there, must be absent). A key changed on disk comes back in
// Applied.Fields with its value there, and the others are written. Values are
// a string or a []string, as SetFields takes them; SetFields's refusals stand.
//
// The page's version moves and its blocks don't, so a field write never
// conflicts with a body write.
func (e *Entry) SetFieldsIf(old, set map[string]any, unset []string) (Applied, error) {
	if len(set) == 0 && len(unset) == 0 {
		return Applied{}, errors.New("no fields to set or unset")
	}
	set, err := fieldValues(set, false)
	if err != nil {
		return Applied{}, err
	}
	old, err = fieldValues(old, true)
	if err != nil {
		return Applied{}, err
	}
	for key := range set {
		if slices.Contains(unset, key) {
			return Applied{}, fmt.Errorf("field %q: both set and unset", key)
		}
	}
	if _, _, _, err := e.forWrite(); err != nil {
		return Applied{}, err
	}
	raw, err := e.frontmatterRaw()
	if err != nil {
		return Applied{}, err
	}
	disk, _ := parse.Frontmatter(raw)
	var fields []FieldConflict
	out := raw
	keys := slices.Sorted(maps.Keys(set))
	for _, key := range append(keys, unset...) {
		if !sameValue(disk[key], old[key]) {
			fields = append(fields, FieldConflict{Key: key, Theirs: disk[key]})
			continue
		}
		switch v := set[key].(type) {
		case string:
			out, err = parse.SetField(out, key, v)
		case []string:
			out, err = parse.SetFieldList(out, key, v)
		default: // unset
			out, err = parse.UnsetField(out, key)
		}
		if err != nil {
			return Applied{}, e.named(err)
		}
	}
	if err := e.commit(raw, out); err != nil {
		return Applied{}, err
	}
	return Applied{Version: FileVersion([]byte(out)), Ops: []parse.Op{}, Conflicts: []parse.Conflict{}, Fields: fields}, nil
}

// forWrite reads the entry for a guarded write: its header, body and version,
// once it is known to be a file of the bundle's content.
func (e *Entry) forWrite() (header, body, version string, err error) {
	if !e.inBundle() {
		return "", "", "", ErrNotInBundle
	}
	if _, err := inBundle(e.root, e.abs); err != nil {
		return "", "", "", err
	}
	raw, err := e.Raw()
	if err != nil {
		return "", "", "", err
	}
	body = parse.Body(raw)
	return raw[:len(raw)-len(body)], body, FileVersion([]byte(raw)), nil
}

// fieldValues checks the values of a field write: a string or a []string, a
// []any of strings (what JSON decoding gives) taken as a []string, or, in old,
// nil for a key that must be absent.
func fieldValues(in map[string]any, old bool) (map[string]any, error) {
	out := make(map[string]any, len(in))
	for key, v := range in {
		switch x := v.(type) {
		case string, []string:
			out[key] = x
			continue
		case nil:
			if old {
				out[key] = nil
				continue
			}
		case []any:
			list := make([]string, 0, len(x))
			for _, item := range x {
				s, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("field %q: a list holds strings, got %T", key, item)
				}
				list = append(list, s)
			}
			out[key] = list
			continue
		}
		return nil, fmt.Errorf("field %q: value must be a string or []string, got %T", key, v)
	}
	return out, nil
}

// sameValue compares a value on disk (Frontmatter's string view) with the one
// the caller read.
func sameValue(disk, old any) bool {
	switch d := disk.(type) {
	case nil:
		return old == nil
	case string:
		o, ok := old.(string)
		return ok && o == d
	case []string:
		o, ok := old.([]string)
		return ok && slices.Equal(o, d)
	}
	return false
}

func blockHashes(bs []parse.Block) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Hash
	}
	return out
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
