---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-17
---

# Adding a site

!!! warning "Partly implemented"
    Writing the entry and `ddm build` work today (M2). Automatic publishing needs the ddm daemon (M6).

## The short version

1. Make sure the repo builds with a workflow that already exists (odoc, mkdocs, forester, ocaml-notebook), or add
   a new workflow to the spec (see [workflows](workflows.md)).
2. Add an entry under `sites:` in `sites.yaml`:

    ```yaml
    sites:
      my-project:
        repo: thecathe/my-project
        workflow: mkdocs
        params: { input_dir: docs/ }
        taxa: [docs, project]
        description: One line for the sites.cathe.dev directory
        mounts: [ { host: my-project.cathe.dev } ]
    ```

3. Commit and push this repo. That's all. You don't need to add an Action, a deploy key or a secret to
   `my-project`.

Within one poll interval, ddm sees the new commit, applies it, clones `my-project`, builds it, publishes it,
regenerates the Caddyfile and reloads Caddy. Any `*.cathe.dev` name is covered by the wildcard DNS record and
certificate, so no DNS change is needed either.

## Checking on it

- `nix develop -c bin/ddm build my-project` runs the same workflow locally, with no publishing (works now). Use it
  before pushing.
- *(M6)* `ddm status` shows each site's last built sha, and whether the build succeeded.
- *(M6)* `ddm sync my-project` re-checks now instead of waiting for the next poll.

## Variations

- **A team-facing project site**: the same thing, under any `cathe.dev` subdomain. If the site should only redirect
  somewhere else (a shared drive, a GitHub Pages site you haven't migrated yet), a redirect mount is an
  [open question](../design/open-questions.md#team-project-sites).
- **Several hosts or paths**: add more `mounts`. Use `path:` to mount under a prefix, and `entry:` to choose the
  landing page.
- **Linking from the forest**: write `site://my-project/...` in a tree. See
  [links between sites](spec.md#links-between-sites).
- **Hidden from the directory**: `indexed: false`.
