#!/usr/bin/env bash
# End-to-end smoke test for wiki: builds a tiny finance-themed bundle in a temp
# dir (content at the bundle root, beside wiki.toml) and checks the core
# commands, filters, and exit codes from a deep subdir to exercise upward
# discovery of wiki.toml.
set -euo pipefail

BIN="${1:-./bin/wiki}"
BIN="$(cd "$(dirname "$BIN")" && pwd)/$(basename "$BIN")"

TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

mkdir -p "$TMP/finance" "$TMP/tasks"

cat > "$TMP/wiki.toml" <<'TOML'
spec = "0.1"
types = ["note", "concept", "task", "dataset"]
TOML

cat > "$TMP/index.md" <<'MD'
---
okf_version: "0.1"
---
# Home
- [Finance](/finance/index.md)
- [Tasks](/tasks/index.md)
MD

cat > "$TMP/finance/index.md" <<'MD'
- [Income](/finance/income.md)
- [Expenses](/finance/expenses.md)
MD

cat > "$TMP/finance/income.md" <<'MD'
---
type: concept
title: Income
tags: [finance, income]
---
Monthly income tracking. Backup: [Q1 receipts](/finance/q1-receipts.md).
MD

cat > "$TMP/finance/expenses.md" <<'MD'
---
type: dataset
title: Expenses
tags: [finance, out-of-pocket]
---
| month   | category  | amount_eur |
|---------|-----------|------------|
| 2026-01 | rent      | 900        |
| 2026-01 | groceries | 220        |
MD

cat > "$TMP/tasks/index.md" <<'MD'
- [Reconcile accounts](/tasks/reconcile.md)
MD

cat > "$TMP/tasks/reconcile.md" <<'MD'
---
type: task
title: Reconcile accounts
status: open
---
- [ ] reconcile the bank statement
- [x] file the Q1 report
MD

contains() { echo "$1" | grep -q "$2"; }
cd "$TMP/finance"   # deep dir: discovery must walk up

echo "--- status ---"
contains "$($BIN status)" "Entries:    6"
contains "$($BIN status)" "Broken:     1"

echo "--- check: broken link is a warning, not an error (exit 0) ---"
$BIN check >/dev/null   # broken link no longer fails the lint
contains "$($BIN check)" "/finance/q1-receipts.md"
contains "$($BIN check)" "warning"

echo "--- undeclared type errors when a vocabulary is declared (opt-in) ---"
printf -- '---\ntype: bogustype\n---\nx\n' > "$TMP/finance/bogus.md"
! $BIN check >/dev/null 2>&1                          # undeclared type => exit 1
contains "$($BIN check 2>&1 || true)" "bogustype"     # and the error names it
rm "$TMP/finance/bogus.md"
$BIN check >/dev/null                                 # clean again (only the pre-existing warning)

echo "--- unresolved finds the broken link ---"
contains "$($BIN unresolved)" "/finance/q1-receipts.md"

echo "--- list --where type=concept / dataset ---"
contains "$($BIN list --where type=concept)" "/finance/income.md"
contains "$($BIN list --where type=dataset)" "/finance/expenses.md"

echo "--- table extracts a dataset's markdown table (csv) ---"
contains "$($BIN table /finance/expenses.md --format csv)" "month,category,amount_eur"
contains "$($BIN table /finance/expenses.md --format csv)" "2026-01,rent,900"

echo "--- list --where tags=out-of-pocket ---"
contains "$($BIN list --where tags=out-of-pocket)" "/finance/expenses.md"

echo "--- list --prefix filters to a subtree ---"
contains "$($BIN list --prefix finance/)" "/finance/income.md"

echo "--- list --sort=timestamp orders results ---"
contains "$($BIN list --sort=timestamp)" "/finance/income.md"

echo "--- aliases: ls = list, mv = move ---"
contains "$($BIN ls --where type=concept)" "/finance/income.md"
contains "$($BIN mv --dry-run /finance/income.md /finance/costs.md)" "would move"

echo "--- tags lists tags (with counts) ---"
contains "$($BIN tags --counts --sort=count)" "finance"

echo "--- properties lists frontmatter keys in use ---"
contains "$($BIN properties)" "status"

