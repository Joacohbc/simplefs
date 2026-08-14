# code-design

A Claude / agent skill encoding code design directives. It nudges the agent to
write self-documenting code, name things well, keep control flow flat, prefer composition and
dependency injection, and avoid premature optimization and over-abstraction.

## Install

```bash
npx skills add Joacohbc/code-design
```

List skills in the repo before installing:

```bash
npx skills add Joacohbc/code-design --list
```

## What's here

- `SKILL.md` — the skill (YAML frontmatter + directives). This is what agents load.

## Publishing

1. Push this repo to GitHub (or any git host).
2. Share the repo. Anyone can install with `npx skills add <owner/repo>`.
3. It surfaces on [skills.sh](https://skills.sh) automatically via install telemetry — no
   registry submission needed.
