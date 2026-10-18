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
- [ ] **For M7: DNS for the private domain** (added 2026-10-17). `the.cathe.dev` and `*.the.cathe.dev` point at
  the droplet's **tailnet** IP (100.x). Decide whether these go in public Cloudflare DNS or only in Tailscale split
  DNS (see open questions, "Infrastructure"). Nothing to do until the droplet runs NixOS.
- [ ] **For M5: tell Claude your Tailscale login name** (added 2026-10-17), e.g. `you@github`, as the admin
  console's **Users** page shows it. It becomes `services.ddm.tailnetOwner` in `nix/hosts/droplet/default.nix`, the
  one identity the private sites let in. It isn't secret.
- [ ] **For M5: a Cloudflare API token** with `Zone:DNS:Edit` on `cathe.dev` and `jpears.me` (added 2026-10-17),
  for the wildcard certificates. Cloudflare → My Profile → API Tokens → Create Token → "Edit zone DNS", both
  zones. It goes in `secrets/cloudflare.age` as `CLOUDFLARE_API_TOKEN=<token>`
  ([runbook](usage/operations.md#installing-once-per-wipe), step 1), never in plain text.
- [ ] **For M5: a GitHub token** that can read the private site repos (added 2026-10-18). A fine-grained token
  (GitHub → Settings → Developer settings → Fine-grained tokens) for `thecathe/forest`, `demos`, `INDIMO` and
  `membranes`, with **Contents: read-only** only. It goes in `secrets/github-token.age`.
- [ ] **For M5: the droplet's size** (added 2026-10-18): RAM and disk, from the DigitalOcean console. Builds and
  nixos-anywhere's installer each want about 1 GiB of RAM.
- [ ] **For M5: a trial droplet, or straight to M7?** (added 2026-10-18). Installing on the real droplet wipes the
  live sites. A throwaway droplet for an hour proves the install first, for a few cents (open questions).
- [ ] **Push `main`** when you're happy with it (added 2026-10-18). GitHub's `cathe-org/dots` is still at
  `8ae185a`, so the droplet (and `ddm deploy`'s `ddm` site) would see the old repo.

- [ ] **Decide membranes' dune version** (added 2026-10-17). membranes declares `(lang dune 3.24)`, but Rocq 9.2.0
  (mebi_plugin) needs dune below 3.24, so the two can't share the OCaml docs pool. If membranes uses nothing from
  dune 3.24, lowering it to `(lang dune 3.23)` (and regenerating `membranes.opam`) lets it join: uncomment it in
  `sites.yaml`. Otherwise it waits for a Rocq release that allows newer dune.

- [ ] **PhD: a Project?** (added 2026-10-17). `trees/project/phd.tree` has both `\taxon{Project}` and
  `\taxon{Summary}` commented out, so I left it; indimo and mebi are now Projects.

## Done

- [x] Project taxons on `indimo` and `mebi` (Claude, 2026-10-17).
- [x] People private by default: unpublished taxa in the fork (Claude, 2026-10-17).
- [x] How `@jpears` and `@cathe` stay public: answered by D39 and D42 (2026-10-17).
- [x] Looked at the preview (you, 2026-10-17).
- [x] Pushed `ocaml-forester`, `vscode-forester` and the forest, with the forest's pins bumped (you, 2026-10-17).

- [x] Told Claude the session's effort level (`medium`), 2026-10-17.
