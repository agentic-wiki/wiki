# How this backlog works

This is the **workflow** layer for a `project-backlog` wiki bundle: the conventions on top of what [AGENTS.md](/AGENTS.md) and the `wiki` tool define. It is a starting point: **edit it to fit how your team works.**

The whole bundle is a **backlog**: a kanban-style tracker in plain Markdown. An issue is a file, a board is an `index.md`, and everything a tracker gives you (status, priority, epics, dependencies, cycles, teams) is frontmatter, tags, folders, and links: queryable with `wiki`, and yours forever.

## First run: pin your conventions

This file ships as a **template with options**. Before creating real issues, sit with the user and commit the primitives, then **edit this file to lock the choices** (delete the paths you will not take), so it becomes the team's actual playbook rather than a menu. Decide together:

- **Board sections:** how you group the board's checkboxes, by **scheduling** (`## Now` / `## Next`), by **cycle** (`## 2026-w27`), or, for classic kanban, by **progress** (`## Doing` / `## Review`). This is a board-heading choice, not a folder layout: scheduling lives in the board, while folders only separate committed from parked work (see *Structure*).
- **Progress statuses:** the set that tracks how far work has got, e.g. `todo` / `in-progress` / `in-review` / `blocked` / `done`. Separate from scheduling.
- **Work-kinds:** the tag vocabulary (`feature` / `bug` / `chore` / `debt`, plus your own).
- **Epics and milestones:** using them at all? Many teams do not.
- **Teams:** one board, or one per team?
- **Fields:** which of `priority` / `estimate` / `assignee` / `cycle` / `due` you actually track.

Then scaffold a **skeleton** the user validates: the chosen folders (each with its `index.md`), a board with the agreed sections and goal(s), and one sample issue in the real shape. Confirm it reads right, then fill in the rest. **When this is done, delete this "First run" section: it has served its purpose.**

## Structure

Folders track **one** thing: whether an issue is committed, parked, or retired. Scheduling (Now vs Next) is the board's job, and progress is `status`, so neither needs a folder. A single-team shape to steal:

```
my-backlog/
├── index.md          # the board: goals + ## Now / ## Next sections (committed work)
├── log.md            # optional: notable decisions and changes, dated
├── active/           # committed work, linked from the board (type: task)
├── backlog/          # unscheduled / parked work (ignore_orphans'd; see wiki.toml)
├── archive/          # shipped or dropped (ignore_orphans'd)
├── epics/            # optional: type: epic, large bodies of work
├── milestones/       # optional: type: milestone, releases / targets
└── notes/            # specs, designs, retros (type: note)
```

Only three tracking folders, because scheduling stays in the board (moving a task Now↔Next is a board edit, not a file move) and progress stays in `status`. A file moves only when work crosses a real boundary: committed (`backlog/` → `active/`) or retired (→ `archive/`). Teams that genuinely want a filesystem view of scheduling *can* add lane folders (`now/`, `next/`) or cycle folders (`2026-w27/`), but that duplicates the board sections and adds a file move on every reschedule; prefer the board. Several teams? Give each its own board (see *Multiple teams*).

## Lifecycle

Three things track a task, and they stay in agreement: its **folder** says whether it is committed / parked / retired, the **board (`index.md`) is the scheduled view** (its `## Now` / `## Next` sections), and **`status` frontmatter is progress**. Let the tool do the moving:

1. **Capture.** A new item is a `type: task` file in `backlog/`. `wiki list --where type=task --prefix backlog/` is the raw backlog; keep a `backlog/index.md` too if you want a browsable list.
2. **Commit.** When you commit to an item, `wiki move` it from `backlog/` into `active/` and add its `- [ ]` checkbox under the board's `## Next` (or `## Now`). `wiki move` rewrites every link to it, so nothing dangles.
3. **Schedule & work.** Reschedule by moving the checkbox between `## Next` and `## Now` on the board, no file move. Advance `status` in place (`todo`, `in-progress`, `in-review`, `blocked`) as the work moves. The file stays in `active/` the whole time.
4. **Finish.** Set `status: done`, check its board box, then retire it (below).

