---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Site spec (v2)

!!! info "Status: validated and buildable, not yet published"
    `sites.yaml` is loaded and validated by `ddm check` (roadmap M1, `ddm/internal/spec`). `bin/ddm build` builds any site locally (M2).
    Nothing is published until M3. Names may still change; see [open questions](../design/open-questions.md#spec-naming).

The spec has seven top-level parts:
- **system**: where the server's own config comes from;
- **paths**: where ddm keeps things;
- **domains**, and the **private_domain** where private mounts live;
- **taxa**: the vocabulary used to classify sites;
- **workflows**: reusable build recipes;
- **sites**: repo + workflow + where it's mounted.

## Checking it

```sh
nix develop -c bin/ddm check   # valid? one-line summary
nix develop -c bin/ddm sites   # every mount: host, path, site, view, workflow, access
```

Every problem is reported at once, one per line, each naming exactly where it is (e.g.
`sites.ddm.mounts[0]: host "ddm.cathe.work" is not under any declared domain`). Unknown keys are errors, so a typo
like `reop:` fails instead of being silently ignored.

## The current spec

This is the repo's real `sites.yaml`, included verbatim:

```yaml
--8<-- "sites.yaml"
```

## Fields

### `system`

| Field | Default | Meaning |
|---|---|---|
| `flake` | none | The flake ddm rebuilds the server from. ddm always pins the exact commit it saw, never a branch. |
| `host` | none (required if `flake` is set) | Which `nixosConfigurations.<host>` to switch to. |
| `poll` | `10m` | How often to check this repo and every site repo for new commits. Go duration syntax: `90s`, `10m`, `1h`. |

### `paths`

| Field | Default | Meaning |
|---|---|---|
| `work` | `/var/lib/ddm/work` | Site checkouts and scratch build output. |
| `www` | `/var/www` | Publish root: `<www>/<view>/<site>/<sha>/` plus a `current` symlink. |
| `caddyfile` | `/var/lib/ddm/Caddyfile` | Where the generated Caddy config is written. |

### `domains`, `private_domain`, `taxa`

`domains` lists the apex domains ddm may publish under. Every mount's host must be a domain or a subdomain of one.
A stale `cathe.work` mount is therefore an error, and so is a lookalike such as `notcathe.dev`.

`private_domain` (here `the.cathe.dev`) is where **private** mounts live. It must sit under one of the `domains`.
It and its subdomains resolve to the server's tailnet address, so:
- every `access: private` mount must be on the private domain or a subdomain of it;
- every public mount must be off it.

A private site can therefore never end up on a public host by a typo, and a public one can never be hidden by
accident.

`taxa` declares the vocabulary that sites may classify themselves with. It drives the `docs.cathe.dev` directory.
If `taxa` is omitted, any taxon is accepted.

### `workflows.<name>`

| Field | Meaning |
|---|---|
| `env.flake` | Where build tools come from. `"{{repo}}"` is the site repo's own devShell, and `"{{system}}"` (or `"{{system}}#name"`) a devShell of this repo. Steps run inside `nix develop <ref> -c`. Omit it for steps that need no tools. |
| `params` | Defaults, which a site or view can override. Substituted as `{{name}}`. |
| `steps` | Run in order. Each is **exactly one** of `run`, `builtin`, `use`, `output`. See [workflows](workflows.md). |
| `root_only` | `false` | The output's links are root-absolute (forester's `base-url="/"`), so its sites can only be mounted at path `/`. A mount with any other `path` is an error. |

Validation checks:
- every `builtin` is one ddm knows (`mkdocs-config`, `site-directory`, `md-ssg`);
- every `use` names an existing workflow, and there are no `use` cycles (reported as e.g. `a → b → a`);
- `params` appear only on `builtin` steps.

Built-in variables: `{{repo}}` (the checkout), `{{system}}` (this repo at the applied sha), `{{out}}` (the scratch
output dir), `{{view}}`, `{{site}}`.

### `sites.<name>`

The **name** is how other sites link here (see [links between sites](#links-between-sites)). It must be lowercase
letters, digits and `-`, because it also becomes a directory name.

| Field | Default | Meaning |
|---|---|---|
| `repo` | none | `owner/name` on GitHub, a full clone URL, or an absolute path, cloned with the server's single read-only token. Omit it for generated sites (e.g. `docs`). |
| `branch` | `main` | |
| `workflow` + `params` | | Shorthand for a single `public` view. Use either this or `views`, not both. |
| `views.<view>` | | Several builds of the same repo, e.g. `public` / `notes` / `private`. Each needs a `workflow`, and may have `params`. |
| `mounts[]` | | Where outputs are served. At least one is required. See below. |
| `access` | `public` | The default `access` for every mount of this site: `public` or `private`. A mount can override it. |
| `taxa` | | Classification, from the declared `taxa`. |
| `description` | | One line for the directory. |
| `indexed` | `true` | `false` hides the site from the `docs.cathe.dev` directory. |

### `mounts[]`

| Field | Default | Meaning |
|---|---|---|
| `host` | required | Must fall under a declared domain. |
| `path` | `/` | A prefix under the host, e.g. `guide/` → `/guide`. |
| `view` | `public` | Which of the site's views to serve. |
| `entry` | none | Where `/` redirects to, e.g. `/@jpears/`. Must start with `/`. |
| `access` | the site's `access` | `public`, or `private`: served only on the tailnet, and only to your Tailscale identity (see [architecture](../design/architecture.md#private-access)). Must match the host: private mounts on `private_domain`, public ones off it. |

Each `host` + `path` may be mounted only once across the whole spec, whatever the view.

## Links between sites

!!! warning "Not implemented yet (M3)"

In any source (a forest `.tree`, markdown, an odoc docstring), write

```text
site://<name>/<path>
```

for example `site://cloakaml/Cloakaml/index.html`. After each build, the `rewrite-links` built-in replaces it with
the target site's URL **in the same view**. A private build links to the private copy if the target has one, and to
the public copy otherwise. A link to a site on the same host becomes a relative link, and an unknown name produces a
loud warning (an error in strict mode).

This replaces the old `DomainLink{LinkTo, LinkPlaceholder, Path}` design: the placeholder becomes one fixed scheme,
so no per-link configuration is needed. Whether forester passes `site://` through untouched is spike S1 on the
[roadmap](../design/roadmap.md#spikes).
