---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Architecture

!!! warning "Design, not implemented"

```text
             this repo (spec + workflows + ddm + NixOS config)
                         │  poll (git ls-remote)
                         ▼
   ┌──────────────── ddm.service ─────────────────┐
   │ system loop: new sha → start ddm-rebuild ────┼──► nixos-rebuild switch --flake …/<sha>#droplet
   │ site loop:   new site sha → workflow → publish → caddy reload
   └──────┬───────────────────────────────────────┘
          │ systemd-run (throwaway user)
          ▼
    nix develop <env> -c <steps>  ──► scratch output ──► /var/www/<view>/<site>/<sha>/
                                                           └─ current → <sha>
   caddy.service (separate) serves /var/www/<view>/<site>/current per the generated Caddyfile
```

## Two loops

- **System loop.** When this repo's sha changes, ddm rebuilds the server first, because a spec change may need a new
  toolchain, a new workflow env or a new ddm. Then it reconciles every site, because a workflow definition may have
  changed.
- **Site loop.** For each site and view: if the repo's sha (or the workflow's definition) differs from the last
  successful build, it syncs the checkout, runs the workflow, runs `rewrite-links`, publishes, regenerates the
  Caddyfile and reloads Caddy. One site failing never blocks the others, as in the old `deploy.go`.

ddm keeps a small state file (applied system sha; last built sha and result per site and view), so a restart picks up
where it left off.

## Self-update

The danger is that ddm is updated by a rebuild that ddm started. So ddm never runs `nixos-rebuild` itself. It starts
a separate oneshot unit, `ddm-rebuild.service`, modelled on NixOS's `system.autoUpgrade` (which might simply be
reused). Then:

1. `ddm-rebuild` builds and switches to the new generation.
2. If ddm's package changed, `switch-to-configuration` restarts `ddm.service` at the end. The rebuild isn't killed,
   because it lives in a different unit.
3. The new ddm reads its state file, sees the system sha applied, and reconciles sites.
4. A failed health check (Caddy config invalid, ddm not up) triggers `nixos-rebuild switch --rollback`.

## Publish layout

```text
/var/www/<view>/<site>/<sha>/    one directory per build
/var/www/<view>/<site>/current   symlink to the live build
```

Publishing is an atomic symlink swap, and rolling a site back is one symlink. Nothing is ever deleted in place, which
retires the risky `rm -rf "$TARGET"` pattern in the old Actions. Old builds are pruned, keeping the last few. Public
host blocks only ever point into `/var/www/public` or `/var/www/notes`.

## Caddy

The Caddyfile is **generated** from `mounts`, replacing the old hand-written file (deleted, D19) and its per-request filesystem
probing (`@has_odoc` / `@has_site`). Each mount becomes a host block (or a path inside one) rooted at the right
`current`. `entry:` becomes `redir / <entry>`; a redirect, which spike S2 confirmed is all the forest needs (its links
are root-absolute). Certificates keep coming from Cloudflare DNS-01, with a wildcard for `*.cathe.dev`.

## Private access

`the.cathe.dev` should be reachable only from your tailnet, and only by **you**. Four options were compared:

| Option | How | Who gets in | Identity known to the site? |
|---|---|---|---|
| A. Network only | DNS → the droplet's tailnet IP (100.x); Caddy `bind <tailnet IP>` | any tailnet device the ACLs let through | no; "can reach it" is the only signal |
| **B. tailscaled whois (chosen)** | as A, plus Caddy `forward_auth` to Tailscale's auth helper (`services.tailscaleAuth`), which asks the local tailscaled who owns the peer's IP | only your login, on your user-owned devices; tagged devices have no user and are refused | yes: login, name. Could back a `/whoami` for the widget |
| C. `caddy-tailscale` plugin | Caddy joins the tailnet as its own node; `tailscale_auth` directive | by login | yes. But the plugin is experimental, needs a custom build and has its own node keys |
| D. `tsidp` + oauth2-proxy | Tailscale as an OIDC provider; a real login flow with cookies | by login | yes. Overkill for one static site |

Not suitable: `tailscale serve` adds identity headers, but only on its own `*.ts.net` name. Funnel makes things
public.

Why B: stock Caddy, your own hostname and cert, and per-user identity. The `bind` means the public interface never
answers for `the.cathe.dev`, even with a forged `Host:` header. To confirm in M5: `services.tailscaleAuth` on the
pinned nixpkgs, and that Caddy's `forward_auth` can reach its socket.

The **public/private toggle** comes almost free, because paths match across views: the same path on another host.
An identity-aware widget is deferred (decision D10, spike S4).

## Forest views

All three views build from the same checkout, with different generated configs:

| View | Config | Effect |
|---|---|---|
| public | `forest.public.toml` | `[forest.tag.private] behaviour = "hide"`, plus the redirect meta rules |
| notes | `forest.notes.toml` | public rules, plus *keep only trees tagged `notes`* (the home tree exempt) |
| private | `forest.private.toml` | nothing hidden; redirect meta shown as external links |

The "keep only tagged" rule doesn't exist yet. It is a small addition to the forester fork (`thecathe/ocaml-forester`,
next to the existing tag-hide code in `lib/compiler/State.ml`'s `plant_article` and
`lib/frontend/Forest_util.ml`). It warns loudly instead of changing behaviour silently, which matches the fork's own
guiding constraint.

The three TOMLs are generated from the forest's `site.yaml` by the scripts in `thecathe/vscode-forester` (which
already provide `forest_gen.py` / `build-forest`). They are written at the forest repo root, because forester failed
with a config in a subdirectory.

## Secrets

All held once, in this repo, encrypted with agenix (or sops-nix; still open):
- a fine-grained GitHub token (Contents: read-only, all your repos);
- the Cloudflare DNS token;
- the Tailscale auth key.

No content repo holds any secret.
