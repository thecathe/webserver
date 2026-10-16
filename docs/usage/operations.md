---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Operations

## Today (the old droplet)

The old Ubuntu droplet still runs the previous hand-made setup: a Caddy config, plus content copied there by each
content repo's own GitHub Action. That setup has been removed from this repo (it is in git history before
2026-10-16), and the droplet will be **wiped and reinstalled** with NixOS in M7. Nothing on it needs preserving:
everything is either in a git repo or generated from one.

The `deploy-dots` Action is gone, so pushing to `main` no longer touches the droplet. Its GitHub secrets
(`WEBSERVER_*`) are still set in the repo settings. They are harmless, but can be deleted.

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