echo "--- property enumerates a key's values ---"
contains "$($BIN property type --counts)" "dataset"
contains "$($BIN property status)" "open"

echo "--- property unknown key => empty, exit 0 (like ls) ---"
$BIN property zzznope >/dev/null

echo "--- checkboxes default = open only ---"
OPEN="$($BIN checkboxes)"
contains "$OPEN" "reconcile the bank statement"
! contains "$OPEN" "file the Q1 report"

echo "--- checkboxes --done ---"
DONE="$($BIN checkboxes --done)"
contains "$DONE" "file the Q1 report"
! contains "$DONE" "reconcile the bank statement"

echo "--- json output ---"
contains "$($BIN list --where type=concept --format json)" '"type": "concept"'

echo "--- csv output: header row from json fields ---"
contains "$($BIN list --where type=concept --format csv)" "_path,type"

echo "--- tsv output: tab-separated header ---"
contains "$($BIN property type --format tsv)" "$(printf 'name\tcount')"

echo "--- read strips frontmatter ---"
READ="$($BIN read /finance/income.md)"
contains "$READ" "Monthly income tracking"
! contains "$READ" "type: concept"

echo "--- outline lists headings ---"
contains "$($BIN outline /index.md)" "Home"

echo "--- read missing file => exit 2 ---"
! $BIN read /nope.md 2>/dev/null

echo "--- search finds content ---"
contains "$($BIN search income)" "/finance/income.md"

echo "--- search --lines shows file:line ---"
contains "$($BIN search --lines income)" "/finance/income.md:"

echo "--- search --where filters ---"
contains "$($BIN search --where type=dataset rent)" "/finance/expenses.md"

echo "--- search modes: all (default) / --any / --exact ---"
contains "$($BIN search 'monthly tracking')" "/finance/income.md"          # default AND: both words, same line
! $BIN search "monthly rent" >/dev/null                                     # AND: words in different files => no match
contains "$($BIN search --any 'monthly rent')" "/finance/income.md"         # --any: either word matches
! $BIN search --exact "monthly tracking" >/dev/null                         # not a verbatim phrase => exit 1

echo "--- search no match => exit 1 ---"
! $BIN search zzzznope >/dev/null

echo "--- links lists outgoing ---"
contains "$($BIN links /finance/index.md)" "/finance/income.md"

echo "--- backlinks lists incoming ---"
contains "$($BIN backlinks /finance/income.md)" "/finance/index.md"

echo "--- orphans (none => empty, exit 0) ---"
$BIN orphans >/dev/null

echo "--- no results => empty, exit 0 (like ls) ---"
$BIN list --where type=nonexistent >/dev/null

echo "--- version ---"
contains "$($BIN version)" "wiki"

echo "--- init scaffolds a check-clean bundle with an operating manual ---"
mkdir -p "$TMP/fresh"
( cd "$TMP/fresh" && $BIN init >/dev/null && $BIN check >/dev/null )
for f in wiki.toml .gitignore index.md AGENTS.md CLAUDE.md WORKFLOW.md; do test -e "$TMP/fresh/$f"; done
contains "$(cat "$TMP/fresh/wiki.toml")" "ignore"             # meta files declared non-entries
( cd "$TMP/fresh" && ! contains "$($BIN list)" "AGENTS" )   # ...so they are not indexed as entries

echo "--- check warns on a filename with a space ---"
printf -- '---\ntype: note\n---\nx\n' > "$TMP/fresh/has space.md"
( cd "$TMP/fresh" && contains "$($BIN check)" "space" )
rm "$TMP/fresh/has space.md"

echo "--- check --fix syncs okf_version drift ---"
sed -i.bak 's/okf_version: "0.1"/okf_version: "0.2"/' "$TMP/fresh/index.md" && rm -f "$TMP/fresh/index.md.bak"
( cd "$TMP/fresh" && contains "$($BIN check)" "okf_version" )   # drift is flagged
( cd "$TMP/fresh" && contains "$($BIN check --fix)" "fixed" )   # and repaired
( cd "$TMP/fresh" && $BIN check >/dev/null )                    # now clean
contains "$(cat "$TMP/fresh/index.md")" 'okf_version: "0.1"'

