---
generated:
  by: Claude
  model: Opus 5.5
  effort: unrecorded
  date: 2026-10-16
  edited: 2026-10-16
---

# Development log

A running record of how this project has been designed and built, so the interesting parts don't get lost:
trade-offs weighed, ideas tried and dropped, and what each session left behind. Entries are chronological, oldest
first, so the log reads as a narrative. The current state of each decision is in the decision record
(`docs/design/record.md`); this log is the story behind it.

| Tag | Meaning |
|---|---|
| **Design** | A decision or design exploration, whether or not anything was built. |
| **Feature** | New capability. |
| **Refactor** | Same behaviour, different structure. |
| **Bug fix** | Behaviour was wrong; now it isn't. |
| **Tooling** | Build, dev environment, CI or editor config. |
| **Docs** | Documentation only. |

---

## 2026-05 to 2026-09: before the log (back-filled)

Reconstructed from git history and the curriculum files that sessions 1–3 left in the repo. Commits before
2026-10-16 carry no `Co-Authored-By: Claude` trailer; the curriculum sessions produced plans, not commits.

- **2026-05-12/13: Tooling.** A first `Caddyfile` for `cathe.dev`, `cathe.work` and `jpears.me`, with wildcard
  subdomains that probe the filesystem per request for odoc output (`_html/`), mkdocs output (`site/`) or plain
  files. Certificates via Cloudflare DNS-01. The `deploy-dots` Action SSHes to the droplet and runs `git pull`.
  `e9c3688` "fixing exposed token -- oops" removed a token committed by accident, an early sign that secrets
  belong in exactly one place.
- **2026-08-04/07: Design.** The README sketches the idea: one repo with CI that tells the web server which repos to
  pull and how to build each. The goal is that hosting a new site never again means per-repo Actions and secrets,
  or moving a repo into a GitHub org. It also sketched the first `deploy.yaml`.
- **2026-09-06: Feature (partial)** (`8ae185a` "WIP: nix, yaml"). A Go module `webserver/` that embeds Caddy, with
  a `deploy` subcommand. Clone-or-reset sync, publishing to `/var/www/<domain>/subdomains/<name>` and Caddy reload
  all work. All four build methods (`md`, `mkdocs`, `odoc`, `ocaml`) are stubs. It came with a dev-shell flake and
  three real GitHub Actions workflows collected in `_example_yamls/` as the pipelines to replace.
- **2026-09 curriculum sessions 1–3: Design.** You chose to implement the system yourself, with Claude as
  architect/teacher, which produced a 14-module curriculum. Decisions from then:
  - **phased Nix**: home-manager on Ubuntu first, NixOS later by blue/green swap;
  - builds stay in Go, not Nix derivations (no network in the sandbox);
  - the `ocaml` method chains into mkdocs, and `generate_docs.py` stays Python;
  - `DomainLink{LinkTo, LinkPlaceholder, Path}` as a build-time find/replace for cross-site links. This was a
    properly designed version of the abandoned `sed` "Fix links" step in `cloakaml-deploy-docs.yml`.

  Session 3 reviewed a first attempt at Module 1 (moving `Linked` to `Repo`, adding validation) and found a
  namespace bug: `LinkTo` was checked against repo names, not domain hostnames. That commit, `32a1e49`, is no
  longer in the history. Its work presumably continued in the uncommitted changes discarded below. The session
  also weighed Hugo against a DIY goldmark SSG for `md`. Hugo is CLI-only and would just be a second "shell out to
  an external SSG", like mkdocs, so the DIY route was preferred for its learning value.

---

## 2026-10-16: from webserver to deployment manager; docs as a site

Branch `docs/dev-site`. **Design** + **Docs** + **Tooling**. No ddm code changed.

**What prompted it.** You wanted to make the forest's HTML output the main landing page, with links out to project
docs, blog posts, the CV, profile, papers and works. You also wanted a private version of the notes. Two threads
joined this:
- the older "dots-deployment-manager" idea, which pulls new commits, rebuilds the system config, updates itself,
  and builds a declared set of repos with reusable workflows;
