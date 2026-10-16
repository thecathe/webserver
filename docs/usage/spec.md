---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Site spec (v2)

!!! warning "Design, not implemented"
    This is the target schema for `sites.yaml`. The code still reads the old `deploy.yaml` schema
    (`webserver/internal/config/config.go`: `repos[]` with `build_method` and `domain[]`). The field names here are
    a starting point; see [open questions](../design/open-questions.md#spec-naming).

The spec has four top-level parts: **system** (the server itself), **domains**, **workflows** (reusable build
recipes) and **sites** (repo + workflow + where it's mounted).

## A complete example

```yaml
system:
  flake: github:thecathe/server-dots   # ddm rebuilds nixosConfigurations.<host> from the exact sha it saw
  host: droplet
  poll: 10m

domains: [cathe.dev, jpears.me]        # every mount's host must fall under one of these

taxa: [docs, personal, work, project, tool, demo, media, presentation, resource, blog, notes]

workflows:
  forester:
    env: { flake: "{{repo}}" }          # use the content repo's own devShell
    params: { view: public }
    steps:
      - run: forest-gen config --view {{view}}        # forest site.yaml → forest.{{view}}.toml
      - run: forest-gen all --config forest.{{view}}.toml
      - builtin: lint-hidden                          # fail closed if a hidden tree leaks into this view
        unless: { view: private }
      - run: forester build forest.{{view}}.toml
      - output: output/

  mkdocs:
    env: { flake: "{{system}}#mkdocs" }  # toolchain pinned in this repo's flake
    params: { input_dir: ./, theme: material }
    steps:
      - builtin: mkdocs-config           # write a minimal mkdocs.yml from params if the repo has none
      - run: mkdocs build -d {{out}}

  odoc:
    env: { flake: "{{repo}}" }
    steps:
      - run: dune build @doc
      - output: _build/default/_doc/_html

  ocaml-notebook:                        # .ml files with doc comments → markdown → mkdocs
    steps:
      - run: python3 {{system}}/scripts/generate_docs.py {{input_dir}} docs/
      - use: mkdocs

  site-directory:
    steps:
      - builtin: site-directory
        params: { domain: cathe.dev }

sites:
  forest:
    repo: thecathe/forest
    taxa: [personal, notes]
    views:
      public:  { workflow: forester, params: { view: public } }
      notes:   { workflow: forester, params: { view: notes } }
      private: { workflow: forester, params: { view: private } }
    mounts:
      - { host: forest.cathe.dev, entry: /index/ }
      - { host: jpears.me,        entry: /@jpears/ }
      - { host: notes.cathe.dev,  view: notes,   entry: /index/ }
      - { host: the.cathe.dev,    view: private, entry: /index/, access: tailnet-owner }

  docs:
    workflow: site-directory
    mounts: [ { host: docs.cathe.dev } ]

  ddm-docs:                              # this site
    repo: thecathe/server-dots
    workflow: mkdocs
    taxa: [docs, project, tool]
    description: Design and usage docs for ddm
    mounts: [ { host: ddm.cathe.dev } ]

  cloakaml:
    repo: thecathe/cloakaml
    workflow: odoc
    taxa: [docs, project, tool]
    description: "..."
    mounts: [ { host: cloakaml.cathe.dev } ]
```

## Fields

### `system`

| Field | Meaning |
|---|---|
| `flake` | The flake ddm rebuilds the server from. ddm always pins the exact commit it saw, never a branch. |
| `host` | Which `nixosConfigurations.<host>` to switch to. |
| `poll` | How often to check this repo and every site repo for new commits. |

### `domains`, `taxa`

`domains` lists the apex domains ddm may publish under. A mount on any other host, for example a stale `cathe.work`
entry, is a validation error. `taxa` declares the vocabulary that sites may classify themselves with. It drives the
`docs.cathe.dev` directory.

### `workflows.<name>`

| Field | Meaning |
|---|---|
| `env` | Where the build tools come from. `flake: "{{repo}}"` means the site repo's own devShell, and `"{{system}}#x"` means a devShell from this repo. Steps run inside `nix develop <ref> -c`. |
| `params` | Defaults, which a site can override. Substituted as `{{name}}`. |
| `steps` | Run in order. See [workflows](workflows.md) for the step kinds. |

Built-in variables: `{{repo}}` (the checkout), `{{system}}` (this repo at the applied sha), `{{out}}` (the scratch
output dir), `{{view}}`, `{{site}}`.

### `sites.<name>`

The **name** is how other sites link here (see [links between sites](#links-between-sites)).

| Field | Meaning |
|---|---|
| `repo` | `owner/name` on GitHub, cloned with the server's single read-only token. Omit it for generated sites (e.g. `docs`). |
| `branch` | Default `main`. |
| `workflow` + `params` | Shorthand for a single `public` view. |
| `views.<view>` | Several builds of the same repo, e.g. `public` / `notes` / `private`. Each has its own `workflow` + `params`. |
| `mounts[]` | Where outputs are served: `host`, optional `path` (a prefix such as `/docs/x`, your "suffix" option), `view` (default `public`), `entry` (where `/` redirects to), and `access`. |
| `taxa`, `description`, `indexed` | Used by the `docs.cathe.dev` directory. `indexed: false` hides the site from it. |

`access: tailnet-owner` serves the mount only on the tailnet, and only to your Tailscale identity; see
[architecture](../design/architecture.md#private-access).

## Links between sites

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
