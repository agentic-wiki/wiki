---
type: task
title: "docs: reframe the stack as format + tool + workflow"
status: todo
priority: medium
tags: [docs]
---

Now that `init` scaffolds the operating manual + workflow into the bundle ([workflow scaffold](../3-graph-and-mutation/005-workflow-scaffold.md)), the "three layers" framing in the wiki `README.md` and the spec README (currently *Markdown + tool + **skill***) is stale.

Reframe to **format + tool + workflow**: two of the three layers (format, workflow) now live in the bundle, only the tool is external. Keep the nuance settled in the design session: skills are **not** deprecated — a skill suits a general-purpose agent managing many things (Cowork, Hermes), while `AGENTS.md`/`WORKFLOW.md` suit a purpose-specific bundle where operating it is the agent's whole job.

Update: wiki `README.md` (intro + "The stack" section), `../spec` README (the three layers + the `wiki init` description), naming `AGENTS.md`/`WORKFLOW.md` as the scaffolded operating layer.
