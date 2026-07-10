---
type: task
title: "switch canonical on-disk links to relative"
status: todo
priority: medium
tags: [feature, links]
---

**Decision made: Option B, relative-on-disk.** All internal links are stored **relative** on disk (they navigate everywhere: GitHub/GitLab, local file open, editors, Obsidian, `wiki`). Anything not relative is normalized to relative. **Internally the graph is unchanged**: every link still resolves to a root-absolute key (`normalizeLink`) and is normalized back to relative only when written. **Out-of-bundle links stay as they are now** (a `../PRD.md` above the bundle root, like a URL: left untouched, reported by `check` as out-of-bundle, never a graph edge). The assessment below is retained as the implementation spec; the "options" are settled in favor of B.

Today the canonical on-disk link is **root-absolute** (`[Income](/finance/income.md)`): `tidy --links` rewrites relative links to it, and AGENTS/spec/scaffold recommend it. Relative links are already *accepted* and resolved into the graph; the tool just canonicalizes them away. Internally, `normalizeLink` (`index.go`) resolves **both** forms to a root-absolute graph key, and each `Link` keeps its verbatim on-disk `Raw` alongside the resolved `Target`.

Proposal to evaluate: make the canonical on-disk form **relative**, while keeping the **internal** representation root-absolute (which is already how the index/graph work). On-disk = portable relative links; in-memory = stable absolute keys. Only the on-disk `Raw` form, what `tidy` emits, and the docs would change; the graph layer is untouched.

The motivating challenge: **rendering/navigation portability.** A root-absolute `/finance/income.md` is resolved from the *server/filesystem* root by generic Markdown renderers, so clicking it on **GitHub/GitLab web, a plain editor, or an opened local file breaks**. A **relative** link resolves from the file's own directory and navigates correctly everywhere. Obsidian is the outlier that makes root-absolute work (its "absolute path in vault" setting resolves from the vault root). So root-absolute is effectively a wiki-internal convention that only Obsidian-like tools honor, whereas relative renders anywhere. Confirm this matrix as step one.

Assess (the scope of this task, do not pre-decide):
- **Rendering/navigation matrix**: GitHub/GitLab web, local file open, VS Code preview, Obsidian, plain Markdown viewers, for both link forms. This is the crux; verify it rather than assume.
- **Move stability** (the cost of flipping): with root-absolute, moving the *source* file is free (its links are unchanged) and only moving a *target* rewrites inbound links. With relative, moving the *source* invalidates **all of its own outgoing links** (they must be recomputed from the new location) *and* moving a target rewrites inbound, more rewrite surface and more diff churn per move. `wiki move` can do it, but the write-back logic grows.
- **Authoring ergonomics**, especially for LLM agents: root-absolute is trivial to author (the bundle-root path is always known); relative needs `../` depth arithmetic and is error-prone. Note the tool already accepts both, so the *authoring* form and the *canonical* form need not be the same (author absolute, `tidy` to relative, or vice versa).
- **Readability**: `/finance/income.md` vs `../../finance/income.md` in a deep file.
- **`check` policy**: if relative becomes canonical, does `check` flag root-absolute (as `tidy --links` currently flags relative), or accept both silently? Today it flags neither.
- **Migration**: flipping means one `tidy --links` sweep rewrites the whole corpus (and the scaffold starters, and the dogfood backlog).
- Wikilink compat (`internal/wikilink`) is unaffected either way.

Options to weigh (pick with the user):
- **A. Keep root-absolute** (status quo): best internal model + agent authoring + move stability; loses generic-renderer navigation.
- **B. Flip canonical to relative on disk, absolute internal**: gains portable navigation; costs move churn + harder hand-authoring.
- **C. Accept both (already true), make `tidy` emit either** via a flag, and choose the default. Authoring stays easy; canonical form is a publish-time choice.
- **D. Per-bundle `wiki.toml` `link_style = relative | root-absolute`**: the bundle owner picks based on where it's browsed (GitHub repo vs Obsidian vault). Most flexible, most surface.