echo "--- tidy --links normalizes links to relative (absolute is valid, not flagged) ---"
mkdir -p "$TMP/fresh/notes"
printf -- '---\ntype: note\n---\nhi\n[home](/index.md)\n' > "$TMP/fresh/notes/example.md"
( cd "$TMP/fresh" && $BIN check >/dev/null )                                    # absolute resolves, so still clean
( cd "$TMP/fresh" && contains "$($BIN tidy)" "would link" )                     # bare tidy = preview, writes nothing
( cd "$TMP/fresh" && $BIN tidy --links >/dev/null )                             # normalize to relative
contains "$(cat "$TMP/fresh/notes/example.md")" 'home](../index.md)'           # /index.md -> ../index.md from notes/

echo "--- ignore: an out-of-bundle ref can be acknowledged in wiki.toml ---"
OOB="$TMP/oob"; mkdir -p "$OOB"
printf 'spec = "0.1"\ntypes = ["note"]\n' > "$OOB/wiki.toml"
printf -- '---\nokf_version: "0.1"\n---\n[prd](../PRD.md)\n' > "$OOB/index.md"
( cd "$OOB" && contains "$($BIN check)" "out-of-bundle" )     # unacknowledged out-of-bundle ref warns
printf 'spec = "0.1"\ntypes = ["note"]\nignore = ["../PRD.md"]\n' > "$OOB/wiki.toml"
( cd "$OOB" && ! contains "$($BIN check)" "out-of-bundle" )   # once listed in ignore, silenced

echo "--- move --dry-run previews, writes nothing ---"
contains "$($BIN move --dry-run /finance/expenses.md /finance/costs.md)" "would move"
test -f "$TMP/finance/expenses.md"

echo "--- move relocates and rewrites incoming links ---"
$BIN move /finance/expenses.md /finance/costs.md >/dev/null
test -f "$TMP/finance/costs.md"
! test -f "$TMP/finance/expenses.md"
contains "$($BIN links /finance/index.md)" "/finance/costs.md"

echo "--- wikilinks: recognized as graph edges, flagged by check ---"
WL="$TMP/wl"; mkdir -p "$WL/sub"
printf 'spec = "0.1"\ntypes = ["note"]\n' > "$WL/wiki.toml"
printf -- '---\nokf_version: "0.1"\n---\n[a](/a.md)\n' > "$WL/index.md"
printf -- '---\ntype: note\n---\nSee [[b]] and [[sub/c|the C]].\n' > "$WL/a.md"
printf -- '---\ntype: note\n---\nplain\n' > "$WL/b.md"
printf -- '---\ntype: note\n---\nplain\n' > "$WL/sub/c.md"
( cd "$WL" && contains "$($BIN backlinks /b.md)" "/a.md" )         # [[b]] is a real backlink
( cd "$WL" && contains "$($BIN backlinks /sub/c.md)" "the C" )     # [[sub/c|display]] resolves, keeps display
( cd "$WL" && ! contains "$($BIN orphans)" "/b.md" )              # wiki-linked, so not an orphan
( cd "$WL" && contains "$($BIN check)" "wikilink" )               # but check flags it as non-standard

echo "--- wikilinks: survive a relocation (re-resolve by basename, [[…]] left as-is) ---"
( cd "$WL" && $BIN move /b.md /sub/b.md >/dev/null )
( cd "$WL" && contains "$($BIN backlinks /sub/b.md)" "/a.md" )    # [[b]] now resolves to the moved file
( cd "$WL" && contains "$(cat a.md)" '\[\[b\]\]' )               # move did not rewrite the wikilink text

echo "--- tidy --wikilinks converts them to standard markdown ---"
( cd "$WL" && $BIN tidy --wikilinks >/dev/null )
( cd "$WL" && contains "$(cat a.md)" '\[b\](./sub/b.md)' )       # [[b]] -> [b](./sub/b.md) (relative)
( cd "$WL" && contains "$(cat a.md)" '\[the C\](./sub/c.md)' )   # [[sub/c|the C]] -> [the C](./sub/c.md)
( cd "$WL" && ! contains "$(cat a.md)" '\[\[' )                  # no wikilinks left
( cd "$WL" && ! contains "$($BIN check)" "wikilink" )            # and check no longer flags any

echo ""
echo "All smoke tests passed!"
