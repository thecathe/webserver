---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Open questions

The agenda for future design sessions. Each question has a **current lean** (what we'd do without further thought),
**what decides it**, and **what's at stake**, so a fresh session can pick any of them up cold. Leans are not
decisions: the landscape is expected to shift once real builds run, so revisit freely. When a question is settled,
move it to the [decision record](record.md) and note it in the [development log](../development.md).

## Spec naming

- **Question:** are `views`, `mounts`, `entry`, `access`, `taxa` and `site://` the right words and shapes?
- **Lean:** keep them until M1 forces a choice. Rename then, before anything depends on them.
- **Decides it:** writing the first real `sites.yaml` with the forest, cloakaml and this site in it.
- **Stake:** low now, high after M1, when names become config people have to edit.

## `site://` survives every generator (spike S1)

- **Question:** does forester (`resolve_uri`, `lib/compiler/Eval.ml`) pass `site://name/path` through untouched? Do
  mkdocs and odoc?
- **Lean:** yes for mkdocs (it only checks relative links) and probably odoc. Unknown for forester.
- **Fallback:** `https://site.invalid/name/path`, which every tool treats as an external URL.
- **Stake:** the cross-site linking design.

## One forest build on two hosts (spike S2)

- **Question:** are links in forest output root-absolute or relative? Can the same build serve `jpears.me` and
  `forest.cathe.dev`?
- **Lean:** serve the same build on both, with `/` redirecting to the entry.
- **Wrinkle:** `url` in the forest config is a single canonical value.
- **Stake:** whether `jpears.me` needs its own build, and whether to use a redirect or a rewrite.

## The forest notes view

- **Question:** a fork addition ("keep only trees with any of these tags") or a script-level filter?
- **Lean:** the fork addition, small and tested next to `Test_redirect_meta.ml`. A script filter can't remove trees
  from forester's own indexes.
- **Also open:**
  - What happens to a notes tree that links to a non-notes tree? It should probably become a `lint-hidden`-style
    warning, or a link to the public copy via `site://`.
  - Is the home tree the right exemption?

## Forest `site.yaml` scope

- **Question:** what moves out of `forest.toml`, and does the generator also write the dev config that the editor
  and `build-forest` use?
- **Lean:** the yaml holds views and the content model, and every TOML (dev included) is generated and gitignored.
- **Stake:** editor tooling (forest-keeper, vscode-forester) must keep finding a config. Check what it reads before
  moving anything.

## Workflow environments (spike S3)

- **Question:** does `nix develop github:thecathe/forest/<sha> -c forester build` work on a clean machine? What about
  repos without a flake?
- **Lean:** the repo's own devShell where one exists, otherwise a devShell in this repo.
- **Also open:** the first build of a new devShell is slow. Should ddm pre-build envs during the system loop?
- **Also open:** the mkdocs-material team warns that MkDocs 2.0 will break plugins and theme overrides, with no
  migration path. nixpkgs pins MkDocs 1.6, so nothing breaks yet, but the `mkdocs` workflow and this site's
  `overrides/` are exposed. Options: stay pinned, or move to a successor (e.g. Zensical) or the planned `md-ssg`.

## ddm runtime details

- **Poll interval:** 10m? Per-site overrides?
- **Notifications:** these replace the dead `email_notifications` field. Email, ntfy, or a status page on
  `ddm.cathe.dev`? The lean is a status page first.
- **Health check:** for rollback, what does "the new generation is healthy" mean? `caddy validate`, ddm answering on
  its socket, and a fetch of each mount's entry?
- **Retention:** how many builds to keep per site?
- **Rebuild hygiene:** cleaning stale `/var/www` roots left by sites removed from the spec.

## Infrastructure

- **Droplet:** provider and size. Stay on DigitalOcean? The NixOS image: a community image, or `nixos-infect` on a
  fresh box?
- **Secrets:** agenix or sops-nix. The lean is agenix, which is simpler; the droplet's host key decrypts.
- **Repo name** after the transfer (`server-dots` is a placeholder).
- **DNS:** a wildcard `*.cathe.dev` plus explicit `the.cathe.dev` (pointing at the tailnet IP) and `jpears.me`.
- **Remote access:** should `the.cathe.dev`'s tailnet IP be in public DNS at all, or only in Tailscale's split DNS?

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

## AI marking beyond this site

- **Question:** how the yellow "generated by Claude" treatment looks in forest pages (`\meta{generated}{…}` plus the
  theme XSLT), odoc output and other generators.
- **Lean:** one shared CSS file, served from this site or a common assets mount, with each generator adding the
  class and data attribute.
- **Also open:** is the effort level recordable reliably? It shows as `unrecorded` so far.

## `cathe.work` sunset

- **Checklist:**
  - list everywhere that links to it (CV PDFs, profiles, email signatures, GitHub bios, papers);
  - redirect what can be redirected while the domain is still held;
  - remove it from the Caddyfile and the spec (M1).
- **Unknown:** when exactly it lapses.

## This site's host

- **Question:** is `ddm.cathe.dev` the right name? Or `dev.cathe.dev`, or a path under `docs.cathe.dev`?
