---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Operations

## Today (the live droplet)

- Ubuntu droplet, with Caddy configured by this repo's `Caddyfile` (hand-maintained) and certificates from
  Cloudflare DNS-01.
- A push to `main` runs `.github/workflows/deploy-dots.yml`, which SSHes in and runs `git pull` in `~/dots`.
  **Nothing is rebuilt or reloaded.** Site content is copied there by each content repo's own Action.
- `webserver deploy --config deploy.yaml` exists, but fails for every repo (all build methods are stubs).

## Available now (M1)

From the repo root:

```sh
nix develop -c go -C ddm run ./cmd/ddm check ../sites.yaml   # validate the spec; one-line summary
nix develop -c go -C ddm run ./cmd/ddm sites ../sites.yaml   # table of every mount
nix develop -c go -C ddm test ./...                          # spec tests, including sites.yaml itself
```

Run `check` before committing a change to `sites.yaml`. Nothing else reads the file yet.

## Planned (once ddm runs, roadmap M6)

| Command | Purpose |
|---|---|
| `ddm status` | Applied system sha; per site and view: last sha, build result, when. |
| `ddm sync [site]` | Check now instead of waiting for the poll. |
| `ddm build <site> [--view v]` | Run a workflow locally without publishing. It also works on your laptop. |
| `ddm rollback <site>` | Point `current` back at the previous published build. |
| `journalctl -u ddm -u ddm-rebuild` | Logs for the daemon, and for system rebuilds. |

System rollback is `nixos-rebuild switch --rollback`, which ddm also runs itself when a rebuild fails its health check.

This page will grow into a runbook as each milestone lands.
