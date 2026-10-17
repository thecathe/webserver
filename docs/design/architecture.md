---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-17
---

# Architecture

!!! warning "Partly implemented"
    Building (M2), publishing, `site://` links and the generated Caddyfile (M3a) work, and can be previewed
    locally. The daemon, its loops and the server itself (M5–M7) are still design.

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
    nix develop <env> -c <steps>  ──► scratch output ──► /var/www/<view>/<site>/<time>-<sha>/
                                                           └─ current → <time>-<sha>
   caddy.service (separate) serves /var/www/<view>/<site>/current per the generated Caddyfile
```

## ddm builds everything itself

Every site is **pulled**: ddm clones each repo (one read-only GitHub token for all of them), runs its workflow on
the server, and publishes the result. No content repo builds its own output or pushes it anywhere, so none needs a
deploy key or secret. The opposite model, where each repo's CI builds and uploads (the old per-repo Actions with
`WEBSERVER_*` secrets), was rejected: a secret per repo, a write path into the server from many places, and
toolchains scattered across CI configs (D14, D32). The cost is that the server needs every toolchain (Nix provides
them per workflow) and enough CPU for the heaviest build. If a build ever outgrows the droplet, the fallback is
still pull-shaped: ddm fetches a CI artifact with the same read-only token.

## Two loops

- **System loop.** When this repo's sha changes, ddm rebuilds the server first, because a spec change may need a new
  toolchain, a new workflow env or a new ddm. Then it reconciles every site, because a workflow definition may have
  changed.
- **Site loop.** For each site and view: if the repo's sha (or the workflow's definition) differs from the last
  successful build, it syncs the checkout, runs the workflow, publishes (resolving `site://` links), regenerates the
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
/var/www/<view>/<site>/<UTC time>-<sha12>/   one directory per publish
/var/www/<view>/<site>/current               symlink to the live one
```

Publishing copies the build, resolves its `site://` links for the target (D28), then swaps `current` by renaming a
fresh symlink over it, which is atomic. Rolling a site back is one symlink. Nothing live is ever deleted in place,
which retires the risky `rm -rf "$TARGET"` pattern in the old Actions. The newest three publishes are kept. Names
lead with the time, so they sort by age, and republishing the same commit (because a spec change altered its links)
gets a new directory rather than overwriting the live one.

Implemented in `ddm/internal/publish` and `ddm/internal/links`.

## Caddy

The Caddyfile is **generated** from `mounts` (`ddm/internal/caddygen`, `ddm caddyfile`), replacing the old
hand-written file (deleted, D19) and its per-request filesystem probing (`@has_odoc` / `@has_site`).

- **Certificates:** Cloudflare DNS-01 (`acme_dns cloudflare`). Each domain's apex gets its own block, and its direct
  subdomains share one `*.domain` block, told apart by `host` matchers, so they share one wildcard certificate. The
  private domain is grouped the same way, separately from the public domain it sits under.
- **Mounts:** each becomes a `handle` (path `/`) or `handle_path` (a prefix), rooted at that view's `current`.
  Longer paths come first. `entry:` is a `redir` on exactly `/`, which spike S2 confirmed is all the forest needs.
- **404s:** every host shows the `docs` site's `404.html`. An unknown subdomain is a 404, not an empty page.
- **Plugin:** the Cloudflare DNS module isn't in stock Caddy. This repo's flake builds `caddy.withPlugins` (exposed
  as `packages.caddy`), the devShell uses it, and so will the server, so `caddy validate` on a laptop checks the
  config the server will run.
