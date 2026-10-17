---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-17
---

# Operations

## Today (the old droplet)

The old Ubuntu droplet still runs the previous hand-made setup: a Caddy config, plus content copied there by each
content repo's own GitHub Action. That setup has been removed from this repo (it is in git history before
2026-10-16), and the droplet will be **wiped and reinstalled** with NixOS in M7. Nothing on it needs preserving:
everything is either in a git repo or generated from one.

The `deploy-dots` Action is gone, so pushing to `main` no longer touches the droplet. Its GitHub secrets
(`WEBSERVER_*`) are still set in the repo settings. They are harmless, but can be deleted.

## Available now (M1–M3a)

From the repo root:

```sh
nix develop -c bin/ddm check                    # validate the spec; one-line summary
nix develop -c bin/ddm sites                    # table of every mount
nix develop -c bin/ddm build <site> [--view v]  # build into .ddm/out/<site>/<view>/<commit>/
nix develop -c bin/ddm publish <site> [--view v] [--target prod|preview]
nix develop -c bin/ddm caddyfile [--target preview]   # print the generated Caddyfile
nix develop -c bin/ddm preview [site ...]       # build, publish and serve everything locally
nix develop -c go -C ddm test ./...             # all tests, including sites.yaml itself
```

Run `check` before committing a change to `sites.yaml`, and `build` before relying on a new site or workflow.
`bin/ddm` compiles ddm from source and runs it, so it always matches the checkout. Every command takes `-h`;
[workflows](workflows.md#trying-one) has build examples.

### Local preview

`ddm preview` builds every mounted view of every site (or just the sites named), publishes them into `.ddm/www`
exactly as the server will lay them out, and serves them with Caddy. Every host gets a local name:

```text
https://forest.cathe.dev/   →   http://forest.cathe.dev.localhost:8080/
https://the.cathe.dev/sites/ →  http://the.cathe.dev.localhost:8080/sites/
```

Browsers send `*.localhost` to your own machine, so no DNS or `/etc/hosts` changes are needed. Caddy listens on
localhost only. `site://` links are resolved to these preview addresses.

- A site that fails to build is reported and skipped; the rest are still served.
- `--serve-only` serves what was last published, without building (builds take minutes; serving is instant).
- `--no-serve` builds and publishes only.
- `--src ddm=.@my-branch` builds a site from a local repo and branch (committed state only). This site, for
  example, is only on GitHub once `main` is pushed.
- **Private mounts are served without the tailnet check**: the preview is for looking, not for access control.
  The forest's public view also includes the profiles that keep it unpublished (D25), so a preview is for you only.

### The toolbar

Every published page gets a bar along its top, drawn by `/_ddm/bar.js`:
- the `nav:` links from `sites.yaml`;
- **view switches**: the same page in the site's other views. On `the.cathe.dev/indimo/` it offers `public` →
  `forest.cathe.dev/indimo/` and `notes` → `notes.cathe.dev/indimo/`, which is the quick way to check what a tree
  looks like publicly;
- a **status line**: site, repo and branch (linked), commit (linked), pool if any, and build date.

How it gets there: publishing inserts `<script type="module" src="/_ddm/bar.js">` before `</head>` in every
`.html` page and `.xsl` stylesheet (the forest's pages are built by its XSLT in the browser). ddm writes the
script's data to `<www>/_ddm/public/` and `<www>/_ddm/private/`, and Caddy serves `/_ddm/` on every host from the
matching one. A public page therefore never sees a private address. The bar is drawn in a shadow root, before
`<body>`, so no site's CSS or layout can affect it. Its look is a placeholder for the styling session.

"Up to date or not" isn't shown yet: that needs the daemon's polling (M6).

### Publishing

`ddm publish` builds one view and publishes it:

```text
<www>/<view>/<site>/<UTC time>-<commit>/   one directory per publish
<www>/<view>/<site>/current                symlink to the live one
```

`current` is swapped with an atomic rename, so a reader never sees a half-copied site. The newest three publishes
are kept. Rolling back by hand means pointing `current` at an older directory (`ln -sfn <dir> current.tmp && mv -T
current.tmp current`); `ddm rollback` will do this (M6).

`--target prod` writes to the spec's `paths.www` (`/var/www`), with `https://` links, and fails on any unresolved
`site://` link. On your machine `/var/www` won't be writable; that target is for the server.

### The Caddyfile

`ddm caddyfile` prints the production config generated from the mounts. Check it with Caddy itself:

```sh
nix develop -c sh -c 'bin/ddm caddyfile | caddy adapt --adapter caddyfile --config /dev/stdin >/dev/null'
```

The devShell's Caddy includes the Cloudflare DNS plugin that the config needs, the same build the server will
run. [Architecture](../design/architecture.md#caddy) explains what each part does.

## Planned (once ddm runs, roadmap M6)

| Command | Purpose |
|---|---|
| `ddm status` | Applied system sha; per site and view: last sha, build result, when. |
| `ddm sync [site]` | Check now instead of waiting for the poll. |
| `ddm rollback <site>` | Point `current` back at the previous published build. |
| `journalctl -u ddm -u ddm-rebuild` | Logs for the daemon, and for system rebuilds. |

System rollback is `nixos-rebuild switch --rollback`, which ddm also runs itself when a rebuild fails its health check.

This page will grow into a runbook as each milestone lands.
