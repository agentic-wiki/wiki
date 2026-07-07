# How this backlog works

This is the **workflow** layer for a `project-backlog` wiki bundle: the conventions on top of what [AGENTS.md](AGENTS.md) and the `wiki` tool define. It is a starting point: **edit it to fit how your team works.**

The whole bundle is a **backlog**: a kanban-style tracker in plain Markdown. An issue is a file, a board is an `index.md`, and everything a tracker gives you (status, priority, epics, dependencies, cycles, teams) is frontmatter, tags, folders, and links: queryable with `wiki`, and yours forever.

## First run: pin your conventions

This file ships as a **template with options**. Before creating real issues, sit with the user and commit the primitives, then **edit this file to lock the choices** (delete the paths you will not take), so it becomes the team's actual playbook rather than a menu. Decide together:

- **Board axis:** group the board and lane folders by **scheduling** (`backlog` / `next` / `now`) or by **cycle** (`2026-w27`, ...). For classic kanban you can instead group by progress, so the columns *are* the statuses (`todo` / `doing` / `review` / `done`).
- **Progress statuses:** the set that tracks how far work has got, e.g. `todo` / `in-progress` / `in-review` / `blocked` / `done`. Separate from scheduling.
- **Work-kinds:** the tag vocabulary (`feature` / `bug` / `chore` / `debt`, plus your own).
- **Epics and milestones:** using them at all? Many teams do not.
- **Teams:** one board, or one per team?
- **Fields:** which of `priority` / `estimate` / `assignee` / `cycle` / `due` you actually track.

Then scaffold a **skeleton** the user validates: the chosen folders (each with its `index.md`), a board with the agreed sections and goal(s), and one sample issue in the real shape. Confirm it reads right, then fill in the rest. **When this is done, delete this "First run" section: it has served its purpose.**

## Structure

A single-team shape to steal:

```
my-backlog/
├── index.md          # the board: goals + Now / Next (scheduled work)
├── log.md            # optional: notable decisions and changes, dated
├── now/              # active issues (type: task)
├── next/             # committed for the upcoming cycle
├── backlog/          # unscheduled work
│   └── index.md      # the parking-lot list (keeps these linked; see below)
├── archive/          # shipped or dropped
│   └── index.md      # optional: lists retained items
├── epics/            # optional: type: epic, large bodies of work
├── milestones/       # optional: type: milestone, releases / targets
└── notes/            # specs, designs, retros (type: note)
```

Several teams? Give each its own board (see *Multiple teams*).

## Lifecycle

Three things track a task, and they stay in agreement: its **folder is the lane** (where it is in the journey), the **board (`index.md`) is the live view** of the scheduled lanes, and **`status` frontmatter is the finer state** within a lane. Let the tool do the moving:

1. **Capture.** A new item is a `type: task` file in `backlog/` (or straight into `next/` if already committed). Link it from `backlog/index.md` so it stays findable and is not a false orphan. `wiki list --type task --prefix backlog/` is the raw backlog.
2. **Schedule.** When you commit to an item, `wiki move` it into `next/` (or `now/`), add its `- [ ]` checkbox to the board, and drop it from `backlog/index.md`. `wiki move` rewrites every link to it, so nothing dangles.
3. **Work.** Inside a lane, update `status` in place (`todo`, `in-progress`, `in-review`, `blocked`); no file move needed. When it becomes active, `wiki move` it `next/` to `now/` and move its checkbox in the same change.
4. **Finish.** Set `status: done`, check its board box, then retire it (below).

The rule: **only scheduled work (Now / Next) sits on the board.** The unscheduled backlog lives in `backlog/`, listed by `backlog/index.md`.

## Keep everything linked

`wiki orphans` is only useful if a hit means a real mistake, not just parked work. So **every issue is linked from exactly one list**: scheduled issues from the board (`index.md`), unscheduled ones from `backlog/index.md`, archived ones from `archive/index.md`. An `index.md` is itself exempt from `orphans`, so listing entries there keeps the signal clean. Add or move an issue, and update the list it belongs to in the same change; then `wiki orphans` only ever surfaces a genuine filing slip, not your whole backlog.

## Issues

Every work item is a `type: task` entry. Two different questions describe it, and they stay apart: **scheduling** (when you will do it) is the **lane** an issue lives in (`backlog`, `next`, `now`, or a `cycle`); **progress** (how far the work has got) is its **`status`** (`todo`, `in-progress`, `in-review`, `done`, plus `blocked`). A `now/` issue nobody has started yet is `status: todo`, not `status: now`. Tracker attributes go in frontmatter; the work-kind is a tag:

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

The lane says *when*, `status` says *how far*, and they move independently. `wiki` treats both as generic properties, so you can slice the backlog any way: `wiki list --type task --tag bug`, `wiki property status --counts`, `wiki property assignee --counts`, `wiki list --type task --prefix teams/web/`.

