---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-17
---

# Workflows

!!! info "Status: implemented (M2, M3a); published locally, not yet live"
    `ddm build` runs any workflow; `ddm publish` and `ddm preview` publish the result (see
    [operations](operations.md)). Nothing is live on the internet until the server exists (M5–M7).

A workflow is a **named, reusable list of steps** that turns a checkout into a directory of static files. Sites refer
to workflows by name and override their `params`.

## Trying one

```sh
nix develop -c bin/ddm build indimo                   # mkdocs
nix develop -c bin/ddm build cloakaml                 # odoc
nix develop -c bin/ddm build forest --view private    # forester
nix develop -c bin/ddm build forest --dry-run         # print the resolved plan, run nothing
nix develop -c bin/ddm build ddm --src .              # build from a local repo's committed state
nix develop -c bin/ddm build indimo --set theme=readthedocs   # override a param
```

The result lands in `.ddm/out/<site>/<view>/<commit>/`, which is gitignored. Every step's output is shown as it
runs, and the command fails on the first failing step.

## Step kinds

| Step | What it does |
|---|---|
| `run: <cmd>` | Runs `sh -euc <cmd>` in the checkout, inside `nix develop <env>` if the workflow has an `env`, otherwise with the host's tools. |
| `builtin: <name>` | Runs a step written in Go and shipped with ddm (below). It may have its own `params`. |
| `use: <workflow>` | Inlines another workflow's steps (`ocaml-notebook` ends with `use: mkdocs`). |
| `output: <dir>` | Declares which directory, relative to the checkout, is the build result. It must stay inside the checkout. Without an `output` step, the result is `{{out}}`. |

Any step may have `when:` (run only if every listed param has that value) or `unless:` (skip if they all do), e.g.
`unless: { view: private }`.

## Params

`{{name}}` is replaced everywhere in `run`, `output`, `env.flake` and builtin `params`. A name that isn't defined is
an error, reported before anything runs.

Where values come from, lowest priority first:

1. the workflow's `params` defaults;
2. the built-in variables: `{{repo}}` (the checkout), `{{system}}` (this repo), `{{out}}` (a scratch output dir),
   `{{site}}`, `{{view}}`;
3. the site's or view's `params`;
4. `ddm build --set key=value`.

Built-ins outrank workflow defaults, so a default can never misreport which view is being built.

For `use:`, the inlined workflow's own defaults only fill in what isn't already set: the caller wins. Its `env`, if
it has one, replaces the caller's for its own steps; otherwise it inherits the caller's.

Commands also see `DDM_SITE`, `DDM_VIEW`, `DDM_OUT`, `DDM_REPO` and `DDM_SYSTEM` as environment variables.

## Built-ins

