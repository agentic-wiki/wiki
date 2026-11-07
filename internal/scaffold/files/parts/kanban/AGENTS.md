The work, one task per file.

- **Read the goal** at the top of `{dir}index.md` before you pick a task, and keep it current.
- **Tasks** are files next to `{dir}index.md`, or in folders by epic or milestone when there are many (`{dir}m1-login/`).
- **Finished tasks**: `wiki move` one to `{dir}archive/` when it records a decision or why something was done. Delete the rest: git keeps them.

```markdown
---
type: task
title: Export invoices as CSV
status: todo          # backlog | todo | in-progress | blocked | done
priority: medium      # high | medium | low
assignee:             # who does it; empty while nobody does
tags: [feature]
---

What and why, and what "done" means. Link what it touches.

- [ ] step
```

- **`status`**: `backlog` (captured, not planned), `todo` (planned and ready), `in-progress`, `blocked` (say on what, in the body), `done`. Only these five.

```sh
wiki list --where type=task --where status=todo                          # what's next
wiki list --where type=task --where status=in-progress                   # what's under way
wiki list --where type=task --where assignee=ana --where status!=done    # ana's open tasks
wiki checkboxes --where type=task                                        # open steps
```