- the earlier forest-privacy work: `redirect` meta, `[forest.tag.private] behaviour = "hide"`, and separate dev and
  webserver configs.

**The design journey.**
- *The private view* started as two candidate ideas: (a) a private build in an entirely separate directory, so no
  paths overlap; (b) a toggle usable only by an authenticated service or over Tailscale. They turned out to be
  complementary rather than alternatives. Separate builds into separate roots mean a public URL can never serve a
  private file. Because relative paths match across views, the "toggle" reduces to swapping the host. Access is
  then a question of how that host is reached.
- *Tailscale* had four options:
  - (A) network-only: bind to the tailnet IP;
  - (B) A plus `forward_auth` to tailscaled's whois, which gives per-user identity with stock Caddy and your own
    domain;
  - (C) the `caddy-tailscale` plugin: experimental, needs a custom build;
  - (D) `tsidp` OIDC: overkill.

  B was chosen. You asked for a widget that does nothing for anyone but you. It was deferred, because putting it on
  public pages reveals the private hostname and may hit browser private-network rules (spike S4).
- *Self-update* was the subtle part of "a manager that rebuilds the system it runs on". If ddm ran `nixos-rebuild`
  itself, switching to a generation with a new ddm would restart ddm mid-rebuild. The answer is to run the rebuild
  in its own unit (as NixOS's `system.autoUpgrade` does), so the restart only hits the daemon. A state file makes
  the restart harmless.
- *Caddy embedded in the Go binary* had been a reasonable early choice, but under self-update it means every ddm
  update restarts the web server. So Caddy is split out into its own NixOS service.
- *Fixed build methods vs workflows.* The `builders` map would need a ddm release for every new kind of site.
  Named YAML workflows with `run:` steps in a Nix env, Go built-ins, and `use:` composition make them data.
  Choosing the env as "the repo's own devShell where it has one" means ddm needn't know anything about forester or
  dune.
- *Cross-site links* generalised `DomainLink`. Instead of a configured placeholder per link, one fixed
  `site://name/path` scheme is resolved per view after every build.
- *Home-manager on Ubuntu* (the 2026-09 plan) no longer made sense once ddm owns the system config, so the droplet
  goes straight to NixOS.
- *Repos.* This repo stays separate from your machines' `thecathe/dots`, but moves back to your personal account.
  It's the one place holding secrets, as a single read-only GitHub token, so new projects need none. Polling
  (`git ls-remote`) over webhooks follows from the same goal: zero per-repo setup.
- *Hosts.* `jpears.me` lands on the forest's `/@jpears/`, `forest.cathe.dev` on `/index/`, `notes.cathe.dev` is a
  `notes`-tagged view, `the.cathe.dev` is private, and `docs.cathe.dev` is a directory generated from the spec, with
  each site classified by taxa. `cathe.work` is being dropped. The notes view needs a small forester-fork addition
  ("keep only trees with these tags"), because per-tag hide can't express it.
- *The forest's script-only TOML keys* move to a `site.yaml`, which generates each view's TOML, giving one source of
  truth.
- *Working mode changed.* You no longer have time to learn this by building it, so Claude implements, and the docs
  become the interface for learning to use it. That raised a second wish: keep the design process itself, for this
  and other projects, so it can be appreciated and iterated on later. Hence this log, a global rule to keep logs
  like it, and the docs becoming a site of their own.

**Rejected or deferred.**
- Same-host auth toggle (private files behind the public listener).
- `caddy-tailscale` and `tsidp` for now.
- The identity widget (deferred).
- Hugo (from session 3).
- Home-manager on Ubuntu.
- Per-repo webhooks as the primary trigger.

**Discarded work.** Uncommitted changes on `main` were discarded at your request, since spec v2 replaces that
schema. Their intent, recorded here:
- an `html` method for ready-made sites, which becomes a trivial workflow;
- `default_config` with required defaults (theme, colours), filled into each repo, which becomes workflow `params`
  defaults. The fill had a range-copy bug: it mutated the loop variable, so the defaults never reached the config;
- an `index` repo published to all three domains.

A patch was saved to the session scratchpad only, which is not durable.

**Built this session.**
- `b6cf86c`: **Tooling.** mkdocs and mkdocs-material in the devShell.
- `fa45e4a`: **Docs.** Project `CLAUDE.md`: integration method, working mode, update-the-docs-before-closing, and
  the AI marking convention.
- `cd247e5`, `9c41127`: **Docs.** This site: the overview, usage (the v2 spec, adding a site, workflows, operations), and design (the
  decision record, architecture, open questions, roadmap). `initial-curriculum.md`, `porting-guide.md` and the
  untracked session-3 curriculum are folded into it and removed.
- `7cda625`: **Feature (docs site).** The AI-generated page marker, as a trial. Pages with `generated` front matter turn pale
  yellow, with the generating model, effort and dates drawn large and faded behind the text.
- `5bb8115`: **Docs.** This log.
- In `thecathe/dots`: `47640eb` (merged as `441d40d`), global Claude rules for design write-ups and AI-content
  marking. This branch merged here as `956c8e5`.

**Open questions raised:** see `docs/design/open-questions.md`. Its headings are spec naming, S1–S4, the notes
view, `site.yaml` scope, workflow envs, ddm runtime, infrastructure, team project sites, AI marking beyond this
site, the `cathe.work` sunset, and this site's host.

---

## 2026-10-16 (later): M1, spec v2 + ddm skeleton

Branch `feat/ddm-spec-v2`. **Feature** + **Docs**.

**What was built.**
- `d57e689`: **Feature.** `sites.yaml` at the repo root, the first real v2 spec. It has the two domains, the taxa,
  six workflows (`forester`, `mkdocs`, `odoc`, `ocaml-notebook`, `static`, `site-directory`) and five sites:
  - `forest`, with three views on four hosts;
  - `docs`, the generated directory;
  - `ddm`, this site;
  - `cloakaml` and `indimo`, carried over from the old `deploy.yaml`.
- `e130850`: **Feature.** A new `ddm/` Go module.
  - `internal/spec` parses the spec with unknown keys rejected, fills defaults, and reports **every** problem in one
    go, each with an exact location (`sites.ddm.mounts[0]: host "ddm.cathe.work" is not under any declared domain`).
  - The `ddm check` / `ddm sites` CLI.
  - A table of 18 invalid-spec cases, a cycle test, and a test that keeps the repo's own `sites.yaml` valid.
- **Docs.**
  - The spec page now includes the real `sites.yaml` through a snippet, so docs and file can't drift. It also
    documents every default and validation rule.
  - Operations has the commands that work today.
  - The roadmap marks M1 done, the decision record gains D17/D18, and the open questions are updated.

**Design notes.**
- *Alongside, not renamed (D17).* The plan said to rename `webserver` → `ddm`. But the live droplet may run the
  `webserver` binary as its Caddy: it embeds Caddy and the Cloudflare DNS plugin, which plain nixpkgs Caddy lacks.
  Renaming or deleting it would make this repo's `main` diverge from what the droplet expects, so blue/green now
  applies to the code too. A nice side effect: `ddm/` depends only on `yaml.v3`.
- *`cathe.work` redirected, not deleted (D18).* Dropping it from the live Caddyfile in M1 would break links while the
  domain is still held. The generated Caddyfile will redirect it instead. Redirects are not mounts, so the spec still
  rejects `cathe.work` hosts.
- *One host+path, one owner.* The plan said "duplicate host+path per view", but a URL can only serve one thing
  whatever its view, so uniqueness is global.
- *Loud by default.* `KnownFields(true)` makes a typo'd key an error rather than a silently ignored field, in line
  with your "prefer loud warnings" constraint from the forester work.
- *The shorthand is resolved at load time.* `workflow:` on a site becomes `views.public` immediately, so nothing
  downstream needs to know the shorthand exists.

**Deviations from the approved plan:** D17 and D18 above. Both mean doing less to the live setup, not more.

**Open questions raised:** a few smaller spec points (derive `system.flake`? `params` on `use:` steps? other
forges?), and the redirect targets for `cathe.work`, which need a look at what's on the droplet.

**Addendum: the droplet can be wiped.** Asked whether to merge with D17/D18, you explained that everything on the
droplet is in git or generated from git, so the whole point of ddm is to reset it. That removed the reason for both
deviations, and for blue/green too:
- **D19:** the legacy setup (`webserver/`, `deploy.yaml`, the hand-written `Caddyfile`, the `deploy-dots` Action)
  was deleted in this branch. The old Caddyfile's useful ideas (wildcard DNS-01 certs, a shared 404 page, odoc's
  root rewrite) were carried into roadmap M3. Its filesystem probing is not carried over.
