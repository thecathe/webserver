---
generated:
  by: Claude
  model: Sonnet 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-22
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
- **the private/public switch.** On every private page there is a `public` button: the same page in the public
  version. It is greyed out when that page has no public version, either because the site has no public mount or
  because the page isn't in the public build (a hidden tree, a person's page). ddm checks this against a list of the
  public build's pages, written at publish into the private-only data. **In a local preview only**, public pages
  also get a `private (preview only)` button, for testing. Production never writes the private addresses for public
  pages, so the button can't exist there;
- a **status line**: site, repo and branch (linked), commit (linked), pool if any, and build date. On public pages
  the repo, branch and commit appear only for repos GitHub serves publicly. A private repo (or a failed check) shows
  just the site and the date.

How it gets there: publishing inserts `<script type="module" src="/_ddm/bar.js">` before `</head>` in every
`.html` page and `.xsl` stylesheet (the forest's pages are built by its XSLT in the browser). ddm writes the
script's data to `<www>/_ddm/public/` and `<www>/_ddm/private/`, and Caddy serves `/_ddm/` on every host from the
matching one. A public page therefore never sees a private address. The bar is drawn in a shadow root, before
`<body>`, so no site's CSS or layout can affect it. Its look is a placeholder for the styling session.

"Up to date or not" isn't shown yet: that needs the daemon's polling (M6).

### The AI-generated marker

Pages that say they were written by an AI get a yellow header that stays at the top while you scroll (D56). Mark a
whole site, or regions of it, with `generated:` in `sites.yaml` ([spec](spec.md#generated-ai-generated-pages)), or a
single page with `generated` front matter. Publishing adds `/_ddm/ai.css` and `/_ddm/ai.js` to every page, and a meta
tag to the pages a site's block covers; to see it, use `ddm preview` (`mkdocs serve` doesn't add them).

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

## The server

The droplet runs `nixosConfigurations.droplet` from this repo's flake (D44–D47):

| Path | What |
|---|---|
| `nix/package.nix` | ddm, built from `ddm/` |
| `nix/modules/ddm.nix` | `services.ddm`: Caddy, `ddm-deploy`, `ddm-rebuild`, Tailscale's auth helper |
| `nix/hosts/droplet/` | the host: SSH, Tailscale, secrets (`default.nix`); disk and boot (`hardware.nix`) |
| `nix/tests/droplet.nix` | the VM test |
| `secrets/` | agenix: `secrets.nix` says who can decrypt; `*.age` are the encrypted files |

On the droplet:

| Unit | Does |
|---|---|
| `caddy.service` | serves `/var/www` on the Caddyfile generated at build time (D45) |
| `ddm-deploy.timer` / `.service` | `ddm deploy`, 10 min after the last run ends: builds each site whose commit, or whose spec or tools, changed, once its cooldown allows (D48) |
| `ddm-force@<site>.service` | `ddm deploy --force <site>`: one site, now |
| `ddm-rebuild.service` | `nixos-rebuild switch` to the latest `main` on GitHub; started by hand for now |
| `tailscaled`, `tailscale-nginx-auth` | the tailnet, and the "who is this?" socket the private gate asks |

```sh
journalctl -u ddm-deploy -f          # watch a deploy
systemctl start ddm-deploy           # deploy now (whatever needs it)
systemctl start ddm-force@<site>     # rebuild one site now, ignoring its state and cooldown
cat /var/lib/ddm/work/state.json     # what was built, when, and any error
systemctl start ddm-rebuild          # pick up a pushed change to this repo (sites.yaml included)
nixos-rebuild switch --rollback      # undo the last system change
```

Before relying on a change to `nix/`, run the VM test (about a minute, needs KVM):

```sh
nix build .#checks.x86_64-linux.droplet -L
```

It boots the droplet's config offline and checks that Caddy runs the generated config, that a public mount serves
its `current` publish, that http redirects to https, the shared 404 page, Caddy's access to the auth socket, and that
a private host refuses a client outside the tailnet. Certificates, the tailnet itself and real builds need the
internet, so only the droplet can check those.

### Who knocks on the public address (port-watch)

Each night `port-watch.service` summarises the previous day from the logs: connections the firewall refused (by
port and address, and in particular 80/443 that didn't come through Cloudflare), and SSH attempts on port 22
(addresses, usernames tried, and any login from outside the tailnet). Reports are kept 90 days in
`/var/lib/port-watch/`. On the droplet:

```sh
port-watch        # the last 7 daily reports
port-watch 30     # the last 30
port-watch now    # the last 24 hours, from the logs directly
```

It only reads logs; it changes nothing. What the numbers mean for the next steps is in D55.

### Installing (once per wipe)

1. **Secrets.** Generate the droplet's host key, so secrets can be encrypted to it before it exists:

    ```sh
    mkdir -p ~/droplet-keys/etc/ssh
    ssh-keygen -t ed25519 -N "" -C droplet -f ~/droplet-keys/etc/ssh/ssh_host_ed25519_key
    cp ~/droplet-keys/etc/ssh/ssh_host_ed25519_key.pub secrets/keys/droplet.pub
    cd secrets
    nix run github:ryantm/agenix -- -e cloudflare.age   -i ~/.ssh/digitalocean_key   # CLOUDFLARE_API_TOKEN=...
    nix run github:ryantm/agenix -- -e github-token.age -i ~/.ssh/digitalocean_key   # the token alone
    ```

    Commit `secrets/keys/droplet.pub` and the `.age` files, and push `main`: the droplet builds sites, and
    rebuilds itself, from GitHub. Keep `~/droplet-keys` private until the install, then delete it.
2. **Install.** nixos-anywhere logs in as root by key. The old Ubuntu droplet disables root SSH and asks `cathe`
   for a sudo password, so first, once, copy your key to root (it asks for that password):

    ```sh
    ssh -t digi 'sudo install -D -m 600 ~/.ssh/authorized_keys /root/.ssh/authorized_keys &&
      echo "PermitRootLogin prohibit-password" | sudo tee /etc/ssh/sshd_config.d/00-root.conf &&
      sudo systemctl reload ssh'
    ssh -i ~/.ssh/digitalocean_key root@<droplet IP> true   # should print nothing
    ```

    Check the addresses in `nix/hosts/droplet/hardware.nix` still match the droplet (a new droplet gets new
    ones). Then install. This **wipes** the machine at that address:

    ```sh
    nix run github:nix-community/nixos-anywhere -- --flake .#droplet \
      --extra-files ~/droplet-keys root@<droplet IP>
    ```

3. **Join the tailnet.** `ssh root@<droplet IP> tailscale up`, open the link it prints, and log in. Then, in the
   Tailscale admin console, **disable key expiry** for `droplet`, or it drops off the tailnet in 180 days.
4. **Check.** (For the first install, the droplet ran in **staging** (D49): only the dummy site `test`, and
   nothing built until told to. Since going live it deploys every site; see "Going live".) `systemctl status caddy`, then deploy the dummy site by hand and watch it:

    ```sh
    systemctl start ddm-deploy && journalctl -u ddm-deploy -e
    ```

    Then check `https://test.cathe.dev/` from anywhere, and `https://test.the.cathe.dev/` from a machine on your
    tailnet (and that it refuses one that isn't). Certificates arrive once Caddy's DNS challenge succeeds, which
    doesn't need the records to point anywhere. Visiting the hosts does, though (Your to-dos).

### Going live (done 2026-10-18)

The droplet went live on 2026-10-18 (D49), in one commit:
- the `staging` module was removed from `flake.nix`, so the droplet serves `sites.yaml`;
- `services.ddm.autoDeploy = true` was set in `nix/hosts/droplet/default.nix`.

After a push, `ssh root@<droplet IP> systemctl start ddm-rebuild`. The first deploy starts 2 minutes later.
Watch it with `journalctl -u ddm-deploy -f`.

**Testing the server again** without touching the real sites (after a reinstall, say): point `services.ddm.spec`
at `sites.staging.yaml` and set `autoDeploy = false`, as the removed module did (it's in git history before the
go-live commit).

## Planned (once ddm runs, roadmap M6)

| Command | Purpose |
|---|---|
| `ddm status` | Applied system sha; per site and view: last sha, build result, when. |
| `ddm sync [site]` | Check now instead of waiting for the poll. |
| `ddm rollback <site>` | Point `current` back at the previous published build. |
| `journalctl -u ddm -u ddm-rebuild` | Logs for the daemon, and for system rebuilds. |

System rollback is `nixos-rebuild switch --rollback`, which ddm also runs itself when a rebuild fails its health check.

This page will grow into a runbook as each milestone lands.
