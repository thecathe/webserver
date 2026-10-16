---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Development log

A running record of how this project has been designed and built, so the interesting parts don't get lost:
trade-offs weighed, ideas tried and dropped, and what each session left behind. Entries are chronological, oldest
first, so the log reads as a narrative. The current state of each decision is in the decision record
(`docs/design/record.md`); this log is the story behind it.

| Tag | Meaning |
|---|---|
| **Design** | A decision or design exploration, whether or not anything was built. |
| **Feature** | New capability. |
| **Refactor** | Same behaviour, different structure. |
| **Bug fix** | Behaviour was wrong; now it isn't. |
| **Tooling** | Build, dev environment, CI or editor config. |
| **Docs** | Documentation only. |

---

## 2026-05 to 2026-09: before the log (back-filled)

Reconstructed from git history and the curriculum files that sessions 1–3 left in the repo. Commits before
2026-10-16 carry no `Co-Authored-By: Claude` trailer; the curriculum sessions produced plans, not commits.

- **2026-05-12/13: Tooling.** A first `Caddyfile` for `cathe.dev`, `cathe.work` and `jpears.me`, with wildcard
  subdomains that probe the filesystem per request for odoc output (`_html/`), mkdocs output (`site/`) or plain
  files. Certificates via Cloudflare DNS-01. The `deploy-dots` Action SSHes to the droplet and runs `git pull`.
  `e9c3688` "fixing exposed token -- oops" removed a token committed by accident, an early sign that secrets
  belong in exactly one place.
- **2026-08-04/07: Design.** The README sketches the idea: one repo with CI that tells the web server which repos to
  pull and how to build each. The goal is that hosting a new site never again means per-repo Actions and secrets,
  or moving a repo into a GitHub org. It also sketched the first `deploy.yaml`.
- **2026-09-06: Feature (partial)** (`8ae185a` "WIP: nix, yaml"). A Go module `webserver/` that embeds Caddy, with
  a `deploy` subcommand. Clone-or-reset sync, publishing to `/var/www/<domain>/subdomains/<name>` and Caddy reload
  all work. All four build methods (`md`, `mkdocs`, `odoc`, `ocaml`) are stubs. It came with a dev-shell flake and
  three real GitHub Actions workflows collected in `_example_yamls/` as the pipelines to replace.
- **2026-09 curriculum sessions 1–3: Design.** You chose to implement the system yourself, with Claude as
  architect/teacher, which produced a 14-module curriculum. Decisions from then:
  - **phased Nix**: home-manager on Ubuntu first, NixOS later by blue/green swap;
  - builds stay in Go, not Nix derivations (no network in the sandbox);
  - the `ocaml` method chains into mkdocs, and `generate_docs.py` stays Python;
  - `DomainLink{LinkTo, LinkPlaceholder, Path}` as a build-time find/replace for cross-site links. This was a
    properly designed version of the abandoned `sed` "Fix links" step in `cloakaml-deploy-docs.yml`.

  Session 3 reviewed a first attempt at Module 1 (moving `Linked` to `Repo`, adding validation) and found a
  namespace bug: `LinkTo` was checked against repo names, not domain hostnames. That commit, `32a1e49`, is no
  longer in the history. Its work presumably continued in the uncommitted changes discarded below. The session
  also weighed Hugo against a DIY goldmark SSG for `md`. Hugo is CLI-only and would just be a second "shell out to
  an external SSG", like mkdocs, so the DIY route was preferred for its learning value.

---

## 2026-10-16: from webserver to deployment manager; docs as a site

Branch `docs/dev-site`. **Design** + **Docs** + **Tooling**. No ddm code changed.

**What prompted it.** You wanted to make the forest's HTML output the main landing page, with links out to project
docs, blog posts, the CV, profile, papers and works. You also wanted a private version of the notes. Two threads
joined this:
- the older "dots-deployment-manager" idea, which pulls new commits, rebuilds the system config, updates itself,
  and builds a declared set of repos with reusable workflows;
- the earlier forest-privacy work: `redirect` meta, `[forest.tag.private] behaviour = "hide"`, and separate dev and
  webserver configs.

**The design journey.**
- *The private view* started as two candidate ideas: (a) a private build in an entirely separate directory, so no
  paths overlap; (b) a toggle usable only by an authenticated service or over Tailscale. They turned out to be
  complementary rather than alternatives. Separate builds into separate roots mean a public URL can never serve a
  private file. Because relative paths match across views, the "toggle" reduces to swapping the host. Access is
  then a question of how that host is reached.