- **D20:** `cathe.work` is simply dropped, with no redirects.
- **D21:** the droplet is reinstalled in place rather than swapped blue/green: same IP, no DNS change, one droplet.
  The cost is downtime, kept short by proving M5/M6 in a VM first.

Along the way we went over why Caddy was ever entangled with the Go code. Wildcard certificates come from Cloudflare
DNS-01, which needs a Caddy plugin, and plugins must be compiled in. `webserver/` was therefore a custom Caddy build
with a `deploy` subcommand bolted on. Under NixOS, `caddy.withPlugins` builds that Caddy declaratively, so ddm and
Caddy can be separate services (D4).

---

## 2026-10-16 (later still): M2, workflow runner

Branch `feat/ddm-workflows`. **Feature** + **Tooling** + **Docs**. Also, merged just before: `d00acf0`, a
**Docs** page of tasks only you can do (`docs/tasks.md`), starting with deleting the old deploy secrets.

**What was built.**
- `45de3f5`: **Tooling.** Workflow devShells in `flake.nix`: `#mkdocs` and `#ocaml-docs`.
- `b41dd7b`: **Refactor.** The spec drops the `lint-hidden` built-in and accepts absolute repo paths.
- `b83ad24`: **Feature.** `internal/gitsync`: the exact-commit checkout.
- `7f8d993`: **Feature.** `internal/workflow`: the up-front resolver and the runner. `mkdocs-config` is the first real
  built-in.
