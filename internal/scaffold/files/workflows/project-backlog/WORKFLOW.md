# How this backlog works

This is the **workflow** layer for a `project-backlog` wiki bundle: the conventions on top of what [AGENTS.md](AGENTS.md) and the `wiki` tool define. It is a starting point: **edit it to fit how your team works.**

The whole bundle is a **backlog**: a kanban-style tracker in plain Markdown. An issue is a file, a board is an `index.md`, and everything a tracker gives you — status, priority, epics, dependencies, cycles, teams — is frontmatter, tags, folders, and links, queryable with `wiki` and yours forever.

## Structure

A single-team shape to steal (a board, plus issues grouped by state):

```
my-backlog/
├── index.md          # the board: goals + Now / Next / Sometime
├── log.md            # optional: important decisions/changes made
├── now/              # active issues (type: task)
├── next/
├── sometime/
├── archive/          # shipped or dropped
├── epics/            # type: epic — large bodies of work
├── milestones/       # type: milestone — releases / targets
└── notes/            # specs, designs, retros (type: note)
```

Several teams? Give each its own board (see *Multiple teams*).

## Issues

Every work item is a `type: task` entry. Tracker attributes go in frontmatter; the work-kind is a tag:

```markdown
---
type: task
status: in-progress        # backlog | todo | in-progress | in-review | done | cancelled
priority: high             # urgent | high | medium | low
estimate: 3                # your unit (points, days) — optional
assignee: john             # optional
cycle: 2026-w27            # sprint / iteration — optional
tags: [feature]            # work-kind: feature | bug | chore | debt | …
---

Problem statement and acceptance criteria. Link the spec, the epic, and any blockers.
```

`wiki` treats these as generic properties, so you can slice the backlog any way: `wiki list --type task --tag bug`, `wiki property status --counts`, `wiki property assignee --counts`, `wiki list --type task --prefix teams/web/`.

**Technical debt** is just a `debt`-tagged task. Surface it with `wiki list --type task --tag debt`, and consider a standing `## Debt` lane on the board so it stays visible instead of quietly accruing.

## The board

Each `index.md` is a board: overarching **goal(s)** at the top, then columns of `- [ ]` checkboxes linking to the issue entries.

- Group columns by **status** (`## Now` / `## In review` / `## Debt` / …) or by **cycle** (`## 2026-w27`). Keep the active set short and truly next.
- A checkbox is checked exactly when its entry's `status` is `done` — update both in one change.
- Issues live in folders by state or cycle (`now/`, `next/`, `sometime/`, `archive/`); move them with `wiki move` as they progress, so links follow.

## Epics and milestones

- An **epic** (`type: epic`) is a large body of work; its child tasks link to it (a `parent:` field or a body link), so `wiki backlinks /epics/onboarding.md` lists everything under it.
- A **milestone** (`type: milestone`) is a release or target; tasks link to it or carry a `milestone:` field, and `wiki backlinks /milestones/v1.md` shows what it still needs.

## Dependencies

Model *blocks* / *blocked-by* as body links between issues. `wiki backlinks /now/tz-bug.md` shows what finishing it would unblock, and `wiki unresolved` surfaces links to issues or specs not written yet — candidate prerequisites.

## Multiple teams

For more than one team, give each its own board and issue folders:

```
index.md              # program board: goals + a link to each team's board + milestones
teams/
├── web/
│   ├── index.md       # the web team's board
│   └── now/ next/ backlog/ archive/
└── mobile/
    └── index.md …
milestones/           # type: milestone (releases / targets)
epics/                # type: epic (cross-team where needed)
```

Keep statuses, priorities, and work-kind tags consistent across teams, so `wiki property status --counts` and `wiki list --type task --tag bug` work program-wide and `--prefix teams/web/` scopes to one.

## Answering everyday questions

Most questions are a query plus a moment of judgment — `wiki` narrows the set, you read and decide:

- **"What's next?"** — read the board's `Now`; `wiki tasks` gathers every open checkbox across the base.
- **"What's next for John?"** — `wiki search "assignee: john"` finds John's issues (frontmatter is searched too), then read their `status`/`priority` and the board to say which is genuinely next.
- **"What's the status of X?"** — `wiki read /now/x.md` for its `status` and detail, plus `wiki backlinks /now/x.md` to see what it's blocking.
- **"What's blocked?"** — `wiki property status --counts` for the tally, then `wiki search "status: blocked"` to list them.
- **"How much debt are we carrying?"** — `wiki list --type task --tag debt`.
- **"What's left for v1?"** — `wiki backlinks /milestones/v1.md`, minus the ones already `done`.

The commands find the candidates; you read the entries and apply judgment (priority, blockers, what the user actually meant). If you need answers beyond the above (group tasks by topic, state, etc) you may build your own skills in order to leverage scripts that orchestrate the tool's output.

## Grooming

Keep the board an honest, lean snapshot of what's next. Ensure that things are discoverable and meaningful.

- Run `wiki check` and fix what it flags; reconcile any checkbox that disagrees with its entry's `status`.
- Review **stale** work (`wiki list --type task --sort=timestamp --reverse`), **blocked** work, and accumulating **debt** (`--tag debt`) — is each still real? unblock, defer, drop, or schedule.
- `wiki orphans` — issue files nothing links to never reach a board; add or retire them.
- **Prune shipped work** to `archive/`, or delete it once its value is captured elsewhere (a dated line in `log.md`, or a promoted knowledge entry). The board is short-lived; the archive is the record.

## Make it yours

Nothing here is fixed beyond "every entry has a `type`." Single team or many, status columns or cycles, epics or flat — reshape this file and the folders to match how you actually run work.
