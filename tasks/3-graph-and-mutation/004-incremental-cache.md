---
type: task
title: .wiki incremental cache
status: todo
priority: low
tags: [debt, perf]
---

Every invocation re-reads and re-parses the whole content tree. Fine at hundreds of files; at thousands the per-call latency will be felt, and efficiency is a core principle. Add a per-repo `.wiki/` cache (gitignored): persist the parsed index, re-parse only files whose mtime changed. Files stay the source of truth; the cache is disposable (rebuild on miss/corruption).
