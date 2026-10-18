---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-18
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
| — | Shared-styling design session (D27), looking at the M3a preview | **done** 2026-10-17 (D43) |
| M3b | The remaining sites, and styling adapters per generator | |
| M4 | Forest side: `forest-public.toml`, views, the notes filter in the fork | |
| M5 | NixOS host config | **built** 2026-10-18; VM test passes; install next (needs the GitHub token) |
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
- **Nothing is live on the internet yet.** That needs the server (M5) and the cutover (M7). (D25 was resolved later
  the same day by D39 and D42.)

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

## The shared-styling design session (done 2026-10-17)
Decided in D43: Gruvbox; Source Serif, Sans and Code Pro; a light/dark toggle in the toolbar; tokens and adapters
linked from `/_ddm/`. The implementation is M3b's first item. The agenda below is kept as the record of what was
discussed.

### Agenda
Run `nix develop -c bin/ddm preview --serve-only`, then compare the six kinds side by side. Agenda:
- the palette, typography and AI marker ([shared styling](open-questions.md#shared-styling-across-sites), D27);
- the **shared header toolbar** and **section indexes**, including `docs.cathe.dev` → `sites.cathe.dev`
  ([site navigation](open-questions.md#site-navigation-a-shared-header-and-section-indexes));
- how far each generator can be themed, odoc especially;
- the status line (repo, branch, commit; "up to date" once the daemon exists);
- the collated OCaml docs page beyond the pool ([OCaml docs](open-questions.md#ocaml-docs-landing-pages-a-shared-index-shared-libraries)).

## M3b: the rest
- **Styling (D43), first:**
  - `theme/tokens.css`, the self-hosted Source fonts, and adapters for mkdocs, odoc and the forest;
  - `<link>` injection at publish;
  - the toolbar reads the tokens and gains the light/dark toggle (a `.cathe.dev` cookie).
- Styling adapters per generator, as designed in the session.
- The header toolbar, section index pages, and the `sites.cathe.dev` / `docs.cathe.dev` split.
- The collated OCaml docs index.
- Any remaining sites, and team project sites.
- odoc and `site://` (still untested).

## M4: forest side
- **Done 2026-10-17, ahead of schedule:**
  - `forest-public.toml` with the public `url` and redirects followed (D39);
  - people unpublished unless listed, via a fork addition (D42);
  - the notes view replaced by the `notes` tag page (D38), so the planned notes-view tag filter is dropped;
  - indimo and mebi tagged as Projects.

  The forest's public view can go live.
- **Remaining:**
- **forest**: port INDIMO's notes into the forest, if you decide to
  ([open question](open-questions.md#indimos-notes-port-to-forester-into-the-main-forest)).
- **forest**: taxa for posts, papers and works. A start on `@cathe` (the `cathe.dev` landing page) whenever you've
  decided what goes there.
- **vscode-forester** *(only if the hand-kept configs drift)*: a `site.yaml` reader and per-view TOML generator (D12).
- `forest.toml`'s `url` is still the template's; set it to `https://the.cathe.dev/` when the private view's
  "copy link" matters.

## M5: NixOS host
**Built 2026-10-18** (D44–D47): `nix/` holds the package, `services.ddm`, the host and the VM test
(`checks.droplet`), which passes offline. Your Tailscale login and the Cloudflare token are in. What's left is the GitHub
token, then the install, in place on the existing droplet ([runbook](../usage/operations.md#the-server)).

The plan was `nixosConfigurations.droplet` in this repo's flake, with:
- Caddy (with plugins), `services.tailscale`, `services.tailscaleAuth`;
- agenix, firewall;
- the `ddm` package and service, and `ddm-rebuild.service`.

Things M3a and after settled that M5 must provide:
- **Caddy:** this flake's `packages.caddy` (Cloudflare DNS plugin). Its environment needs `CLOUDFLARE_API_TOKEN` and
  `TAILNET_OWNER` (your Tailscale login; unset means nobody gets into private sites, D30). The generated config
  expects `services.tailscaleAuth`'s socket at `/run/tailscale-nginx-auth/tailscale-nginx-auth.sock`, so confirm
  the path on the pinned nixpkgs.
- **Paths:** `/var/www` (publishes plus `/_ddm/`), `/var/lib/ddm` (work dir, Caddyfile). The OCaml pool caches an
  opam switch under the work dir: about 1.5 GB, and a first build of about 6 minutes at about 600 MiB peak (D34).
  mebi_plugin may have to leave the pool on the droplet; size the droplet first.
- **GitHub token:** read access to **private** repos too. `thecathe/forest`, `demos`, `INDIMO` and `membranes` are
  not public.
- **Tools at build time:** `nix` (every workflow env comes from flakes), `git`, plus network access for opam.
- **DNS** (M7): `the.cathe.dev` and `*.the.cathe.dev` to the tailnet IP; the public names to the public IP.

## M6: ddm daemon
- Both loops, the state file, self-update, health check and rollback, and the socket CLI.
- Tested in a NixOS VM test before any real droplet exists.

## M7: cutover (live 2026-10-18)
Installed in staging first (D49): only a dummy site, and no automatic deploys. Then the forest was made public only
by tag (D51) and the bot policy built (D50), and the droplet went live on 2026-10-18.

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
- **Enforced bot blocking through Cloudflare's proxy** (asked for 2026-10-18). Proxying the public hosts (orange
  cloud) would let Cloudflare's AI-crawler blocking and bot rules refuse scrapers that ignore robots.txt (D50).
  To work out first: certificates (Cloudflare to origin, and whether Caddy's DNS-01 setup stays), the real client
  address Caddy sees, caching of the generated sites, and keeping the private domain unproxied (tailnet IPs can't be
  proxied).
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
