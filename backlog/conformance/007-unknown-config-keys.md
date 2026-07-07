---
type: task
title: "check warns on unknown wiki.toml keys"
status: done
priority: medium
tags: [conformance, config, dx]
---

An unrecognized `wiki.toml` key was silently ignored, so a typo or a renamed field did nothing with no signal. Surfaced by stress-test feedback: a bundle still using the pre-rename `skip` (now `ignore`) got no ignore behavior *and* no warning, only the downstream confusion of its meta files flagged as missing `type`.

Done: `bundle.parseConfig` collects any key outside {`spec`, `types`, `ignore`, `ignore_orphans`} into `Bundle.Unknown`; `index.Check` emits one `unknown wiki.toml key: <k>` **warning** per key (warning, so exit stays 0, it never breaks CI, it just makes the footgun visible). Tests: `TestParseConfigUnknownKeys` (bundle), `TestCheckUnknownConfigKey` (index).
