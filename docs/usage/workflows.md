---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Workflows

!!! warning "Design, not implemented"
    Workflows replace the fixed build methods of the old `webserver/` tool (removed), which were all stubs.

A workflow is a **named, reusable list of steps** that turns a checkout into a directory of static files. Sites refer
to workflows by name and override their `params`.

## Step kinds

| Step | What it does |
|---|---|
| `run: <cmd>` | Runs a shell command in the checkout, inside the workflow's Nix `env`. |
| `builtin: <name>` | Runs a step written in Go and shipped with ddm (below). |
| `use: <workflow>` | Inlines another workflow's steps, so workflows can be composed (`ocaml-notebook` ends with `use: mkdocs`). |
| `output: <dir>` | Declares which directory, relative to the checkout, is the build result. Defaults to `{{out}}`. |

Any step may have `when:` or `unless:` (e.g. `unless: { view: private }`).

## Built-ins (planned)

| Built-in | Purpose |
|---|---|
| `mkdocs-config` | Writes a minimal `mkdocs.yml` from `params` (theme, colours, input dir) if the repo doesn't have one. |
| `lint-hidden` | For forest views other than private, it fails the build if a hidden (e.g. `private`-tagged) tree is transcluded or linked from a visible one. Forester's hide only removes pages, so a transclusion would still leak the content. |
| `rewrite-links` | Resolves `site://name/path` links. It always runs after the build. |
| `site-directory` | Generates the `docs.cathe.dev` directory from the spec's `sites`, `taxa` and `description`s. |
| `md-ssg` (later) | A small static-site generator built in Go (goldmark, front matter, one layout), for plain markdown repos. |

## Where tools come from

Each workflow names an `env`:

- `flake: "{{repo}}"`: the site repo's own devShell. The forest already has one, which pins the forester fork, so
  ddm needs to know nothing about forester.
- `flake: "{{system}}#mkdocs"`: a devShell in this repo, for repos that don't ship their own.

Builds run with `nix develop <ref> -c …`, **not** as Nix derivations. Derivations are sandboxed with no network, and
site builds follow branches and sometimes fetch dependencies. Each build runs as a throwaway system user, with only
its checkout and scratch output writable. Only ddm itself writes to `/var/www`.

## Porting a GitHub Actions workflow

The workflows in `_example_yamls/` are what these replace. The general mapping:

| GitHub Actions step | ddm equivalent |
|---|---|
| `actions/checkout` | ddm's repo sync, once per new commit |
| Toolchain setup (`ocaml/setup-ocaml`, `actions/setup-python`) | the workflow's `env`; Nix provides it once, not per build |
| Project dependency install (`opam install . --deps-only`, `pip install …`) | a `run:` step, or better, the repo's own devShell |
| The build command (`dune build @doc`, `mkdocs build`, `python generate_docs.py`) | a `run:` step |
| "Create subdomain dir" plus the empty-variable guard before `rm -rf` | ddm's publish step: versioned directories and a symlink swap, so nothing is ever `rm -rf`'d in place |
| `scp` to the server | gone: build and publish happen on the same machine |
| Deriving the subdomain from the repo name (stripping `-docs`, …) | gone: the mount states the host explicitly |

| Example | Becomes |
|---|---|
| `cloakaml-deploy-docs.yml` | `odoc`. Its abandoned "Fix links" `sed` step is what `site://` links + `rewrite-links` solve properly. |
| `demos-deploy-docs-md.yml` | `mkdocs` (it used theme `shadcn`, which would become a param) |
| `demos-deploy-docs.yml` + `generate_docs.py` | `ocaml-notebook` (generate markdown, then `use: mkdocs`) |
