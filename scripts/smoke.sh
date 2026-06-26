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
type: index
title: Home
okf_version: "0.1"
---
# Home
- [Finance](/finance/index.md)
- [Tasks](/tasks/index.md)
MD

cat > "$TMP/finance/index.md" <<'MD'
---
type: index
title: Finance
---
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
---
type: index
title: Tasks
---
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
contains "$($BIN status)" "Entries:  6"
contains "$($BIN status)" "Broken:   1"

echo "--- check (broken link => exit 1) ---"
! $BIN check >/dev/null
contains "$($BIN check)" "/finance/q1-receipts.md"

echo "--- unresolved finds the broken link ---"
contains "$($BIN unresolved)" "/finance/q1-receipts.md"

echo "--- list --type concept / dataset ---"
contains "$($BIN list --type concept)" "/finance/income.md"
contains "$($BIN list --type dataset)" "/finance/expenses.md"

echo "--- list --tag out-of-pocket ---"
contains "$($BIN list --tag out-of-pocket)" "/finance/expenses.md"

echo "--- list --prefix filters to a subtree ---"
contains "$($BIN list --prefix finance/)" "/finance/income.md"

echo "--- tasks default = open only ---"
OPEN="$($BIN tasks)"
contains "$OPEN" "reconcile the bank statement"
! contains "$OPEN" "file the Q1 report"

echo "--- tasks --done ---"
DONE="$($BIN tasks --done)"
contains "$DONE" "file the Q1 report"
! contains "$DONE" "reconcile the bank statement"

echo "--- json output ---"
contains "$($BIN list --type concept --format json)" '"type": "concept"'

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

echo "--- search --type filters ---"
contains "$($BIN search --type dataset rent)" "/finance/expenses.md"

echo "--- search no match => exit 1 ---"
! $BIN search zzzznope >/dev/null

echo "--- links lists outgoing ---"
contains "$($BIN links /finance/index.md)" "/finance/income.md"

echo "--- backlinks lists incoming ---"
contains "$($BIN backlinks /finance/income.md)" "/finance/index.md"

echo "--- orphans (none => exit 1) ---"
! $BIN orphans >/dev/null

echo "--- no results => exit 1 ---"
! $BIN list --type nonexistent >/dev/null

echo "--- version ---"
contains "$($BIN version)" "wiki"

echo "--- init scaffolds a check-clean bundle ---"
mkdir -p "$TMP/fresh"
( cd "$TMP/fresh" && $BIN init >/dev/null && $BIN check >/dev/null )
test -f "$TMP/fresh/wiki.toml"
test -f "$TMP/fresh/.gitignore"

echo "--- move --dry-run previews, writes nothing ---"
contains "$($BIN move --dry-run /finance/expenses.md /finance/costs.md)" "would move"
test -f "$TMP/finance/expenses.md"

echo "--- move relocates and rewrites incoming links ---"
$BIN move /finance/expenses.md /finance/costs.md >/dev/null
test -f "$TMP/finance/costs.md"
! test -f "$TMP/finance/expenses.md"
contains "$($BIN links /finance/index.md)" "/finance/costs.md"

echo ""
echo "All smoke tests passed!"
