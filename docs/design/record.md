---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Decision record

The living list of decisions. Each says **what** was decided, **why**, and **when**. A decision that is replaced is
marked *superseded* and kept, so the reasoning trail survives. For the story of how these were reached, see the
[development log](../development.md). For what is still undecided, see [open questions](open-questions.md).

## Current decisions

### D1. One server repo, separate from the NixOS-host dots (2026-10-16)
This repo is the server's repo, separate from `thecathe/dots` (your machines). It holds the spec, the workflows, ddm,
and the server's NixOS config. It moves from `cathe-org` back to your personal account (name TBD; `thecathe/dots` is
taken, and `server-dots` is a placeholder), along with the other `cathe-org` repos. **Why:** it is the one project
that needs GitHub secrets, and doing that once means no other project ever needs any.

### D2. ddm rebuilds the server, including itself (2026-10-16)
ddm pulls this repo and switches the droplet to `nixosConfigurations.<host>` at the exact sha it saw. **Why:** the
server's toolchains, Caddy and ddm itself are then all versioned with the spec that uses them. How it survives
updating itself is in [architecture](architecture.md#self-update).

### D3. The droplet goes straight to NixOS (2026-10-16)
*The blue/green part is superseded by D21* (reinstall in place).
*Supersedes the 2026-09 phased plan* (home-manager on Ubuntu first, NixOS later). **Why:** a manager that rebuilds
the system config needs a system config to own, and home-manager on Ubuntu can't own system services. Blue/green:
build a new droplet, validate it, cut DNS over, retire the old one. Never convert the live box in place.

### D4. Caddy runs separately from ddm (2026-10-16)
nixpkgs `services.caddy` built with `caddy.withPlugins` (caddy-dns/cloudflare). ddm writes the Caddyfile and reloads.
*Supersedes* embedding Caddy in the Go binary. **Why:** restarting or updating ddm must never take the sites down.

### D5. Workflows are YAML steps plus Go built-ins (2026-10-16)
*Supersedes* the fixed `builders` map (`md`/`mkdocs`/`odoc`/`ocaml`). **Why:** workflows become reusable and
composable, and a new kind of site doesn't need a ddm release. Builds still run outside Nix derivations (decision
kept from 2026-09: derivations have no network and don't suit branch-following builds).

### D6. Two domains: `cathe.dev` and `jpears.me` (2026-10-16)
`cathe.work` is being dropped. Its professional/CV role moves to `jpears.me`. Validation rejects hosts outside the
declared domains. Before it lapses, update anything that links to `cathe.work`.

### D7. Hosts (2026-10-16)
- `jpears.me` → forest `/@jpears/` (profile; CV pages, with the CV transcluding parts of them).
- `forest.cathe.dev` → forest `/index/`. `cathe.dev` itself is undecided.
- `notes.cathe.dev` → the notes view.
- `the.cathe.dev` → the private view.
- `docs.cathe.dev` → the generated directory.
- `<project>.cathe.dev` → docs sites.
- `ddm.cathe.dev` → this site.

### D8. The forest has three views (2026-10-16)
- **public**: hides `private`.
- **notes**: only trees tagged `notes`, and never `private`.
- **private**: everything.

Each is a separate build into a separate root, so a public URL can never serve a private file. Relative paths are
identical across views.

### D9. Private view: Tailscale, identity via tailscaled (2026-10-16)
This is option B of the four compared in [architecture](architecture.md#private-access). The host binds only to the
tailnet IP, and Caddy's `forward_auth` asks tailscaled who the visitor is, allowing only your login. The rejected
alternatives are kept there.

### D10. Identity widget deferred (2026-10-16)
A widget that only does something for your identity (e.g. a public⇄private toggle). Preferred in both views, but
only after spike S4, and dropped if it adds weaknesses (a hostname visible in public JS; browser private-network
blocking).

### D11. Content lives in the forest, classified by taxa (2026-10-16)
Blog posts, papers and works are trees with taxa (`Post`, `Paper`, …), and the generated taxon pages serve as their
indexes. Larger external things (project docs) are separate sites, linked by `site://`.

### D12. A forest `site.yaml` for site and content config (2026-10-16)
The settings only the scripts read, plus the per-view differences, move out of `forest.toml` into a `site.yaml` in
the forest repo. That's consistent with this repo's spec being YAML. A generator writes each view's
`forest.<view>.toml`. **Why:** one source of truth instead of three hand-kept TOML files that drift.

### D13. Links between sites by name: `site://name/path` (2026-10-16)
*Supersedes* `DomainLink{LinkTo, LinkPlaceholder, Path}` (2026-09), which itself replaced an abandoned `sed` step in
`cloakaml-deploy-docs.yml`. **Why:** a fixed scheme needs no per-link config, and it resolves per view.

### D14. Polling first, one read-only token (2026-10-16)
ddm discovers new commits by polling with `git ls-remote`, and clones with a single fine-grained, read-only GitHub
token. **Why:** zero setup per repo. Webhooks are an optional later addition.

### D15. Claude implements; docs are the interface (2026-10-16)
*Supersedes* the 2026-09 curriculum mode (you implement, Claude teaches). **Why:** time. The docs, this site, are how
you learn to use the system fully without building it.

### D16. Integration: local `--no-ff` into `main` (2026-10-16)

### D17. `ddm/` is built alongside the legacy `webserver/` (2026-10-16, M1)
*Superseded by D19 the same day*, once it was clear the droplet can be wiped.
*Refines* the plan to rename `webserver` → `ddm`. The live droplet may be running the `webserver` binary as its
Caddy (it embeds Caddy plus the Cloudflare DNS plugin), so `webserver/`, `deploy.yaml` and the `Caddyfile` stay
untouched until cutover (M7) and are removed then. **Why:** blue/green applies to the code as well as the machine:
nothing in this repo should change what the live box does until the new one is proven. A side effect is that `ddm/`
depends only on `gopkg.in/yaml.v3`, not on Caddy's large dependency tree.

### D18. `cathe.work` is redirected until it lapses, not deleted early (2026-10-16, M1)
*Superseded by D20 the same day.*
*Refines* D6. Removing the `cathe.work` blocks from the live `Caddyfile` in M1 would break existing links while the
domain is still held. Instead, the generated Caddyfile (M3) redirects `cathe.work` and `*.cathe.work` to their new
homes. The spec rejects new `cathe.work` mounts, because redirects are not mounts. Once the domain lapses, the
redirect block is deleted.

### D19. The legacy setup is deleted (2026-10-16, M1)
`webserver/`, `deploy.yaml`, the hand-written `Caddyfile` and the `deploy-dots` Action are removed; `ddm/` is the
only code. **Why:** nothing on the droplet needs preserving (it is all in git or generated from git), so there is
no live setup to protect. The old Caddyfile's useful ideas (an odoc root rewrite, a shared 404 page) are carried
into roadmap M3.

### D20. `cathe.work` is simply dropped (2026-10-16)
No redirects. It disappears with the old droplet. The spec still rejects `cathe.work` hosts, and updating external
links to it remains a personal checklist item.

### D21. The droplet is reinstalled in place (2026-10-16)
*Supersedes the blue/green part of D3.* When M5 is ready, the existing droplet is wiped and NixOS installed on it
(e.g. nixos-anywhere, or DigitalOcean rebuild + nixos-infect). **Why:** nothing on it needs preserving, it keeps the
same IP so DNS doesn't change, and there is no second droplet to pay for. The cost is downtime between wipe and
first successful deploy, which is acceptable. Everything is first proven in a local NixOS VM test (M5/M6), to keep
that window short.

### D22. No forest-specific built-ins (2026-10-16, M2)
The planned `lint-hidden` built-in is dropped. The forest's own `build-forest --strict-private` (from
`thecathe/vscode-forester`) already fails a build in which a visible tree links to a hidden one, so the `forester`
workflow runs it, except for the private view. **Why:** ddm stays ignorant of forester. The logic lives with the
tool that understands trees and is tested there.

### D23. Workflow environments: the repo's devShell, or a small one from this repo (2026-10-16, M2)
`env.flake` is `{{repo}}` when the site repo ships a usable devShell (the forest), and otherwise one of this repo's
per-tool devShells: `#mkdocs`, `#ocaml-docs`. **Why:** repos stay free of ddm-specific config, the toolchain is
pinned by this repo's `flake.lock`, and the shells are small enough not to drag in Go. Found in practice: cloakaml's
pushed flake only has opam, so `{{repo}}` would not have worked there.

### D24. Params resolve up front, and the caller wins (2026-10-16, M2)
Precedence, lowest first: workflow defaults, built-in vars, view params, `--set`. An inlined `use:` workflow's
defaults only fill gaps, and its env replaces the caller's only if it has one. An unknown `{{param}}` is an error
before anything runs. **Why:** loud, early failures; `--dry-run` shows exactly what will run.

### D25. The forest's public and notes views don't go live before M4 (2026-10-16, M2)
The first real build showed that the public view, built from today's `forest.toml` (your dev config, where
`redirect` means "show as external"), includes full pages for people's profiles (`trees/people/@*.tree`). Avoiding
that was the whole reason for redirect-meta. `private`-tagged trees *are* excluded. So publishing these two views
waits for M4's per-view configs, while the private view (tailnet-only) and the other sites can go live in M3.