- `fa8c105`: **Feature.** `ddm build` (`--view`, `--src`, `--branch`, `--set`, `--dry-run`), plus the `bin/ddm`
  wrapper.
- `55fd51b`: **Feature.** The workflows in `sites.yaml` now match the tools that exist. `generate_docs.py` moved to
  `scripts/`.
- **Docs.** The workflows page was rewritten as a reference (step semantics, param precedence, envs, what each
  workflow was verified on). Spec, operations, adding-a-site, roadmap and open questions were updated, and the
  decision record gains D22–D25.

**Every workflow was run on a real repo:**
- `indimo` (mkdocs, with `mkdocs.yml` generated);
- `cloakaml` (odoc, about 2 minutes on the first run);
- `forest` (forester, about 10 s);
- `demos` (ocaml-notebook);
- this docs site.

**Spikes.**
- **S1:** forester and mkdocs pass `site://` through untouched, so the scheme stays. odoc is untested.
- **S2:** forest links are root-absolute (`base-url="/"`), so one build serves both `jpears.me` and
  `forest.cathe.dev`, and forests can't sit under a path prefix.
- **S3:** a clean-checkout forest build works in the forest's own devShell. A cold Nix store waits for the M5 VM
  test.

**Found along the way.**
- **D25, the important one.** The public view built from today's `forest.toml` contains full pages for people's
  profiles. That file is your dev config, where `redirect` means "show as external", and hiding those profiles was
  the original reason for redirect-meta. `private`-tagged trees are correctly excluded. The forest's public and notes
  views therefore wait for M4.
