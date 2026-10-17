---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-17
---

# Your to-dos

Things only you can do: outside this repo, in accounts Claude doesn't touch. Tick them off by deleting the line, or
moving it to **Done** with a date. Claude adds to this page whenever work leaves something for you.

## Open

- [ ] **Delete the old deploy secrets from GitHub** (added 2026-10-16). The `deploy-dots` Action that used them was
  removed (D19).
    - In `cathe-org/dots` → Settings → Secrets and variables → Actions, delete `WEBSERVER_IP`,
      `WEBSERVER_USER`, `WEBSERVER_SSH_PRIVATE_KEY` and `WEBSERVER_SSH_PASSPHRASE`.
    - The per-repo docs Actions (in `cloakaml`, `demos`, …; see `_example_yamls/`) use the same secret names and
      a `SITE` variable. Delete those once ddm builds those sites (M2/M3 onwards), together with the Actions
      themselves.
- [ ] **Update links to `cathe.work`** before it lapses (D20): CV PDFs, profiles, email signatures, GitHub bios,
  papers.
- [ ] **Activate the new global Claude rules** (design write-ups, AI-content marking) by rebuilding/switching from
  `thecathe/dots` (merged there as `441d40d`).
- [ ] **Decide what `cathe.dev` shows** (added 2026-10-16). It lands on the forest's `@cathe` page (D7), which
  is yours to write.
- [ ] **Decide how `@jpears` and `@cathe` stay public** in `forest-public.toml` (added 2026-10-16). See open
  questions, "Forest configs". The lean is removing `\meta{redirect}` from those two trees.
- [ ] **Look at the preview, before the styling session** (added 2026-10-17). From the repo root:
  `nix develop -c bin/ddm preview --serve-only`, then open the addresses it prints (e.g.
  `http://the.cathe.dev.localhost:8080/sites/`, the site map). The first run without `--serve-only` builds everything
  (a few minutes). This site is built from GitHub, so it only shows the latest docs once `main` is pushed;
  meanwhile add `--src ddm=.`.
- [ ] **For M7: DNS for the private domain** (added 2026-10-17). `the.cathe.dev` and `*.the.cathe.dev` point at
  the droplet's **tailnet** IP (100.x). Decide whether these go in public Cloudflare DNS or only in Tailscale split
  DNS (see open questions, "Infrastructure"). Nothing to do until the droplet runs NixOS.
- [ ] **For M5: your Tailscale login name** (added 2026-10-17), e.g. `you@github`, as shown in the Tailscale admin
  console. It becomes `TAILNET_OWNER`, the one identity the private sites let in.
- [ ] **For M5: a Cloudflare API token** with `Zone:DNS:Edit` on `cathe.dev` and `jpears.me` (added 2026-10-17),
  for the wildcard certificates. It is stored encrypted in this repo (agenix), never in plain text.

- [ ] **Tag the project trees** (added 2026-10-17). `projects.cathe.dev` is the forest's `Project` taxon page, which
  lists trees with `\taxon{Project}`. Today only Cloakaml has it; add it to the `indimo` and `mebi` trees (and any
  other project).
- [ ] **Decide membranes' dune version** (added 2026-10-17). membranes declares `(lang dune 3.24)`, but Rocq 9.2.0
  (mebi_plugin) needs dune below 3.24, so the two can't share the OCaml docs pool. If membranes uses nothing from
  dune 3.24, lowering it to `(lang dune 3.23)` (and regenerating `membranes.opam`) lets it join: uncomment it in
  `sites.yaml`. Otherwise it waits for a Rocq release that allows newer dune.

- [ ] **Push the forest's `main`** (added 2026-10-17). `forest-public.toml` and the `notes` tag page are merged into
  your forest's local `main` (not pushed). ddm builds the forest from GitHub, so until you push, the public forest
  only builds with `--src`: `bin/ddm preview --src forest=$HOME/Documents/git/thecathe/forest`.
- [ ] **Three people without a redirect stay public** (added 2026-10-17): `@lstefanesco`, `@afrancalanza`,
  `@mpretnar` have no `\meta{redirect}`, so the public forest still builds their pages. Give each a redirect (to
  their own page), or tag them `private`, or decide they can stay.

## Done

- [x] Told Claude the session's effort level (`medium`), 2026-10-17.