- **Preview:** the same generator, for `http://<host>.localhost:<port>` with no TLS or private gate
  ([operations](../usage/operations.md#local-preview)).

## Private access

`the.cathe.dev` should be reachable only from your tailnet, and only by **you**. Four options were compared:

| Option | How | Who gets in | Identity known to the site? |
|---|---|---|---|
| A. Network only | DNS → the droplet's tailnet IP (100.x); Caddy `bind <tailnet IP>` | any tailnet device the ACLs let through | no; "can reach it" is the only signal |
| **B. tailscaled whois (chosen)** | DNS → the tailnet IP; Caddy refuses non-tailnet source addresses, then `forward_auth` to Tailscale's auth helper (`services.tailscaleAuth`), which asks the local tailscaled who owns the peer's IP | only your login, on your user-owned devices; tagged devices have no user and are refused | yes: login, name. Could back a `/whoami` for the widget |
| C. `caddy-tailscale` plugin | Caddy joins the tailnet as its own node; `tailscale_auth` directive | by login | yes. But the plugin is experimental, needs a custom build and has its own node keys |
| D. `tsidp` + oauth2-proxy | Tailscale as an OIDC provider; a real login flow with cookies | by login | yes. Overkill for one static site |

Not suitable: `tailscale serve` adds identity headers, but only on its own `*.ts.net` name. Funnel makes things
public.

Why B: stock Caddy (plus the DNS plugin), your own hostname and certificate, and per-user identity.

**How the gate works (as generated, D30).** Every block on the private domain runs, in order:
1. `abort` unless the client address is a Tailscale one (`100.64.0.0/10`, `fd7a:115c:a1e0::/48`);
2. drop any client-sent `Tailscale-Login` header;
3. `forward_auth` to the auth helper's socket, copying back the `Tailscale-Login` it reports;
4. 403 unless that login equals `TAILNET_OWNER`, an environment variable the NixOS module sets. If it is unset,
   nobody gets in: the gate fails closed.

The original design bound these blocks to the tailnet address (`bind 100.x`). That turned out to be impossible
alongside the public blocks: on Linux, a socket bound to one address can't listen on a port that a wildcard socket
already holds (checked with a two-socket test), so Caddy would fail to start. The source-address check gives the same
guarantee from the public side. A forged `Host:` header from the internet is aborted at step 1, and a forged source
address can't complete a TCP handshake. Steps 2–4 were tested against a stub auth helper: the owner gets 200, anyone
else 403, an unset owner 403, and a spoofed header 403. The real helper's socket path is confirmed in M5.

**What's private is declared in `sites.yaml`** (D29): `access: private` on a site or a mount, and every private mount
must sit on `private_domain` (`the.cathe.dev`). The spec refuses a private mount anywhere else and a public mount on
it, so a typo can't move a site across the line. The full **site map** at `the.cathe.dev/sites/` lists every site
and mount, each marked public or private, and is regenerated on every build.

The **public/private toggle** comes almost free, because paths match across views: the same path on another host.
An identity-aware widget is deferred (decision D10, spike S4).

## Forest views

All three views build from the same checkout, with different generated configs:

| View | Config | Effect |
|---|---|---|
| public | `forest-public.toml` (D26) | `[forest.tag.private] behaviour = "hide"`, plus redirect meta rules that expose only the `@jpears` and `@cathe` people pages |
| notes | `forest-public.toml` + the notes filter | public rules, plus *keep only trees tagged `notes`* (the home tree exempt) |
| private | `forest.toml` (today's dev config) | nothing hidden; redirect meta shown as external links |

The "keep only tagged" rule doesn't exist yet. It is a small addition to the forester fork (`thecathe/ocaml-forester`,
next to the existing tag-hide code in `lib/compiler/State.ml`'s `plant_article` and
`lib/frontend/Forest_util.ml`). It warns loudly instead of changing behaviour silently, which matches the fork's own
guiding constraint.

For now the two configs are kept by hand (D26). Both live at the forest repo root, because forester failed with a
config in a subdirectory. If they start to drift, the scripts in `thecathe/vscode-forester` (which already provide
`forest_gen.py` / `build-forest`) could generate both from a forest `site.yaml` (D12).

## Secrets

All held once, in this repo, encrypted with agenix (or sops-nix; still open):
- a fine-grained GitHub token (Contents: read-only, all your repos);
- the Cloudflare DNS token;
- the Tailscale auth key.

No content repo holds any secret.
