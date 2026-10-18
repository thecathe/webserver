---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-18
---

# Your to-dos

Things only you can do: outside this repo, in accounts Claude doesn't touch. Tick them off by deleting the line, or
moving it to **Done** with a date. Claude adds to this page whenever work leaves something for you.

## Open

- [ ] **Delete the old deploy secrets from GitHub** (added 2026-10-16). The `deploy-dots` Action that used them was
  removed (D19).
    - In `thecathe/webserver` (formerly `cathe-org/dots`) → Settings → Secrets and variables → Actions, delete
      `WEBSERVER_IP`, `WEBSERVER_USER`, `WEBSERVER_SSH_PRIVATE_KEY` and `WEBSERVER_SSH_PASSPHRASE`.
    - The per-repo docs Actions (in `cloakaml`, `demos`, …; see `_example_yamls/`) use the same secret names and
      a `SITE` variable. Delete those once ddm builds those sites (M2/M3 onwards), together with the Actions
      themselves.
- [ ] **Update links to `cathe.work`** before it lapses (D20): CV PDFs, profiles, email signatures, GitHub bios,
  papers.
- [ ] **Activate the new global Claude rules** (design write-ups, AI-content marking) by rebuilding/switching from
  `thecathe/dots` (merged there as `441d40d`).
- [ ] **Decide what `cathe.dev` shows** (added 2026-10-16). It lands on the forest's `@cathe` page (D7), which
  is yours to write.
- [ ] **For M7: DNS for the private domain** (added 2026-10-17). `the.cathe.dev` and `*.the.cathe.dev` point at
  the droplet's **tailnet** IP (100.x). Decide whether these go in public Cloudflare DNS or only in Tailscale split
  DNS (see open questions, "Infrastructure"). Nothing to do until the droplet runs NixOS.

  If Caddy can't get certificates with an account token, make a user token instead (My Profile → API Tokens), with
  the same zones and permissions. It goes in `secrets/cloudflare.age` as `CLOUDFLARE_API_TOKEN=<token>`
  ([runbook](usage/operations.md#installing-once-per-wipe), step 1), never in plain text.
- [ ] **At install: a GitHub token** for the droplet (added 2026-10-18). A fine-grained token with resource owner
  `thecathe`, **All repositories**, and only **Contents: read-only**, with the longest expiry offered (note the
  date). "All repositories" includes ones you create later, so it never needs widening. It goes in
  `secrets/github-token.age`, and it's the only GitHub credential ddm needs.
- [ ] **Decide membranes' dune version** (added 2026-10-17). membranes declares `(lang dune 3.24)`, but Rocq 9.2.0
  (mebi_plugin) needs dune below 3.24, so the two can't share the OCaml docs pool. If membranes uses nothing from
  dune 3.24, lowering it to `(lang dune 3.23)` (and regenerating `membranes.opam`) lets it join: uncomment it in
  `sites.yaml`. Otherwise it waits for a Rocq release that allows newer dune.

- [ ] **PhD: a Project?** (added 2026-10-17). `trees/project/phd.tree` has both `\taxon{Project}` and
  `\taxon{Summary}` commented out, so I left it; indimo and mebi are now Projects.

## Done

- [x] Cloudflare token, encrypted as `secrets/cloudflare.age` (you, 2026-10-18).
- [x] Install in place on the existing droplet, no trial droplet (you, 2026-10-18), resizing it to 2 GB first (CPU and RAM only).
- [x] Moved the checkout to `~/Documents/git/thecathe/webserver` (you, 2026-10-18).
- [x] Tailscale in `dots`, and the machines rebuilt (you, 2026-10-18).

- [x] Moved `cathe-org/dots` to `thecathe/webserver` (you, 2026-10-18); Claude pushed `main` there.

- [x] Tailscale login `thecathe@github` (you, 2026-10-18), now `services.ddm.tailnetOwner`.
- [x] Droplet size (you, 2026-10-18): Basic, 1 GB RAM, 25 GB disk, 1 vCPU.

- [x] Project taxons on `indimo` and `mebi` (Claude, 2026-10-17).
- [x] People private by default: unpublished taxa in the fork (Claude, 2026-10-17).
- [x] How `@jpears` and `@cathe` stay public: answered by D39 and D42 (2026-10-17).
- [x] Looked at the preview (you, 2026-10-17).
- [x] Pushed `ocaml-forester`, `vscode-forester` and the forest, with the forest's pins bumped (you, 2026-10-17).

- [x] Told Claude the session's effort level (`medium`), 2026-10-17.