## AI-generated content marking

*Trial, 2026-10-16.* Every page Claude generates carries front matter:

```yaml
generated: { by: Claude, model: Opus 5.5, effort: unrecorded, date: 2026-10-16, edited: 2026-10-16 }
```

The site renders it as a pale yellow page, with a small label at the top and the full sentence ("This page was
generated by Claude … last edited …") drawn very large and faded on a layer behind the text. You can read the page
normally, or look past it to the note. The implementation is `overrides/main.html` + `stylesheets/ai-generated.css`.

It is meant as a **convention across all sites**. Other generators need their own version, which is an
[open question](open-questions.md#ai-marking-beyond-this-site). The same rule is in your global Claude
instructions.

## Superseded (2026-09 curriculum sessions)

| Decision | Replaced by |
|---|---|
| Phased Nix: home-manager on Ubuntu, then NixOS | D3 |
| Builds in `builders.go`, a fixed method map | D5 (the "not Nix derivations" part stays) |
| Rebuild via the GH Action calling `webserver deploy`, plus a systemd timer | D2 + D14 (ddm polls; the Action is at most a "poke now") |
| Domain roles: `cathe.dev` dev, `cathe.work` professional, `jpears.me` personal | D6 / D7 |
| Caddyfile hand-maintained, filesystem probing per subdomain | Generated from mounts (roadmap M3); old file deleted (D19) |
| Blue/green droplet swap | D21 (reinstall in place) |
| `DomainLink` placeholder links | D13 |
| `ocaml` method chains into mkdocs; `generate_docs.py` stays Python | Kept, as the `ocaml-notebook` workflow |
| Curriculum mode | D15 |