- cloakaml's *pushed* flake only has opam; your local checkout has an unpushed version with odoc. Its old Action
  built from the `docs` branch. Hence D23, a `#ocaml-docs` shell in this repo, and `branch: docs` in the spec.
- `build-forest --strict-private` already did what `lint-hidden` was going to do (D22).
- The forest's `url` is still the template's `my-great-forest.net`. Setting it per view joins M4.
- `go -C ddm run` runs with `ddm/` as the working directory, which made every relative path awkward. Hence `bin/ddm`,
  which compiles from source and runs from the repo root.

**Design notes.**
- Resolution happens entirely before execution, so `--dry-run` shows the exact commands, and a typo'd `{{param}}`
  fails before a 2-minute toolchain download rather than after.
- `nix develop` per step re-evaluates the env each time. It's cheap after the first run, so not worth optimising yet.
- The scratch `{{out}}` is per site and view. The result is then *copied* to `.ddm/out/<site>/<view>/<commit>/`, so
  M3's publish step gets an immutable directory named by its commit.

**Open questions raised:**
- Should a site be able to override its workflow's env?
- Should root-absolute workflows be declared `root_only`, so validation rejects path prefixes?
- odoc and `site://`.

---

## 2026-10-16 (wrap-up): forest-public.toml, cathe.dev, shared styling

Branch `docs/session-wrap-up`. **Design** + **Docs**. The only spec change is one new mount.

**Decided.**
- **D26:** the public and notes views will use a hand-made **`forest-public.toml`**. The only people pages it
  exposes are `@jpears` and `@cathe`; every other `Person` redirects as redirect-meta intended. This is the
  smallest fix for D25's people-profile leak, and it puts the `site.yaml` generator (D12) on hold unless the two
  configs drift. Open: how to say "every Person except these two". The lean is removing `\meta{redirect}` from those
  two trees, so the per-taxon rule has nothing to act on.
- **D7 updated:** `cathe.dev` lands on the forest's `/@cathe/`, with content to come. It is now a mount in
  `sites.yaml`.
- **D27:** two motivations behind ddm made explicit:
  - a **consistent look** across every site and generated output;
  - **cross-linking between subdomains**.

  `site://` already covers the linking. The styling has a new open-questions section and a roadmap entry: one
  source of truth for palette, typography and the AI marker, with an adapter per generator (mkdocs `extra_css`, the
  forester base theme, odoc's `odoc.css`), and possibly shared chrome. It is worth designing before or alongside
  M3, while few sites are live.

**For you (added to the to-dos page):** decide what `cathe.dev`/`@cathe` shows, and how `@jpears`/`@cathe` stay
public in `forest-public.toml`.

**Where the next session starts.** M3 (publish, `rewrite-links`, Caddyfile generation, `docs.cathe.dev`) and the
first forest step of M4 (`forest-public.toml`) are both ready to begin. The shared-styling design affects M3's
`theme`/`rewrite-links` post-build steps, so it may be worth discussing first. The open questions page has it.

**Session in review** (as the global rule suggests). Today went from an idea to a spec, a validator and a working
build runner, with every workflow proven on a real repo. What held up:
- separate builds per view;
- `site://` links (S1 confirmed them);
- builds outside Nix derivations;
- caller-wins param resolution.

What changed on contact with reality:
- blue/green became reinstall-in-place once the droplet was known to be disposable;
- `lint-hidden` became the forest's own `--strict-private`;
- the forest's public view turned out to expose people's profiles. That was the most valuable finding: it would
  have shipped silently without a real build.

Worth revisiting: whether hand-kept forest configs stay manageable, and whether styling should shape M3 rather than
follow it.