| Built-in | Status | Purpose |
|---|---|---|
| `mkdocs-config` | works | Writes a minimal `mkdocs.yml` if the repo has none. Params: `input_dir` (docs_dir, default `docs`; mkdocs can't use the repo root), `theme` (default `material`), `site_name` (default: the site name). |
| `site-directory` | works (M3a) | Generates pages from the spec. Param `page`: `directory` (default) lists every indexed, publicly mounted site by taxon (`sites.cathe.dev`), or with `taxon` only that taxon's sites (`docs.cathe.dev`: `taxon: docs`), under `title`; `map` lists **every** mount of every site, grouped by host, each with a public/private badge (the private site map at `the.cathe.dev/sites/`). Both write the shared `404.html`. |
| `md-ssg` | later | A small static-site generator built in Go, for plain markdown repos. |

`site://` links are not a workflow step: they are resolved when a build is **published**, because what a link
becomes depends on where it is published (the server, or a local preview). See
[links between sites](spec.md#links-between-sites).

There is no forest-specific built-in. The forest's own `build-forest --strict-private` already fails a build in
which a visible tree links to a hidden one, so ddm knows nothing about forester.

## Where tools come from

Each workflow names an `env`:

| `env.flake` | Used by | Provides |
|---|---|---|
| `"{{repo}}"` | `forester` | The site repo's own devShell. The forest's comes from `thecathe/vscode-forester` and pins the forester fork, `build-forest`, texlive and python. |
| `"{{system}}#mkdocs"` | `mkdocs`, `ocaml-notebook` | mkdocs, mkdocs-material, python3 (this repo's `flake.nix`). |
| `"{{system}}#ocaml-docs"` | `odoc` | ocaml, dune, findlib, odoc, ppx_deriving, yojson. Add a library here when a project needs one. |

Builds run with `nix develop <ref> --command …`, **not** as Nix derivations: derivations have no network and don't
suit builds that follow branches. A repo's first build downloads its toolchain (cloakaml's took about 2 minutes), and
later builds reuse it.

## The workflows in `sites.yaml`

| Workflow | Steps | Verified on |
|---|---|---|
| `forester` | `build-forest {{config}}` (`--strict-private` unless private) → `output/` | `forest`, about 10 s |
| `mkdocs` | `mkdocs-config` → `mkdocs build -d {{out}}` | `indimo` (no mkdocs.yml, so one is generated), `ddm` (this site) |
| `odoc` | `dune build @doc` → `_build/default/_doc/_html` | `cloakaml` (`docs` branch), before the pool; kept for a project built on its own |
| `odoc-pool` | pooled: `opam-pool-switch.sh` → `dune build @doc --profile release` in one opam switch → `_build/default/_doc/_html` | `ocaml` (index), `cloakaml`, `mebi-plugin` (membranes waits on a dune version clash, D34) |
| `ocaml-notebook` | `generate_docs.py` over `{{ml_dir}}/**/*.ml` → `docs/` (plus a default `index.md`) → `use: mkdocs` | `demos` (`ml_dir: ocaml`) |
| `static` | `output: {{input_dir}}` | none yet |
| `site-directory` | `builtin: site-directory` (`page: directory` or `map`) | `docs`, both views |

`forester` is `root_only: true`: forest output links are root-absolute, so the spec rejects mounting a forest under a
path prefix.

## Pooled workflows

A workflow with `pool: true` builds all of its sites in one run. It exists for **OCaml docs**: odoc only turns a
reference such as `Showable.t` into a link when `Showable`'s package is documented in the same build. Built
separately, the reference stays plain text.

How a pooled build runs (`ddm/internal/build/pool.go`):
1. Every member site's repo is synced at its own branch, exactly as for any site.
2. The checkouts are copied (without `.git`, `_build`, `_opam`) into `.ddm/scratch/@<workflow>/<view>/ws/<site>/`,
   next to a two-line `dune-workspace`. That is dune's standard way to build several projects as one, so a library
   found in the workspace is used in place of an installed copy.
3. The workflow runs once, with `{{repo}}` set to the workspace and `{{site}}` set to `@<workflow>`.
4. The result is published once, as `<www>/<view>/@<workflow>/`, with a `ddm-build.json` naming every member's
   repo, branch and commit. Every member's mounts serve it, each with its own `entry`.

**Membership is the spec.** A site is in the pool exactly when it uses the pooled workflow. A project left out stays
a separate site, and references to it stay plain text. A member's push rebuilds the whole pool.

**`odoc-pool`'s environment** is one opam switch, built by `scripts/opam-pool-switch.sh` and cached in
`.ddm/cache/@odoc-pool/` between builds. It tries every member's lockfile (`*.opam.locked`) first, for exact
versions. If the lockfiles conflict, it solves once over every member's `.opam` constraints instead. A pool can only hold projects that agree on one version of every
shared tool. Today membranes needs `dune >= 3.24` while Rocq 9.2.0 (mebi_plugin) needs `dune < 3.24`, so membranes
is left out (D34). The env is `{{system}}#ocaml-opam`: opam plus the system libraries opam packages expect.

The `ocaml` site has no repo. It serves the pool's root, which is odoc's list of every package, as the collated
OCaml documentation page.

`{{cache}}` (every workflow has it) is a directory kept between builds: `.ddm/cache/<site>`, or
`.ddm/cache/@<workflow>` for a pool. Every `run:` step also logs its **peak memory**, for sizing the server.

## Porting a GitHub Actions workflow

The workflows in `_example_yamls/` are what these replace. The general mapping:

| GitHub Actions step | ddm equivalent |
|---|---|
| `actions/checkout` | ddm's repo sync: an exact commit, cleaned of untracked files |
| Toolchain setup (`ocaml/setup-ocaml`, `actions/setup-python`) | the workflow's `env`; Nix provides it once, not per build |
| Project dependency install (`opam install . --deps-only`, `pip install …`) | the devShell (the repo's own, or one in this repo) |
| The build command (`dune build @doc`, `mkdocs build`, `python generate_docs.py`) | a `run:` step |
| "Create subdomain dir" plus the empty-variable guard before `rm -rf` | ddm's publish step: versioned directories and a symlink swap, so nothing is ever `rm -rf`'d in place |
| `scp` to the server | gone: build and publish happen on the same machine |
| Deriving the subdomain from the repo name (stripping `-docs`, …) | gone: the mount states the host explicitly |

| Example | Became |
|---|---|
| `cloakaml-deploy-docs.yml` | `odoc`. Its abandoned "Fix links" `sed` step is what `site://` links, resolved at publish time, solve properly. |
| `demos-deploy-docs-md.yml` | `mkdocs`. It used theme `shadcn`, which isn't in nixpkgs; material is the default. |
| `demos-deploy-docs.yml` + `generate_docs.py` | `ocaml-notebook`; the script now lives in `scripts/`. |
