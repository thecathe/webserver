---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-18
---

# Site spec (v2)

!!! info "Status: validated, buildable and publishable locally"
    `sites.yaml` is loaded and validated by `ddm check` (M1). `bin/ddm build` builds any site (M2), and
    `bin/ddm preview` publishes and serves them locally (M3a). Nothing is live until the server exists (M5–M7).
    Names may still change; see [open questions](../design/open-questions.md#spec-naming).

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
| `caddyfile` | `/var/lib/ddm/Caddyfile` | Unused since D45: the server's Caddyfile is built with the system. |

### `domains`, `private_domain`, `taxa`

`domains` lists the apex domains ddm may publish under. Every mount's host must be a domain or a subdomain of one.
A stale `cathe.work` mount is therefore an error, and so is a lookalike such as `notcathe.dev`.

`private_domain` (here `the.cathe.dev`) is where **private** mounts live. It must sit under one of the `domains`.
It and its subdomains resolve to the server's tailnet address, so:
- every `access: private` mount must be on the private domain or a subdomain of it;
- every public mount must be off it.

A private site can therefore never end up on a public host by a typo, and a public one can never be hidden by
accident.

`taxa` declares the vocabulary that sites may classify themselves with. It drives the `sites.cathe.dev` directory, and `docs.cathe.dev` lists the `docs` taxon.
If `taxa` is omitted, any taxon is accepted.

### `nav`

The toolbar's links, shown at the top of every published page, in order:

```yaml
nav:
  - { title: cathe.dev, link: "mount://cathe.dev/" }
  - { title: site map, link: "mount://the.cathe.dev/sites/" }
```

`link` is `site://name/path`, `mount://host/path` or an `https://` URL. A link to a private mount only appears on
private pages. Besides these links, the toolbar shows the same page in the site's other views and what the page was
built from; see [operations](operations.md#the-toolbar).

### `workflows.<name>`

| Field | Meaning |
|---|---|
| `env.flake` | Where build tools come from. `"{{repo}}"` is the site repo's own devShell, and `"{{system}}"` (or `"{{system}}#name"`) a devShell of this repo. Steps run inside `nix develop <ref> -c`. Omit it for steps that need no tools. |
| `params` | Defaults, which a site or view can override. Substituted as `{{name}}`. |
| `steps` | Run in order. Each is **exactly one** of `run`, `builtin`, `use`, `output`. See [workflows](workflows.md). |
| `pool` | `false` | Build every site that uses this workflow (in the same view) **together**: each site's repo is synced as usual, all the checkouts are copied into one workspace, and the steps run once with `{{repo}}` set to it. The one result is served by every member's mounts. A member then needs no mounts of its own. See [workflows](workflows.md#pooled-workflows). |
| `root_only` | `false` | The output's links are root-absolute (forester's `base-url="/"`), so its sites can only be mounted at path `/`. A mount with any other `path` is an error. |

Validation checks:
- every `builtin` is one ddm knows (`mkdocs-config`, `site-directory`, `md-ssg`);
- every `use` names an existing workflow, and there are no `use` cycles (reported as e.g. `a → b → a`);
- `params` appear only on `builtin` steps.

Built-in variables: `{{repo}}` (the checkout), `{{system}}` (this repo at the applied sha), `{{out}}` (the scratch
output dir), `{{cache}}` (kept between builds), `{{view}}`, `{{site}}` (`@<workflow>` in a pooled build).

### `sites.<name>`

The **name** is how other sites link here (see [links between sites](#links-between-sites)). It must be lowercase
letters, digits and `-`, because it also becomes a directory name.

| Field | Default | Meaning |
|---|---|---|
| `repo` | none | `owner/name` on GitHub, a full clone URL, or an absolute path, cloned with the server's single read-only token. Omit it for generated sites (e.g. `docs`). |
| `branch` | `main` | |
| `workflow` + `params` | | Shorthand for a single `public` view. Use either this or `views`, not both. |
| `views.<view>` | | Several builds of the same repo, e.g. `public` / `notes` / `private`. Each needs a `workflow`, and may have `params`. |
| `mounts[]` | | Where outputs are served. At least one is required, except for a site built only by a pooled workflow: the pool's other mounts serve it. See below. |
| `access` | `public` | The default `access` for every mount of this site: `public` or `private`. A mount can override it. |
| `taxa` | | Classification, from the declared `taxa`. |
| `description` | | One line for the directory. |
| `indexed` | `true` | `false` hides the site from the generated directories (`sites.cathe.dev`, `docs.cathe.dev`). |

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

In any source (a forest `.tree`, markdown, an odoc docstring), write

```text
site://<name>/<path>
```

for example [`site://cloakaml/`](site://cloakaml/) or [`site://forest/index/`](site://forest/index/): both links
in this sentence are written that way in this page's source.

When a build is **published**, every `site://` link in its HTML, XML, XSL, CSS, JS, JSON and SVG files becomes the
target's URL:
- **Same view first.** A private build links to the target's private view if it has one, and to its public view
  otherwise.
- **The first mount of that view is canonical**, so `site://forest/…` goes to `forest.cathe.dev`, the forest's first
  public mount.
- **Always absolute.** One build may be served on several hosts (the forest is on three), so a root-relative link
  could land on the wrong one.
- **Never public → private.** A publicly served build that links to a private mount fails, since the link would
  publish a private hostname.
- **Unknown names** are left as they are and reported. That is a warning, or an error with `--strict`, and always
  an error for prod.

Links are resolved at publish time, not in a workflow step, because the result depends on where the build is
published. In a [local preview](operations.md#local-preview) the same link becomes
`http://cloakaml.cathe.dev.localhost:8080/`.

**From the forest, by redirect tree.** A tree such as `trees/site-cloakaml.tree` containing

```text
\title{cloakaml}
\taxon{Reference}
\meta{redirect}{[cloakaml docs](site://cloakaml/)}
```

lets every other tree link to cloakaml with an ordinary `[cloakaml](site-cloakaml)`. Verified on 2026-10-17 with a
test tree in both kinds of forest config:
- **`redirect` behaviour** (`forest-public.toml`, the public views): the tree isn't built, and every link to it
  becomes a link to cloakaml's docs.
- **`external` behaviour** (`forest.toml`, the private view): the tree is built, and its meta is shown as a link to
  cloakaml's docs.

Use the `[text](site://…)` form. The bare `\meta{redirect}{site://cloakaml/}` redirects correctly, but in the
private view its meta is shown as plain text, which isn't a link position, so it stays unrewritten.

ddm's own generated pages also use `mount://<host>/<path>`, which names one exact mount (the site map links every
address, not only the canonical one). It is resolved the same way.

This replaces the old `DomainLink{LinkTo, LinkPlaceholder, Path}` design: the placeholder becomes one fixed scheme,
so no per-link configuration is needed. Spike S1 confirmed that forester and mkdocs pass `site://` through
untouched. odoc is untested.