The rule: **only committed work (everything in `active/`) sits on the board.** The parked backlog lives in `backlog/`, listed by `backlog/index.md`.

**The trap to avoid: don't turn scheduling or progress into folders.** Do not create `todo/`, `doing/`, `done/` (progress) or `now/`, `next/` (scheduling) folders and shuffle files between them as work advances or gets rescheduled. Both are *mutable* state, so encoding them as folders means a file move (and a matching board edit) on every change, and three things, folder, board section, and `status`, that must agree and drift. Keep scheduling in the board sections and progress in `status`; the only folder move is the real, infrequent boundary: `backlog/` → `active/` → `archive/`.

## Orphans and parked work

`wiki orphans` is only useful if a hit means a real mistake, not just parked work. The scaffolded `wiki.toml` sets `ignore_orphans = ["backlog/**", "archive/**"]`, so unscheduled and retired issues never show up as orphans even though nothing links to them (they stay fully indexed and searchable). That keeps `wiki orphans` a clean signal: a hit is an *active* issue that lost its board link, not your whole backlog. Link entries for navigation when it helps, but never just to appease the report; extend `ignore_orphans` if you add more parked folders.

## Issues

Every work item is a `type: task` entry. Two different questions describe it, and they stay apart: **scheduling** (when you will do it) is the **board section** it sits under (`## Now` / `## Next`, or a `cycle`), optionally mirrored by a `cycle:` field; **progress** (how far the work has got) is its **`status`** (`todo`, `in-progress`, `in-review`, `done`, plus `blocked`). A task under `## Now` that nobody has started is `status: todo`, not `status: now`. Both are properties of the task, not its folder (the folder only says committed vs parked). Tracker attributes go in frontmatter; the work-kind is a tag:

```markdown
---
type: task
status: in-progress        # progress: todo | in-progress | in-review | blocked | done | cancelled
priority: high             # urgent | high | medium | low
estimate: 3                # points or days (optional)
assignee: john             # optional
cycle: 2026-w27            # sprint / iteration (optional)
tags: [feature]            # work-kind: feature | bug | chore | debt | ...
---

Problem statement and acceptance criteria. Link the spec, the epic, and any blockers.
```

The board section says *when*, `status` says *how far*, and they move independently. `wiki` treats every frontmatter field the same, so you slice the backlog with one flag: `wiki list --where type=task --where tags=bug`, `wiki list --where type=task --where status=blocked` (repeat `--where` for AND), `wiki property status --counts` (the tally), `wiki list --where type=task --prefix teams/web/`.

**Technical debt** is just a `debt`-tagged task. Surface it with `wiki list --where type=task --where tags=debt`, and consider a standing `## Debt` section on the board so it stays visible instead of quietly accruing.

## The board

Each `index.md` is a board: overarching **goal(s)** at the top, then sections of `- [ ]` checkboxes linking to the issue entries.

- Group sections by scheduling (`## Now` / `## Next`), by cycle (`## 2026-w27`), or, for classic kanban, by progress (`## Doing` / `## Review`); add a `## Debt` section if you keep one. Keep the active section short and truly current.
- A checkbox is checked exactly when its entry's `status` is `done`. Update both in one change.
- The board carries committed work only (everything in `active/`); the parked backlog is `backlog/index.md`. Link to it from the board so it stays one click away.

## Epics and milestones (optional)

Most tasks need neither. Reach for them only when they earn their keep, and **link only to the immediate parent**:

- An **epic** (`type: epic`) groups a large body of work. Its tasks link up to it (an `epic: /epics/onboarding.md` field or a body link), so `wiki backlinks /epics/onboarding.md` lists everything under it.
- A **milestone** (`type: milestone`) is a release or target. An epic (or a lone task that has no epic) links up to it.

