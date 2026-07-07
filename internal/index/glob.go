package index

import (
	"path"
	"strings"
)

// matchGlob reports whether the root-absolute bundle path p matches pattern.
// Patterns are matched segment by segment against p's "/"-separated segments:
//
//   - a plain segment (or one with `*`, `?`, `[…]`) matches exactly one segment,
//     via path.Match, so `*` and `?` never cross a `/`;
//   - `**` matches zero or more whole segments, so `docs/**` covers everything
//     under docs/ and `**/tmp/**` matches tmp/ at any depth.
//
// A pattern with no wildcard is therefore an exact path match (a single file),
// which is why `ignore = ["AGENTS.md"]` still works. Both pattern and p are
// leading-slash bundle paths (e.g. "/backlog/**", "/backlog/idea.md").
//
// The path is `path.Clean`ed first so any `.`/`..` is resolved before matching
// (bundle entry paths never contain them, but this keeps the matcher correct for
// any caller). The pattern is not cleaned: a `..` there would swallow a `**`.
func matchGlob(pattern, p string) bool {
	if p != "" {
		p = path.Clean(p)
	}
	return matchSegs(splitSlash(pattern), splitSlash(p))
}

// matchAnyGlob reports whether p matches any of the patterns.
func matchAnyGlob(patterns []string, p string) bool {
	for _, pat := range patterns {
		if matchGlob(pat, p) {
			return true
		}
	}
	return false
}

func splitSlash(s string) []string {
	if s = strings.Trim(s, "/"); s == "" {
		return nil
	}
	return strings.Split(s, "/")
}

// matchSegs matches pattern segments against name segments, with `**` consuming
// zero or more segments (backtracking) and every other segment matching exactly
// one via path.Match.
func matchSegs(pat, name []string) bool {
	if len(pat) == 0 {
		return len(name) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(name); i++ {
			if matchSegs(pat[1:], name[i:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	if ok, err := path.Match(pat[0], name[0]); err != nil || !ok {
		return false
	}
	return matchSegs(pat[1:], name[1:])
}
