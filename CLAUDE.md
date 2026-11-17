# Project instructions

What this repository is, how it is laid out and how to build and test it are in README.md; the design is in the
docs site (`docs/`).

## Working mode
- Outward-facing steps (repo transfers, droplet creation, DNS, Tailscale keys/ACLs, GitHub tokens, cutover) always
  need the user's go-ahead first.

## Keeping the docs current
Periodically, and always before closing a session that changed or designed anything:
- Record design work in the shared devlog (this project's folder, per the global "Design write-ups" rule). This
  repo's docs stay documentation (what is decided, how to use it, what is open); the story, with the reasoning and
  what was found along the way, goes in the devlog, which is private by default.
- Update `docs/design/record.md` (living decisions; mark superseded ones rather than deleting) and
  `docs/design/open-questions.md`. Update `docs/design/roadmap.md` when milestones move.
- Bump `edited` in the `generated` front matter of every page touched.
- Add anything left for the user to do by hand (accounts, secrets, DNS, external links) to `docs/tasks.md`.
- Check the site still builds: `nix develop -c mkdocs build --strict`.

## AI-generated content marking
Every page Claude writes or substantially edits carries front matter:

```yaml
---
generated:
  by: Claude
  model: Opus 5.5        # the model that last edited the page
  effort: medium         # the session's effort level
  date: 2026-10-16       # first generated
  edited: 2026-10-16     # last edited
---
```

`model` and `effort` are those of the session that last edited the page. For the effort, use the `orient:` block's
"AI-marker effort" line; write `effort: unrecorded` when the model is not listed there or the level isn't known.
ddm's shared marker (D56, `ddm/internal/aimark/`) turns this into the yellow sticky
header: a mkdocs hook (`scripts/mkdocs_ddm_generated.py`) passes the front matter on, and `generated:` in `sites.yaml`
marks a whole site or regions of it. Pages the user writes by hand simply omit `generated`. `mkdocs serve` doesn't show
the header; `ddm preview` does. The convention is in `docs/design/record.md`.
