# Project instructions

This repo is the server repo for **ddm**, the deployment manager: it pulls site repos, builds them with reusable
workflows, publishes them under `cathe.dev` / `jpears.me`, and (planned) rebuilds the server's own NixOS config.
The design lives in the docs site (`mkdocs.yml`, `docs/`), which is itself planned to be served at `ddm.cathe.dev`.

Code: `ddm/` (Go; `nix develop -c go -C ddm test ./...`; run it as `nix develop -c bin/ddm …`) reads `sites.yaml`. The
Python in `scripts/` has tests too (`nix develop -c python3 -m unittest discover -s scripts`). The legacy setup (`webserver/`,
`deploy.yaml`, the hand-written `Caddyfile`, the `deploy-dots` Action) was deleted on 2026-10-16: the droplet will be
wiped and reinstalled with NixOS, so nothing on it needs preserving.

## Git integration
- Integration branch: `main`.
- Integrate finished branches with a local `git merge --no-ff <branch>` into `main`. Never squash.
- Do not push or open PRs unless explicitly asked.

## Working mode
- Claude implements; the user is not working through the old curriculum (decided 2026-10-16). Explain decisions in
  the docs rather than in chat, so the user can learn to *use* the system fully without having to build it.
- Outward-facing steps (repo transfers, droplet creation, DNS, Tailscale keys/ACLs, GitHub tokens, cutover) always
  need the user's go-ahead first.

## Keeping the docs current
Periodically, and always before closing a session that changed or designed anything:
- Record design work in the shared devlog, folder `webserver/` (`~/Documents/git/thecathe/devlog/webserver/`,
  index at `webserver/index.md`), per the global "Design write-ups" rule: update the mini-project's entry (or start
  one) and its `index.md` line in the same devlog commit. `DEVELOPMENT.md` was moved there (`webserver/archive/`) on
  2026-10-22 and no longer exists here. This repo's docs stay documentation (what is decided, how to use it, what is
  open); the story, with the reasoning and what was found along the way, goes in the devlog. The devlog is private
  by default.
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
  effort: medium         # the session's effort level (medium for every session so far)
  date: 2026-10-16       # first generated
  edited: 2026-10-16     # last edited
---
```

`model` and `effort` are those of the session that last edited the page; write `effort: unrecorded` when the
session's level isn't known. ddm's shared marker (D56, `ddm/internal/aimark/`) turns this into the yellow sticky
header: a mkdocs hook (`scripts/mkdocs_ddm_generated.py`) passes the front matter on, and `generated:` in `sites.yaml`
marks a whole site or regions of it. Pages the user writes by hand simply omit `generated`. `mkdocs serve` doesn't show
the header; `ddm preview` does. The convention is in `docs/design/record.md`.