So the chain is task, epic, milestone, walked with `wiki backlinks`. A task carries **one** parent link, not both, and the milestone is never copied onto every task, so it changes in one place. `wiki backlinks /milestones/v1.md` still shows everything working toward it.

## Dependencies

Model *blocks* / *blocked-by* as real Markdown links between issues, not prose like `Depends on: tz-bug`: only a link is an edge the graph can follow. `wiki backlinks /active/tz-bug.md` then shows what finishing it would unblock, and `wiki unresolved` surfaces links to issues or specs not written yet: candidate prerequisites.

## Multiple teams

For more than one team, give each its own board and folders:

```
index.md              # program board: goals, a link to each team's board, milestones
teams/
├── web/
│   ├── index.md       # the web team's board (## Now / ## Next)
│   └── active/ backlog/ archive/
└── mobile/
    └── index.md ...
milestones/           # type: milestone (releases / targets)
epics/                # type: epic (cross-team where needed)
```

Keep statuses, priorities, and work-kind tags consistent across teams, so `wiki property status --counts` and `wiki list --where type=task --where tags=bug` work program-wide and `--prefix teams/web/` scopes to one.

## Answering everyday questions

Most questions are a query plus a moment of judgment: `wiki` narrows the set, you read and decide.

- **"What's next?"**: read the board's `Now`; `wiki tasks` gathers every open checkbox across the base.
- **"What's next for John?"**: `wiki list --where type=task --where assignee=john` lists John's issues exactly (an exact frontmatter match, not a substring scan like `search`), then read their `status` / `priority` and the board to say which is genuinely next.
- **"What's the status of X?"**: `wiki read /active/x.md` for its `status` and detail, plus `wiki backlinks /active/x.md` to see what it is blocking.
- **"What's blocked?"**: `wiki property status --counts` for the tally, then `wiki list --where type=task --where status=blocked` to list them.
- **"What's in the backlog?"**: `wiki list --where type=task --prefix backlog/`.
- **"How much debt are we carrying?"**: `wiki list --where type=task --where tags=debt`.
- **"What's left for v1?"**: `wiki backlinks /milestones/v1.md`, minus the ones already `done`.

The commands find the candidates; you read the entries and apply judgment (priority, blockers, what the user actually meant). Anything beyond a filter (sprint velocity, a burndown, cycle time, roll-ups by state) is a small skill over `wiki list --format json`, which carries every frontmatter field: that reporting is the skill layer's job, not a `wiki` feature. Don't reach for a query language `wiki` deliberately does not have.

## Grooming

Keep the board an honest, lean snapshot of what's next, and the base discoverable and meaningful.

- Run `wiki check` and fix what it flags; reconcile any checkbox that disagrees with its entry's `status`.
- Review **stale** work (`wiki list --where type=task --sort=timestamp --reverse`), **blocked** work, and accumulating **debt** (`--where tags=debt`): is each still real? unblock, defer, drop, or schedule.
- `wiki orphans`: `backlog/` and `archive/` are `ignore_orphans`'d, so a hit here is an active issue that lost its board link; re-link or retire it.
- **Retire done work** promptly (see *Retiring done work*), so the active board stays a snapshot of what is next.

## Retiring done work

The board is a lean snapshot of what is next, so finished and cancelled work leaves it. Per task, pick one (in batches, during grooming):

- **Delete it** (default for routine work): git keeps the history, and anything lasting (a decision, a shipped result) goes as a dated line in `log.md` or a promoted knowledge entry. Remove its board checkbox in the same change.
- **Archive it**: `wiki move` it to `archive/`, when a browsable record is worth keeping. `archive/` is `ignore_orphans`'d, so it will not clutter `wiki orphans`; find archived work with `wiki list --where type=task --prefix archive/`.

## Make it yours

Nothing here is fixed beyond "every entry has a `type`." Single team or many, lanes or cycles, epics or flat: reshape this file and the folders to match how you actually run work.