**Technical debt** is just a `debt`-tagged task. Surface it with `wiki list --type task --tag debt`, and consider a standing `## Debt` lane on the board so it stays visible instead of quietly accruing.

## The board

Each `index.md` is a board: overarching **goal(s)** at the top, then sections of `- [ ]` checkboxes linking to the issue entries.

- Group sections by scheduling lane (`## Now` / `## Next`), by cycle (`## 2026-w27`), or, for classic kanban, by progress (`## Doing` / `## Review`); add a `## Debt` lane if you keep one. Keep the active section short and truly current.
- A checkbox is checked exactly when its entry's `status` is `done`. Update both in one change.
- The board carries scheduled work only; the unscheduled backlog is `backlog/index.md`. Link to it from the board so it stays one click away.

## Epics and milestones (optional)

Most tasks need neither. Reach for them only when they earn their keep, and **link only to the immediate parent**:

- An **epic** (`type: epic`) groups a large body of work. Its tasks link up to it (an `epic: /epics/onboarding.md` field or a body link), so `wiki backlinks /epics/onboarding.md` lists everything under it.
- A **milestone** (`type: milestone`) is a release or target. An epic (or a lone task that has no epic) links up to it.

So the chain is task, epic, milestone, walked with `wiki backlinks`. A task carries **one** parent link, not both, and the milestone is never copied onto every task, so it changes in one place. `wiki backlinks /milestones/v1.md` still shows everything working toward it.

## Dependencies

Model *blocks* / *blocked-by* as body links between issues. `wiki backlinks /now/tz-bug.md` shows what finishing it would unblock, and `wiki unresolved` surfaces links to issues or specs not written yet: candidate prerequisites.

## Multiple teams

For more than one team, give each its own board and issue lanes:

```
index.md              # program board: goals, a link to each team's board, milestones
teams/
├── web/
│   ├── index.md       # the web team's board
│   └── now/ next/ backlog/ archive/
└── mobile/
    └── index.md ...
milestones/           # type: milestone (releases / targets)
epics/                # type: epic (cross-team where needed)
```

Keep statuses, priorities, and work-kind tags consistent across teams, so `wiki property status --counts` and `wiki list --type task --tag bug` work program-wide and `--prefix teams/web/` scopes to one.

## Answering everyday questions

Most questions are a query plus a moment of judgment: `wiki` narrows the set, you read and decide.

- **"What's next?"**: read the board's `Now`; `wiki tasks` gathers every open checkbox across the base.
- **"What's next for John?"**: `wiki search "assignee: john"` finds John's issues (frontmatter is searched too), then read their `status` / `priority` and the board to say which is genuinely next.
- **"What's the status of X?"**: `wiki read /now/x.md` for its `status` and detail, plus `wiki backlinks /now/x.md` to see what it is blocking.
- **"What's blocked?"**: `wiki property status --counts` for the tally, then `wiki search "status: blocked"` to list them.
- **"What's in the backlog?"**: `wiki list --type task --prefix backlog/`.
- **"How much debt are we carrying?"**: `wiki list --type task --tag debt`.
- **"What's left for v1?"**: `wiki backlinks /milestones/v1.md`, minus the ones already `done`.

The commands find the candidates; you read the entries and apply judgment (priority, blockers, what the user actually meant). If you need answers beyond these (group tasks by topic, roll up by state, and so on), build a small skill that orchestrates the tool's output rather than asking `wiki` to grow a query language.

## Grooming

Keep the board an honest, lean snapshot of what's next, and the base discoverable and meaningful.

- Run `wiki check` and fix what it flags; reconcile any checkbox that disagrees with its entry's `status`.
- Review **stale** work (`wiki list --type task --sort=timestamp --reverse`), **blocked** work, and accumulating **debt** (`--tag debt`): is each still real? unblock, defer, drop, or schedule.
- `wiki orphans`: with everything linked (see *Keep everything linked*), a hit here is a real filing slip, so link it or retire it.
- **Retire done work** promptly (see *Retiring done work*), so the active board stays a snapshot of what is next.

## Retiring done work

The board is a lean snapshot of what is next, so finished and cancelled work leaves it. Per task, pick one (in batches, during grooming):

- **Delete it** (default for routine work): git keeps the history, and anything lasting (a decision, a shipped result) goes as a dated line in `log.md` or a promoted knowledge entry. Remove its board checkbox in the same change.
- **Archive it**: `wiki move` it to `archive/` and list it in `archive/index.md` (so it stays discoverable and is not an orphan), when a browsable record is worth keeping. Find archived work with `wiki list --type task --prefix archive/`.

## Make it yours

Nothing here is fixed beyond "every entry has a `type`." Single team or many, lanes or cycles, epics or flat: reshape this file and the folders to match how you actually run work.
