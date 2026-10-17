---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-17
---

# Open questions

The agenda for future design sessions. Each question has a **current lean** (what we'd do without further thought),
**what decides it**, and **what's at stake**, so a fresh session can pick any of them up cold. Leans are not
decisions: the landscape is expected to shift once real builds run, so revisit freely. When a question is settled,
move it to the [decision record](record.md) and note it in the [development log](../development.md).

## Spec naming

- **Question:** are `views`, `mounts`, `entry`, `access`, `taxa` and `site://` the right words and shapes?
- **Status (M1):** the first real `sites.yaml` was written with these names, and nothing felt wrong. They are now
  in code (`ddm/internal/spec`), but nothing depends on them outside this repo yet.
- **Lean:** keep them. Revisit if building M2/M3 makes any of them awkward.
- **Smaller points:**
  - Should `system.flake` be derived from the repo this spec lives in, rather than stated?
  - Should `params` be allowed on `use:` steps, to tweak an inlined workflow?
  - Should `repo` accept other forges, or stay GitHub-only?
- **Stake:** rising: renames get more expensive once the daemon and the Caddy generator read these names.

## `site://` survives every generator (spike S1)

- **Answered (M2):** forester and mkdocs (strict mode) both emit `site://cloakaml/…` verbatim, so the scheme stays.
- **Still open:** odoc docstring links (`{{:site://…}text}`) are untested. Test when a project first needs one.
- **Answered (M3a):** forest redirect trees work. `\meta{redirect}{[text](site://name/)}` resolves in both the
  `redirect` and `external` configs. The bare form only works with `redirect`; see
  [links between sites](../usage/spec.md#links-between-sites).

## One forest build on two hosts (spike S2)

- **Answered (M2):** links are root-absolute, so one build serves both hosts unchanged, and `/` → entry can be a
  plain redirect.
- **Follow-ups:**
  - The forest's `url` is still the template's `https://www.my-great-forest.net/`, used in `uri` attributes
    and some broken-link warnings. Set it per view in M4; the lean is `https://forest.cathe.dev/` for public.
  - Root-absolute sites can't be mounted under a path prefix. **Answered (M3a, D33):** workflows declare
    `root_only: true`, and validation rejects a `path:` on their mounts.

## The forest notes view

- **Question:** a fork addition ("keep only trees with any of these tags") or a script-level filter?
- **Lean:** the fork addition, small and tested next to `Test_redirect_meta.ml`. A script filter can't remove trees
  from forester's own indexes.
- **Also open:**
  - What happens to a notes tree that links to a non-notes tree? It should probably be a `--strict-private`-style
    failure, or a link to the public copy via `site://`.
  - Is the home tree the right exemption?

## Forest configs: `forest-public.toml` now, `site.yaml` later?

- **Decided (D26):** start with a hand-made `forest-public.toml` next to `forest.toml`.
- **Open: expressing "only these people".** Today the meta rules are per key and per taxon
  (`[forest.meta.redirect.Person]`), so the public config can redirect *every* `Person`. How do `@jpears` and
  `@cathe` stay visible? Options:
  - drop `\meta{redirect}` from those two trees, so the rule has nothing to act on;
  - a tag (e.g. `public-person`) with its own behaviour;
  - a small fork addition for per-address exceptions.

  The lean is the first, since it needs no code. Check what the redirect target on those trees is for before
  removing it.
- **Later question:** whether to generate the TOMLs from a forest `site.yaml` (D12). Only worth it if the two hand-kept
  files drift. If so, the yaml holds views and the content model, and every TOML (dev included) is generated.
- **Stake:** editor tooling (forest-keeper, vscode-forester) must keep finding a config. Check what it reads before
  moving anything.

## Workflow environments (spike S3)

- **Answered (M2):** the forest builds from a clean checkout in its own devShell. Repos without a usable devShell
  build in one of this repo's (`#mkdocs`, `#ocaml-docs`); cloakaml's pushed flake only has opam.
- **Open:** a site can't yet choose its env; it comes with the workflow. Should sites be able to override `env`, e.g.
  an odoc project with unusual libraries? The lean is yes, as a site-level `env:`, once a second OCaml project
  needs it. Until then, add libraries to `#ocaml-docs`.
- **Also open:** the first build of a new devShell is slow. Should ddm pre-build envs during the system loop?
- **Also open:** the mkdocs-material team warns that MkDocs 2.0 will break plugins and theme overrides, with no
  migration path. nixpkgs pins MkDocs 1.6, so nothing breaks yet, but the `mkdocs` workflow and this site's
  `overrides/` are exposed. Options: stay pinned, or move to a successor (e.g. Zensical) or the planned `md-ssg`.

## ddm runtime details

- **Poll interval:** 10m? Per-site overrides?
- **Notifications:** these replace the dead `email_notifications` field. Email, ntfy, or a status page? The lean is
  a status page first, most naturally the private site map (`the.cathe.dev/sites/`, D29), extended with each
  mount's live commit and last build once the daemon keeps state (M6).
- **Health check:** for rollback, what does "the new generation is healthy" mean? `caddy validate`, ddm answering on
  its socket, and a fetch of each mount's entry?
- **Retention:** how many builds to keep per site?
- **Rebuild hygiene:** cleaning stale `/var/www` roots left by sites removed from the spec.

## Infrastructure

- **Droplet:** reinstalled in place (D21). Open: nixos-anywhere (kexec into an installer, partition with disko,
  fully declarative) or DigitalOcean rebuild + nixos-infect (simpler, less declarative disk layout). The lean is
  nixos-anywhere. Also: is the current droplet size enough for forester and odoc builds?
- **Secrets:** agenix or sops-nix. The lean is agenix, which is simpler; the droplet's host key decrypts.
- **Repo name** after the transfer (`server-dots` is a placeholder).
- **DNS:** a wildcard `*.cathe.dev`, `cathe.dev` and `jpears.me` to the public IP, plus `the.cathe.dev` and
  `*.the.cathe.dev` to the tailnet IP (D29). Wildcard certificates for both come from the same Cloudflare token.
- **Remote access:** should `the.cathe.dev`'s tailnet IP be in public DNS at all, or only in Tailscale's split DNS?

## Private sites in a local preview

- **Question:** `ddm preview` serves private mounts without any check, on localhost only. Is that enough, or should
  a preview skip private views unless asked (`--private`)?
- **Lean:** fine as is: the preview is yours, on your machine. Revisit if previews are ever shared.

## Private-access widget (spike S4)

- **Question:** can a public page `fetch` a tailnet-only host in current browsers, given private-network-access
  rules? Is revealing the hostname in public JS acceptable?
- **Lean:** deferred (decision D10). If S4 is clean, `the.cathe.dev/whoami` behind `forward_auth`, with the widget
  inert unless the answer is your login.

## Team project sites

- **Goal:** project-specific sites that a team can use, hosting material or redirecting to it.
- **Questions:**
  - **Access:** public, shared via Tailscale (node sharing to teammates, with ACLs by login), or something else for
    people outside your tailnet?
  - **Redirect-only sites:** a mount kind like `redirect: https://…` with no repo and no build?
  - **Contribution:** can teammates trigger rebuilds, or only push to the content repo and wait for the poll?
  - **Branding:** per-project theming via workflow `params`?
- **Lean:** public sites and redirect mounts first, then Tailscale sharing for restricted ones.

## Shared styling across sites

- **Goal (D27):** a consistent look across the forest, mkdocs sites, odoc output and anything later, plus
  cross-linking between subdomains.
- **Questions:**
  - **Source of truth:** one CSS file of design tokens (palette, fonts, spacing) plus the AI-marker styles? Where is
    it served from: an `assets` site on its own host, or a path on an existing one?
  - **Delivery:** link to it at runtime (one fetch, always current, but cross-origin) or copy it into each build
    (self-contained and versioned with the build)? The lean is copying, via a `theme` built-in run after every
    build, which also suits `rewrite-links`.
  - **Per generator:**
    - mkdocs: `extra_css` plus Material palette overrides;
    - forester: the base-theme fork's CSS and XSLT;
    - odoc: replace `odoc.support/odoc.css`.

    How far can each go before fighting its generator's own design?
  - **Shared chrome:** a common header or footer linking `cathe.dev`, `jpears.me`, `docs.cathe.dev`?
  - **Light and dark:** all generators should honour `prefers-color-scheme`, with the same token names.
- **Stake:** the more sites go live with their own default looks, the more retrofitting there is. Worth designing
  before or alongside M3.

## AI marking beyond this site

- **Question:** how the yellow "generated by Claude" treatment looks in forest pages (`\meta{generated}{…}` plus the
  theme XSLT), odoc output and other generators.
- **Lean:** one shared CSS file, served from this site or a common assets mount, with each generator adding the
  class and data attribute.
- **Also open:** is the effort level recordable reliably? It shows as `unrecorded` so far.

## `cathe.work` sunset

Dropped outright (D20): no redirects, and it disappears with the old droplet. Remaining, a personal checklist:
update anything that links to it (CV PDFs, profiles, email signatures, GitHub bios, papers) before it lapses.

## This site's host

- **Question:** is `ddm.cathe.dev` the right name? Or `dev.cathe.dev`, or a path under `docs.cathe.dev`?