Code touchpoints when a direction is chosen: `normalizeLink` (keep, it already yields the absolute key), a new relativize helper (`filepath.Rel` over bundle paths, mirroring `normalizeLink`), the `tidy --links` direction, `move` write-back (recompute a moved file's outgoing relative links from its new dir), and a docs/spec/AGENTS/scaffold sweep. See [consolidate relative links](/3-graph-and-mutation/006-relative-link-lint.md) and [out-of-bundle links warn](/conformance/004-out-of-bundle-links.md) for the current resolver behavior.

## Assessment (findings, decision still needs sign-off)

**Key realization: the internal graph is already form-agnostic.** `normalizeLink` (`index.go`) canonicalizes *both* relative and root-absolute targets to a root-absolute `Target` key, and everything downstream (graph edges, `backlinks`, `orphans`, `unresolved`, `search`, `Resolve`/`byPath`, all query/JSON output) keys off that resolved `Target` and `_path`. So switching the on-disk form changes **nothing** in memory. The blast radius is only: (1) what gets *written* to disk (`move`, `tidy`), (2) `check` policy, (3) docs/scaffold.

### What breaks / needs work

- **`move` / rename, the biggest surface.** Two separate rewrite jobs, and relative flips both:
  - *Inbound links (TO the moved file):* today `move` writes the new target as `dest`, which is root-absolute (`index.go:730`). Relative-canonical needs each linking file's **own** spelling: `filepath.Rel(dir(linkingFile), dest)`, computed per source. New logic, not hard.
  - *The moved file's OWN outgoing links (FROM it):* today `move` leaves these untouched, and the comment "the moved file's own outgoing links stay valid" (`index.go:664`) is true **only because they're root-absolute**. Under relative-canonical, changing a file's directory invalidates every relative link inside it, so `move` must recompute them from the new location. This is a **brand-new responsibility** `move` doesn't have, and it is the heart of "what happens on rename."
  - *Nuance:* a pure rename **within the same directory** (what `Slugify` does via `Move`) leaves the file's own relative links valid (dir unchanged), so slug renames stay safe; only a cross-folder move triggers the new outgoing rewrite.
  - Wikilinks are unaffected (they resolve by basename every run, `move` already skips them).
- **`tidy --links` direction flips + a new helper.** `normalizeEntryLinks` today rewrites Relative→absolute via `normalizeLink`; it would instead rewrite Absolute→relative via a new `filepath.Rel(dir(entry), target)` helper. `normalizeLink` stays (still the resolver for the graph key). The space-target and out-of-bundle guards carry over unchanged. Flag help text flips.
- **Docs / spec / scaffold sweep.** AGENTS.md (the two link statements), `../spec/README.md`, and the scaffold starters. The starter `WORKFLOW.md`/`index.md`/entry files use root-absolute links and would be rewritten to relative, and must still scaffold **check-clean and tidy-clean** (existing tests assert this). Mechanical but broad.

### What does NOT break / is NOT needed

- The graph, `backlinks`, `orphans`, `unresolved`, `search`, `byPath`, `Resolve`, `read`/`outline`/`list`/`table`, JSON/CSV output (`_path` etc.): **zero change**, all keyed off the resolved root-absolute form.
- **`check`**: no change needed. It never flagged relative links (they're valid + resolved); it flags *broken* (both forms) and *out-of-bundle* regardless of spelling. Non-canonical spelling is a `tidy` concern, not a `check` error, so the policy is symmetric and untouched.
- Anchors (`#h`), `<…>` space-wrapping, link titles: the rewrite helpers already preserve these; the new relativize path inherits it.
- Out-of-bundle links (`../PRD.md` above root): already relative and left untouched; the `withinDir` guard is form-agnostic.
- Wikilink compat (`internal/wikilink`): unaffected either way.

### Rendering/navigation matrix (the motivating win, verify but expected)

| Tool | root-absolute `/finance/income.md` | relative `../finance/income.md` |
|---|---|---|
| GitHub / GitLab web | **broken** (resolves at the site root, not repo root) | works |
| Local file open / plain editor | **broken** (filesystem root) | works |
| VS Code Markdown preview | workspace-root-dependent / inconsistent | works |
| Obsidian | works (with "absolute path in vault") | works |
| `wiki` itself | works (its resolver handles both) | works |

So root-absolute is effectively a wiki-/Obsidian-internal convention; relative navigates in every generic Markdown context. This confirms the challenge that prompted the task.

### The real tradeoff (ergonomics vs portability)

- **Authoring**, esp. LLM agents: root-absolute is trivial (bundle-root path always known); relative needs `../` depth arithmetic → error-prone. *But* since the tool accepts both and `tidy` canonicalizes, an agent can keep authoring root-absolute and let `tidy --links` convert to relative at commit, so authoring need not regress.
- **Readability**: `/finance/income.md` (stable, obvious) vs `../../finance/income.md` (depth-noisy).
- **Diff churn**: relative makes a cross-folder `move` dirty the moved file's own body, not just inbound links.
- **Portability**: the relative win above.

### Implementation plan

1. **New relativize helper** in `index.go`, the inverse of `normalizeLink`: given the linking entry's dir and a resolved root-absolute target, return the on-disk relative spelling (`filepath.Rel`, `./x.md` / `../x.md`, anchor + `<…>` space-wrapping preserved). `normalizeLink` stays as the resolver that computes the graph key.
2. **`tidy --links` flips direction**: rewrite Absolute→relative (was Relative→absolute) in `normalizeEntryLinks`. Same space-target and out-of-bundle guards (out-of-bundle left exactly as authored). Flag help text flips.
3. **`move` gains the moved-file's own-outgoing-links rewrite.** Today it only rewrites inbound links, writing them as the root-absolute `dest` (`index.go:730`); now (a) inbound links are written relative to each linking file (`Rel(dir(source), dest)`), and (b) the moved file's own relative links are recomputed from its new directory. A same-directory rename (Slugify) needs only (a). This is the bulk of the work.
4. **`check`: no change** (it never flagged relative; broken/out-of-bundle stay form-agnostic).
5. **Docs/scaffold sweep**: AGENTS.md's two link statements, `../spec/README.md`, and all scaffold starters rewritten to relative links; starters must still scaffold **check-clean and tidy-clean** (assert in the scaffold tests). Update the "root-absolute" wording everywhere to "relative".
6. **Tests**: relativize helper (depths, siblings, anchors, spaces); `tidy --links` absolute→relative; `move` across folders rewriting both inbound (relative per source) and the moved file's own links; same-dir rename leaving own links intact; out-of-bundle untouched; smoke.

Note: internal graph, `backlinks`, `orphans`, `search`, `Resolve`, and all query/JSON output are untouched, they key off the resolved root-absolute form, which is computed the same way regardless of on-disk spelling.