- *Tailscale* had four options:
  - (A) network-only: bind to the tailnet IP;
  - (B) A plus `forward_auth` to tailscaled's whois, which gives per-user identity with stock Caddy and your own
    domain;
  - (C) the `caddy-tailscale` plugin: experimental, needs a custom build;
  - (D) `tsidp` OIDC: overkill.

  B was chosen. You asked for a widget that does nothing for anyone but you. It was deferred, because putting it on
  public pages reveals the private hostname and may hit browser private-network rules (spike S4).
- *Self-update* was the subtle part of "a manager that rebuilds the system it runs on". If ddm ran `nixos-rebuild`
  itself, switching to a generation with a new ddm would restart ddm mid-rebuild. The answer is to run the rebuild
  in its own unit (as NixOS's `system.autoUpgrade` does), so the restart only hits the daemon. A state file makes
  the restart harmless.
- *Caddy embedded in the Go binary* had been a reasonable early choice, but under self-update it means every ddm
  update restarts the web server. So Caddy is split out into its own NixOS service.
- *Fixed build methods vs workflows.* The `builders` map would need a ddm release for every new kind of site.
  Named YAML workflows with `run:` steps in a Nix env, Go built-ins, and `use:` composition make them data.
  Choosing the env as "the repo's own devShell where it has one" means ddm needn't know anything about forester or
  dune.
- *Cross-site links* generalised `DomainLink`. Instead of a configured placeholder per link, one fixed
  `site://name/path` scheme is resolved per view after every build.
- *Home-manager on Ubuntu* (the 2026-09 plan) no longer made sense once ddm owns the system config, so the droplet
  goes straight to NixOS.
- *Repos.* This repo stays separate from your machines' `thecathe/dots`, but moves back to your personal account.
  It's the one place holding secrets, as a single read-only GitHub token, so new projects need none. Polling
  (`git ls-remote`) over webhooks follows from the same goal: zero per-repo setup.
- *Hosts.* `jpears.me` lands on the forest's `/@jpears/`, `forest.cathe.dev` on `/index/`, `notes.cathe.dev` is a
  `notes`-tagged view, `the.cathe.dev` is private, and `docs.cathe.dev` is a directory generated from the spec, with
  each site classified by taxa. `cathe.work` is being dropped. The notes view needs a small forester-fork addition
  ("keep only trees with these tags"), because per-tag hide can't express it.
- *The forest's script-only TOML keys* move to a `site.yaml`, which generates each view's TOML, giving one source of
  truth.
- *Working mode changed.* You no longer have time to learn this by building it, so Claude implements, and the docs
  become the interface for learning to use it. That raised a second wish: keep the design process itself, for this
  and other projects, so it can be appreciated and iterated on later. Hence this log, a global rule to keep logs
  like it, and the docs becoming a site of their own.

**Rejected or deferred.**
- Same-host auth toggle (private files behind the public listener).
- `caddy-tailscale` and `tsidp` for now.
- The identity widget (deferred).
- Hugo (from session 3).
- Home-manager on Ubuntu.
- Per-repo webhooks as the primary trigger.

**Discarded work.** Uncommitted changes on `main` were discarded at your request, since spec v2 replaces that
schema. Their intent, recorded here:
- an `html` method for ready-made sites, which becomes a trivial workflow;
- `default_config` with required defaults (theme, colours), filled into each repo, which becomes workflow `params`
  defaults. The fill had a range-copy bug: it mutated the loop variable, so the defaults never reached the config;
- an `index` repo published to all three domains.

A patch was saved to the session scratchpad only, which is not durable.

**Built this session.**
- `b6cf86c`: **Tooling.** mkdocs and mkdocs-material in the devShell.
- `fa45e4a`: **Docs.** Project `CLAUDE.md`: integration method, working mode, update-the-docs-before-closing, and
  the AI marking convention.
- **Docs.** This site: the overview, usage (the v2 spec, adding a site, workflows, operations), and design (the
  decision record, architecture, open questions, roadmap). `initial-curriculum.md`, `porting-guide.md` and the
  untracked session-3 curriculum are folded into it and removed.
- **Feature (docs site).** The AI-generated page marker, as a trial. Pages with `generated` front matter turn pale
  yellow, with the generating model, effort and dates drawn large and faded behind the text.
- In `thecathe/dots`: global Claude rules for design write-ups and AI-content marking.

**Open questions raised:** see `docs/design/open-questions.md`. Its headings are spec naming, S1–S4, the notes
view, `site.yaml` scope, workflow envs, ddm runtime, infrastructure, team project sites, AI marking beyond this
site, the `cathe.work` sunset, and this site's host.
