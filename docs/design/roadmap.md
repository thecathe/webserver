---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-17
---

# Roadmap

Each milestone is one or more branches, merged `--no-ff` into `main`. Steps that touch the outside world (GitHub
transfers, droplets, DNS, Tailscale, tokens, cutover) stop and wait for your go-ahead.

| | Milestone | Status |
|---|---|---|
| M0 | Rules and docs: this site, the AI marking, `DEVELOPMENT.md`, global rules | **done** 2026-10-16 |
| M1 | Spec v2 + `ddm` skeleton | **done** 2026-10-16 |
| M2 | Workflow runner + built-ins | **done** 2026-10-16 |
| M3a | Publish, links, Caddy generation, local preview; one site of every kind | **done** 2026-10-17 |
| — | Shared-styling design session (D27), looking at the M3a preview | next |
| M3b | The remaining sites, and styling adapters per generator | |
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

## M3a: publish, links, Caddy, preview (done)
- `ddm/internal/publish`: `<www>/<view>/<site>/<time>-<sha>/` plus an atomically swapped `current`; keeps 3.
- `ddm/internal/links`: `site://` (and `mount://`) resolved at publish time (D28). Verified from mkdocs and from
  forest redirect trees in both forest configs.
- `ddm/internal/caddygen`: the Caddyfile from mounts. It covers wildcard certificates per domain via Cloudflare
  DNS-01, entry redirects, the shared 404 page, and the private gate (D30). It carries over everything useful from the
  deleted hand-written file, and drops its filesystem probing. It passes `caddy validate` with the Cloudflare plugin,
  which the devShell now builds.
- `site-directory`: the public directory (`docs.cathe.dev`) and the **private site map** (`the.cathe.dev/sites/`,
  D29).
- Spec: `access: public | private`, site-level `access`, `private_domain`, `root_only` (D33), and the `demos` site.
- `ddm publish`, `ddm caddyfile`, `ddm preview` (D31).
- One site of every kind publishes and serves in the preview:
  - forester: `forest`, all three views;
  - odoc: `cloakaml`;
  - ocaml-notebook: `demos`;
  - plain-markdown mkdocs: `indimo`;
  - themed mkdocs: `ddm`;
  - generated pages: `docs`, both views.
- **Nothing is live on the internet yet.** That needs the server (M5) and the cutover (M7). D25 still holds the
  forest's public and notes views until M4.

## After M3a (2026-10-17)
- The **OCaml docs pool** (D34): `cloakaml`, `mebi-plugin` and `membranes` built together, served at
  `ocaml.cathe.dev`, with build cost measured on your machine.
- Forest addresses `indimo.cathe.dev`, `mebi.cathe.dev`, `projects.cathe.dev`; INDIMO's mkdocs notes at
  `indimo-notes.cathe.dev` (D35).
- The **toolbar** on every page, with view switches and a status line (D36). `sites.cathe.dev` is the directory and
  `docs.cathe.dev` the documentation index (D37). On a package's own host, odoc's `Index` now opens
  `ocaml.cathe.dev`.

- **The forest's public view** builds from `forest-public.toml` (D39), with notes and recents as public pages
  (D38). Its XSLT is rendered at build time (D40). This brings forward M4's first item.

## Next: the shared-styling design session
Run `nix develop -c bin/ddm preview --serve-only`, then compare the six kinds side by side. Agenda:
- the palette, typography and AI marker ([shared styling](open-questions.md#shared-styling-across-sites), D27);
- the **shared header toolbar** and **section indexes**, including `docs.cathe.dev` → `sites.cathe.dev`
  ([site navigation](open-questions.md#site-navigation-a-shared-header-and-section-indexes));
- how far each generator can be themed, odoc especially;
- the status line (repo, branch, commit; "up to date" once the daemon exists);
- the collated OCaml docs page beyond the pool ([OCaml docs](open-questions.md#ocaml-docs-landing-pages-a-shared-index-shared-libraries)).

## M3b: the rest
- Styling adapters per generator, as designed in the session.
- The header toolbar, section index pages, and the `sites.cathe.dev` / `docs.cathe.dev` split.
- The collated OCaml docs index.
- Any remaining sites, and team project sites.
- odoc and `site://` (still untested).

## M4: forest side
Each repo gets its own branch and its own `DEVELOPMENT.md` entry.
- **forest, first**: `forest-public.toml` (D26), exposing only the `@jpears` and `@cathe` people pages, with `url`
  set to the public host. Point the public and notes views in `sites.yaml` at it (`params: { config:
  forest-public.toml }`), then rebuild and check that no other `@…` directory appears in the output. **This unblocks
  publishing the public views (D25).**
- **forest**: port INDIMO's notes into the forest, if you decide to
  ([open question](open-questions.md#indimos-notes-port-to-forester-into-the-main-forest)).
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

## Shared styling (design session next; D27)
- One source of truth for the palette (today: loose gruvbox in `docs/stylesheets/theme.css`), typography and the
  AI-generated marker, served from one place (e.g. an `assets` site, or a path on `ddm.cathe.dev`).
- Adapters per generator: mkdocs `extra_css`, the forester theme (`thecathe/forester-base-theme` XSLT and CSS), and
  odoc's `odoc.css` (`--theme-uri` or a post-build override).
- Possibly a shared header or nav linking the sites, using `site://` links.
- Designed in its own session, after M3a, against the preview. See
  [open questions](open-questions.md#shared-styling-across-sites).

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
| 6 DomainLink rewriting | M3a (`site://`, resolved at publish) |
| 7 GH Action rebuild, 8 systemd timer | M6 (ddm polling) |
| 9 home-manager on Ubuntu | dropped (D3) |
| 10 NixOS migration | M5 + M7 (in place, not blue/green: D21) |
| 11 webhooks | Later |
| 12 Caddyfile generation | M3a |
| 13 GitHub Pages | Later |
| 14 port `generate_docs.py` | Later |
