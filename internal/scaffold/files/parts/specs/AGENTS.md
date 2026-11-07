What the product is, one spec per concept, and how much of it is built. A spec describes the product, not the work to build it.

- **Specs** are files next to `{dir}index.md`, the product map, which links each one.
- **`{dir}technical-debt.md`** lists the workarounds and limitations you accept, one item each: what, why, how to fix it.

```markdown
---
type: spec
status: planned       # planned | partial | built
---

# Invoicing

What it is and how it behaves. Link the specs it relates to.

## Open questions

1. **Short name.** The question, the options, a recommendation.
```

- **`status`**: `planned` (nothing built yet), `partial` (say in the body what is), `built` (the body matches what shipped). When work changes what the product does, update the spec and its `status` with it.
- **Open questions** live in the spec they concern, under `## Open questions`. When one is answered, write the answer into the spec and remove the question, and the heading with the last one.

```sh
wiki property status --counts --where type=spec                 # how far the build is
wiki list --where type=spec --where status!=built               # what is left to build
wiki search --exact "## Open questions" --where type=spec       # what is waiting on a decision
```
