---
name: hookish
description: A skill with exactly one hook, for proving a skill's declared hook actually intercepts a real event through the real composition root — see internal/app/ecosystem_test.go.
version: 1.0.0
permissions:
  hooks: [PreToolUse]
---

# Hookish

Exists only for the dynamic-hook-registration end-to-end test. Its one hook,
`guard`, blocks every `PreToolUse` it sees.
