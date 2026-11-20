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

// coversGlob reports whether pattern matches every path below the folder dir,
// at any depth, by matchGlob's rules. Once dir's segments are matched, what
// remains must match any one or more names: at least one `**`, plus at most
// one segment that matches any name (`*`). So `archive/**`, `**/tmp/**` and
// `archive/*/**` cover a folder, while `archive/*` (its direct files),
// `archive/*/*/**` (not its direct files) and `docs/**/*.md` (its .md files)
// do not.
func coversGlob(pattern, dir string) bool {
	if dir != "" {
		dir = path.Clean(dir)
	}
	return coversSegs(splitSlash(pattern), splitSlash(dir))
}

func coversSegs(pat, dir []string) bool {
	if len(dir) == 0 {
		return matchesAnyNames(pat)
	}
	if len(pat) == 0 {
		return false
	}
	if pat[0] == "**" {
		if matchesAnyNames(pat) {
			return true // this `**` takes the rest of dir, and what follows matches anything below
		}
		for i := 0; i <= len(dir); i++ {
			if coversSegs(pat[1:], dir[i:]) {
				return true
			}
		}
		return false
	}
	if ok, err := path.Match(pat[0], dir[0]); err != nil || !ok {
		return false
	}
	return coversSegs(pat[1:], dir[1:])
}

// matchesAnyNames reports whether pat matches every sequence of one or more
// names: at least one `**` (zero or more names), and at most one segment that
// matches any single name (`*`). A lone `*` matches exactly one name, so it
// does not.
func matchesAnyNames(pat []string) bool {
	globstar, single := false, 0
	for _, s := range pat {
		switch {
		case s == "**":
			globstar = true
		case strings.Trim(s, "*") == "":
			single++ // `*`: any one name
		default:
			return false
		}
	}
	return globstar && single <= 1
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
