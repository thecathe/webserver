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

- [ ] **Push `main` and rebuild the droplet** (added 2026-10-18): `systemctl start ddm-rebuild`. It brings the
  firewall that lets only Cloudflare reach the web ports (D53) and the devlog site (D54). Afterwards,
  `curl -m 5 http://<droplet address>/` from outside should time out, while the sites still load.
- [ ] **Reset other clones of this repo** (added 2026-10-18), e.g. on the laptops: the history was rewritten, so
  `git fetch && git reset --hard origin/main` (after checking they hold nothing unpushed). Optionally, ask GitHub
  Support to purge cached views of the old commits; nothing links to them any more.
- [ ] **Keep `thecathe/devlog` private** (added 2026-10-18). Its public site is built from it with `publish.yaml`
  (D54); the repo and its history keep everything.
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
- [ ] **Tag the rest of the forest `public`** (added 2026-10-18), deliberately, tree by tree or with
  `trees/public/` (D51). Today only `index`, `cv`, `@jpears` and `@cathe` are public, so the entry pages ddm
  serves for `indimo`, `mebi` and projects are empty or missing until their trees are tagged. The survey of what
  *was* public under the old rule is in `.ddm/forest-survey.md` (local only).
- [ ] **Before going live: read the generated robots.txt files** (added 2026-10-18):
  `nix develop -c bin/ddm caddyfile` shows every host's, or `curl https://<host>/robots.txt` once live. The
  crawler lists are in `sites.yaml` under `robots:` (D50).
- [ ] **Decide membranes' dune version** (added 2026-10-17). membranes declares `(lang dune 3.24)`, but Rocq 9.2.0
  (mebi_plugin) needs dune below 3.24, so the two can't share the OCaml docs pool. If membranes uses nothing from
  dune 3.24, lowering it to `(lang dune 3.23)` (and regenerating `membranes.opam`) lets it join: uncomment it in
  `sites.yaml`. Otherwise it waits for a Rocq release that allows newer dune.

- [ ] **PhD: a Project?** (added 2026-10-17). `trees/project/phd.tree` has both `\taxon{Project}` and
  `\taxon{Summary}` commented out, so I left it; indimo and mebi are now Projects.

## Done

- [x] The May 2026 Cloudflare user token (`cfut_…`, from the old Caddyfile) deleted from My Profile; nothing used
  it any more. The server uses only the account token in `secrets/cloudflare.age` (you, 2026-10-18).
- [x] Old deploy secrets: none left in this repo or `demos`; `cloakaml`'s four and its `SITE` variable deleted,
  and its docs Action removed from `docs` and `main` (Claude, at your request, 2026-10-18).
- [x] The leaked Cloudflare token scrubbed from this repo's history, force-pushed, and every quoted commit hash
  updated (Claude, at your request, 2026-10-18).
- [x] DNS: `test.the.cathe.dev` → the tailnet address, and a proxied `*.jpears.me` wildcard, which covers
  `cv.jpears.me` (you, 2026-10-18).
- [x] Pushed `ocaml-forester`, `vscode-forester` and the forest for the publish-by-tag rule (you, 2026-10-18).
- [x] GitHub token, encrypted as `secrets/github-token.age`; droplet resized to 2 GB (you, 2026-10-18).
- [x] NixOS installed on the droplet, joined to the tailnet (you and Claude, 2026-10-18).
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
