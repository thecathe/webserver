---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Roadmap

Each milestone is one or more branches, merged `--no-ff` into `main`. Steps that touch the outside world (GitHub
transfers, droplets, DNS, Tailscale, tokens, cutover) stop and wait for your go-ahead.

| | Milestone | Status |
|---|---|---|
| M0 | Rules and docs: this site, the AI marking, `DEVELOPMENT.md`, global rules | **done** 2026-10-16 |
| M1 | Spec v2 + `ddm` skeleton | **done** 2026-10-16 |
| M2 | Workflow runner + built-ins | **done** 2026-10-16 |
| M3 | Publish, links, Caddy generation, `docs.cathe.dev` | next (consider the shared-styling design first) |
| M4 | Forest side: `forest-public.toml`, views, the notes filter in the fork | |
| M5 | NixOS host config | |
| M6 | ddm daemon | |
| M7 | Cutover | |

## M1: spec v2 + `ddm` skeleton (done)
- A new `ddm/` Go module. The legacy `webserver/`, `deploy.yaml`, hand-written `Caddyfile` and `deploy-dots` Action
  were deleted (D19).
- `ddm/internal/spec` loads `sites.yaml`. Unknown keys are errors, defaults are filled, and every problem is reported at
  once. It checks hosts outside `domains`, unknown workflows, views and builtins, `use:` cycles, duplicate host+path,
  undeclared taxa, and malformed names, entries and access values. Tests: `ddm/internal/spec/spec_test.go`, including
  one that keeps the repo's own `sites.yaml` valid.
- `ddm check` and `ddm sites` commands.
- The dead v1 fields (`auto_rebuild`, `email_notifications`, `flags`) have no v2 equivalent: their intent moved to
  `system.poll` and to the open question on notifications.
- `cathe.work` is gone from the repo along with the old Caddyfile (D20).

## M2: workflow runner (done)
- `ddm/internal/gitsync`: finds a branch's commit (`git ls-remote`) and checks out exactly that commit, cleaned of
  untracked files.
- `ddm/internal/workflow`: resolves a site view into a flat plan (params, `use:`, `when`/`unless`), then runs it in
  `nix develop <env>`. Built-in registry: `mkdocs-config` works; `rewrite-links` and `site-directory` are
  placeholders for M3.
- `ddm build` (via `bin/ddm`) builds one view into `.ddm/out/<site>/<view>/<commit>/`.
- Every workflow was verified on a real repo: forester (`forest`), odoc (`cloakaml`, `docs` branch), mkdocs (`indimo`,
  this site), ocaml-notebook (`demos`).
