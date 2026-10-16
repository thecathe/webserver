---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Workflows

!!! info "Status: implemented (M2), not yet published"
    `ddm build` runs any workflow locally. Publishing the result is M3. The `rewrite-links` and `site-directory`
    built-ins are registered but report "not implemented yet".

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
| `rewrite-links` | M3 | Resolves `site://name/path` links. |
| `site-directory` | M3 | Generates the `docs.cathe.dev` directory from the spec. |
| `md-ssg` | later | A small static-site generator built in Go, for plain markdown repos. |

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
| `odoc` | `dune build @doc` → `_build/default/_doc/_html` | `cloakaml` (`docs` branch) |
| `ocaml-notebook` | `generate_docs.py` over `{{ml_dir}}/**/*.ml` → `docs/` (plus a default `index.md`) → `use: mkdocs` | `thecathe/demos` with `ml_dir: ocaml` (not in the spec) |
| `static` | `output: {{input_dir}}` | none yet |
| `site-directory` | `builtin: site-directory` | M3 |

## Porting a GitHub Actions workflow

The workflows in `_example_yamls/` are what these replace. The general mapping:

| GitHub Actions step | ddm equivalent |
|---|---|
| `actions/checkout` | ddm's repo sync: an exact commit, cleaned of untracked files |
| Toolchain setup (`ocaml/setup-ocaml`, `actions/setup-python`) | the workflow's `env`; Nix provides it once, not per build |
| Project dependency install (`opam install . --deps-only`, `pip install …`) | the devShell (the repo's own, or one in this repo) |
| The build command (`dune build @doc`, `mkdocs build`, `python generate_docs.py`) | a `run:` step |
| "Create subdomain dir" plus the empty-variable guard before `rm -rf` | ddm's publish step (M3): versioned directories and a symlink swap, so nothing is ever `rm -rf`'d in place |
| `scp` to the server | gone: build and publish happen on the same machine |
| Deriving the subdomain from the repo name (stripping `-docs`, …) | gone: the mount states the host explicitly |

| Example | Became |
|---|---|
| `cloakaml-deploy-docs.yml` | `odoc`. Its abandoned "Fix links" `sed` step is what `site://` links + `rewrite-links` solve properly. |
| `demos-deploy-docs-md.yml` | `mkdocs`. It used theme `shadcn`, which isn't in nixpkgs; material is the default. |
| `demos-deploy-docs.yml` + `generate_docs.py` | `ocaml-notebook`; the script now lives in `scripts/`. |