- Spikes: S1 and S3 pass, and S2 was answered here too (see [Spikes](#spikes)).
- **Found:** the forest's public and notes views currently publish people's profiles (D25). M4 must land before
  they go live.

## M3: publish, links, Caddy
- Versioned roots with a `current` symlink.
- `rewrite-links`.
- `internal/caddygen`: entry redirects, per-view roots, the `the.cathe.dev` bind + `forward_auth` block. It replaces
  the deleted hand-written Caddyfile. Carry over from it (see git history before
  2026-10-16):
  - certificates via `acme_dns cloudflare`, with a wildcard per domain;
  - a shared 404 page for every host;
  - odoc's root rewrite, since odoc output nests under the package name. This is now just `entry:` on the mount.
  - Drop its per-request filesystem probing (`@has_odoc` / `@has_site`): each mount knows its root.
- The `site-directory` built-in for `docs.cathe.dev`.
- **This site goes live here.** So can every site except the forest's public and notes views, which wait for M4
  (D25). The private view can go live, behind its tailnet-only host.

## M4: forest side
Each repo gets its own branch and its own `DEVELOPMENT.md` entry.
- **forest, first**: `forest-public.toml` (D26), exposing only the `@jpears` and `@cathe` people pages, with `url`
  set to the public host. Point the public and notes views in `sites.yaml` at it (`params: { config:
  forest-public.toml }`), then rebuild and check that no other `@…` directory appears in the output. **This unblocks
  publishing the public views (D25).**
- **forest**: taxa for posts, papers and works. A start on `@cathe` (the `cathe.dev` landing page) whenever you've
  decided what goes there.
- **vscode-forester** *(only if the hand-kept configs drift)*: a `site.yaml` reader and per-view TOML generator (D12).
- **ocaml-forester**: the notes-view tag filter.
- Set the forest's `url` in each config (it is still the template's `https://www.my-great-forest.net/`), so the
  `uri` attributes and "copy link" point at the real host.
- **Required before the forest's public and notes views go live (D25).**

## M5: NixOS host
`nixosConfigurations.droplet` in this repo's flake, with:
- Caddy (with plugins), `services.tailscale`, `services.tailscaleAuth`;
- agenix, firewall;
- the `ddm` package and service, and `ddm-rebuild.service`.

## M6: ddm daemon
- Both loops, the state file, self-update, health check and rollback, and the socket CLI.
- Tested in a NixOS VM test before any real droplet exists.

## M7: cutover
Reinstall the existing droplet in place (D21): wipe it, install NixOS from this flake, let ddm build every site. DNS
keeps pointing at the same IP. Downtime lasts from the wipe until the first deploy, kept short by proving M5/M6 in a
VM first. Then point teammates at project sites.

## Shared styling (not yet scheduled; D27)
- One source of truth for the palette (today: loose gruvbox in `docs/stylesheets/theme.css`), typography and the
  AI-generated marker, served from one place (e.g. an `assets` site, or a path on `ddm.cathe.dev`).
- Adapters per generator: mkdocs `extra_css`, the forester theme (`thecathe/forester-base-theme` XSLT and CSS), and
  odoc's `odoc.css` (`--theme-uri` or a post-build override).
- Possibly a shared header or nav linking the sites, using `site://` links.
- Likely after M3, since the shared assets need somewhere to be served from. To be designed in its own session;
  see [open questions](open-questions.md#shared-styling-across-sites).

## Later
- The identity widget (after S4).
- Webhooks for instant rebuilds.
- Migrating GitHub Pages sites (each old repo becomes a redirect stub).
- The `md-ssg` built-in (goldmark, front matter, one layout).
- Porting `generate_docs.py` to Go.
- Notifications.
- Redirect-only mounts for team sites.

## Spikes

| | Question | Answer |
|---|---|---|
| S1 | Does forester pass `site://name/path` through untouched? | **Yes** (M2): forester emits it verbatim as an external link, and so does mkdocs in strict mode. odoc is untested. |
| S2 | Are forest output links root-absolute or relative? (one build, two hosts) | **Root-absolute** (M2): `base-url="/"`, `href="//"`, stylesheet `/default.xsl`. One build serves `jpears.me` and `forest.cathe.dev` unchanged. A forest can only be mounted at path `/`. |
| S3 | Does `nix develop github:thecathe/forest/<sha> -c forester build` work on a clean machine? | **Yes for a clean checkout** (M2): about 10 s, with a warm Nix store. A cold store is verified in the M5 VM test. |
| S4 | Can a public page `fetch` a tailnet-only host? (the widget) | later |

## Where the old curriculum went

The 2026-09 curriculum (Modules 1–14) was written for you to implement. Its content maps onto the milestones like this:

| Old module | Now |
|---|---|
| 1 schema | M1 |
| 2 md SSG | Later (`md-ssg` built-in) |
| 3 mkdocs, 4 odoc, 5 ocaml | M2 (as workflows) |
| 6 DomainLink rewriting | M3 (`site://` + `rewrite-links`) |
| 7 GH Action rebuild, 8 systemd timer | M6 (ddm polling) |
| 9 home-manager on Ubuntu | dropped (D3) |
| 10 NixOS migration | M5 + M7 (in place, not blue/green: D21) |
| 11 webhooks | Later |
| 12 Caddyfile generation | M3 |
| 13 GitHub Pages | Later |
| 14 port `generate_docs.py` | Later |
