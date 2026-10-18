---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-17
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

---

## 2026-10-17: M3a, publish, links, Caddy and a local preview

Branch `feat/ddm-publish`. **Feature** + **Design** + **Docs**.

**The plan, reshaped by you.** M3 was "publish, links, Caddy, `docs.cathe.dev`", with the shared-styling design
(D27) possibly first. You chose a middle path (D31): get **one site of every kind** published and viewable side by
side, hold the styling session against that, then do the rest (M3b). Three requests during planning shaped it:
- **"Does ddm build everything itself?"** Yes: every site is pulled and built on the server with one read-only token,
  and no content repo holds a secret. Now stated explicitly as D32, with the push model recorded as rejected.
- **A private site map** listing every site, address and public/private status, so automatic deployments never get
  lost, plus public/private declared in the yaml (D29).
- **Forest trees that redirect to other sites** (`\meta{redirect}{site://cloakaml/}`). This works, in the link form
  `[text](site://…)`; see below.

**Built** (one commit each unless noted):
- `c18d3dc` **spec**:
  - `access: public | private` (was `tailnet-owner`), and a site-level `access` default;
  - `private_domain: the.cathe.dev`: private mounts must be on it, public ones off it;
  - `root_only` workflows (D33), and the `demos` site.
- `beba278` **links**: `site://` resolved at publish time (D28). Same view first, first mount canonical, always
  absolute, never public → private.
- `b3a993f` **publish**: `<www>/<view>/<site>/<time>-<sha>/`, `current` swapped by an atomic rename, keep 3.
- `9251aa2` **site-directory**: the public directory and the private **site map** (`the.cathe.dev/sites/`), both
  with the shared 404 page. Generated pages link with a new `mount://host/path` form.
- `4a56a08` **caddygen**:
  - the Caddyfile from mounts, with one wildcard certificate per domain via Cloudflare DNS-01;
  - entry redirects, 404s, and the private gate (D30);
  - golden tests that real `caddy adapt` must accept;
  - the devShell's Caddy now has the Cloudflare plugin (`packages.caddy`).
- `dde9cc2` **cli**: `ddm publish`, `ddm caddyfile`, `ddm preview`.
- Fixes and docs: `42a8ccd`, `b33617e`, `bedd992`, `e8e0446`, `7f3040c`.

**Verified.**
- All tests pass.
- The real `sites.yaml`'s prod Caddyfile passes `caddy validate` (with a well-formed fake token).
- `ddm preview` publishes every site and view, and curl confirms entry redirects, the 404 page on unknown paths
  and hosts, `/sites` → `/sites/`, and the map's badges (2 private, 9 public mounts).
- No `site://` or `mount://` is left in any link position.

**Found along the way.**
- **The tailnet `bind` can't work** (D30). On Linux, a socket bound to the tailnet address can't listen on `:443`
  while the public blocks' wildcard socket does. A two-socket test confirmed it, so Caddy would have failed at boot
  on the server. The private gate now checks the source address, strips client identity headers, asks the Tailscale
  auth helper, and requires `TAILNET_OWNER` (fail-closed). It was tested against a stub helper: owner 200, others
  403, unset 403, spoofed header 403.
- **Prose isn't a link.** The first rewriter also rewrote this site's `<code>site://…</code>` examples. Links now
  only match right after a quote or `(`.
- **Forest redirect trees.** In the `external` config (the private view), the forest prints a bare meta value as
  text, so `\meta{redirect}{site://cloakaml/}` stayed unrewritten there. The link form `[cloakaml docs](site://cloakaml/)`
  renders as an `href` and works in both configs (tested with a throwaway tree, outside the forest repo).
- `demos` needs `ml_dir: ocaml`, which was known in M2 but missed in the new entry.
- This site builds from GitHub, where `main` isn't pushed yet. Hence `preview --src ddm=.`.

**Decisions:** D28–D33 (record). **Superseded:** `tailnet-owner`; the tailnet `bind`; `rewrite-links` as a
workflow built-in.

**Open questions raised:** whether previews should hide private views; the site map as the future status page.

**For you (to-dos page):**
- look at the preview;
- for M5/M7: your Tailscale login, a Cloudflare token, and DNS for `the.cathe.dev` and `*.the.cathe.dev`.

**Where the next session starts:** the shared-styling design session, with `bin/ddm preview --serve-only` running.

---

## 2026-10-17 (later): odoc landing pages, navigation, and the directory split

Branch `docs/odoc-and-navigation`. **Design** + one spec line.

After looking at the preview you raised three things.
- **cloakaml lands on an almost empty "OCaml package documentation" page.** odoc's root lists the dune project's own
  opam packages, so it has one item unless a repo defines several. The mount now has `entry: /cloakaml/`. That needed
  no code, since `entry` already redirects `/`.
- **A collated OCaml docs page, and splitting `Showable`/`Thing` into a shared package.** The catch is that odoc
  only cross-links packages built together. Options and a lean are in the open questions.
- **A shared header toolbar** linking `cathe.dev`, `jpears.me` and one index per section, and renaming today's
  directory to `sites.cathe.dev`, with `docs.cathe.dev` becoming the docs section's index. The lean shape is: taxa
  grow into sections, `site-directory` gets a `taxon` param, and section indexes are paths on `sites.cathe.dev`
  (`demos.cathe.dev` is already taken by a site). All of this joins the styling session's agenda (roadmap).

Also: you gave the effort level (`medium`), now recorded on pages edited today. The convention is in the record.

---

## 2026-10-17 (later still): an OCaml docs bundle, and INDIMO into the forest

Branch `docs/ocaml-bundle-and-indimo`. **Design** only.

- **Effort:** every session so far ran at `medium` (you confirmed). Every page now records that, and the
  `CLAUDE.md` template says so.
- **Cross-linking OCaml docs across projects.** odoc only links packages built in one run. Separately built docs
  leave references as unlinked text, so there is nothing for ddm to rewrite cleanly. Five options are in the open
  questions. The lean is a **dune-workspace bundle**: each repo as a subdirectory under one `dune-workspace`, which
  is dune's standard multi-project mechanism, not a hack. One `dune build @doc` cross-links everything, and its root
  page doubles as the collated OCaml index. Each project keeps its host as a mount of the one build. Costs: one
  shared environment, and every push rebuilds the bundle. Not committed to; plain text stays acceptable.
- **INDIMO's notes into forester.** The lean is to merge them into the main forest (a folder plus the `indimo` tag,
  which `forest.toml` already declares), with `indimo.cathe.dev` becoming a forest mount. A separate forest only if
  others edit them. Scheduled with the forest work (M4), if you decide to.

**Addendum (same day).** INDIMO: only you edit it, so it merges into the main forest. A tree tagged `indimo` is
public, and `indimo` plus `private` stays private, which the existing `private` tag already gives.

---

## 2026-10-17 (evening): the OCaml docs pool, and new addresses

Branch `feat/ocaml-pool`. **Feature** + **Design** + **Docs**.

**Decided with you** (D34, D35):
- **The pool, as you framed it.** Each OCaml repo stays its own site. A workflow marked `pool: true` builds every
  site that uses it together, in one dune workspace, and serves the one result from all their mounts. Membership is
  the spec: anything left out stays plain text, which is acceptable.
- **Exact versions.** The environment is one opam switch, from the lockfiles where they agree.
- **mebi_plugin joins the pool** while builds run on your machine, to measure the cost.
- **New forest addresses:** `indimo.cathe.dev`, `mebi.cathe.dev` and `projects.cathe.dev`.
- **INDIMO's notes** move to `indimo-notes.cathe.dev` (they are notes, not documentation).
- **A header status line** (repo, branch, commit, freshness) joins the styling session's agenda.

**Built.**
- **Spec:** `pool:` on workflows, and pool members need no mounts. Helpers: `PoolOf`, `PoolMembers`, `Served`,
  `MountsServing`.
- **`build/pool.go`:**
  - syncs every member at its own branch;
  - copies the checkouts (without `.git`/`_build`/`_opam`) into one workspace with a `dune-workspace`;
  - runs once;
  - publishes as `@odoc-pool`, with `ddm-build.json` listing each member's repo, branch and commit (ready for the
    status line).
- **Workflows:** `{{cache}}`, kept between builds, and peak memory logged for every step.
- **Publishing, links, Caddy and the CLI** all go through `Served`. A preview builds a pool once, and the site map
  gains "Without an address of their own".
- **`scripts/opam-pool-switch.sh`** (cached opam root and switch: lockfiles first, else one solve), plus the
  `#ocaml-opam` devShell.

**Measured on this machine** (pool: cloakaml + mebi_plugin):

| | Time | Peak memory (largest process) | Disk |
|---|---|---|---|
| First switch build (Rocq 9.2.0 and everything, from source) | 6 min | 608 MiB | 1.3 GB switch + 211 MB opam root |
| Switch check on later builds | 18 s | 398 MiB | |
| `dune build @doc` for the pool | 59 s | 521 MiB | |

Peak memory is per process (the largest), so parallel jobs can add up to more. The droplet would also need room for
a second switch during upgrades. A rebuild with the switch cached takes about 1 min 20 s end to end.

**Found.**
- **A pool needs one version of every shared tool.** Rocq 9.2.0 requires `dune < 3.24`, and membranes requires
  `dune >= 3.24`. No switch can build both, even ignoring the lockfiles, so membranes is commented out of the pool
  (see your to-dos). cloakaml and mebi_plugin's lockfile agree, so their switch uses exact versions.
- **`--deps-only` applies to every package on the command line**, so naming `odoc` beside the projects installed
  only odoc's dependencies. odoc is now installed separately.
- **dune marks its outputs read-only.** Copies are now owner-writable, so publishing can rewrite links.
- **The forest's Project page lists only Cloakaml.** The `indimo` and `mebi` trees don't carry `\taxon{Project}`
  (your to-dos).

**Verified.**
- Tests pass, including new pool tests (spec, build, caddygen goldens).
- In the preview:
  - `ocaml.cathe.dev` lists `cloakaml` and `rocq-mebi` with all of mebi's libraries;
  - `cloakaml.cathe.dev` → `/cloakaml/`;
  - `indimo`, `mebi` and `projects` redirect into the forest;
  - `indimo-notes` serves the mkdocs notes;
  - the site map lists mebi-plugin as served by the pool.

---

## 2026-10-17 (night): a toolbar, sites.cathe.dev, and odoc's Index link

Branch `feat/nav-bar`. **Feature** + **Docs**.

You asked for three things after browsing the preview; none needed anything from you first.

- **odoc's "Index" on `cloakaml.cathe.dev`** showed the whole pool (including `rocq-mebi`) under cloakaml's
  address. On a pool member's own host, the generated Caddyfile now redirects everything outside `/<package>/` and
  `/odoc.support/` to `ocaml.cathe.dev`, keeping the path, so "Index" and links to other packages open there.
- **A toolbar, now rather than after the styling session** (D36), so you can flip a forest page between its private
  and public views. It is one module script added at publish and drawn in a shadow root before `<body>`. It shows:
  - the spec's new `nav:` links;
  - the same page in the site's other views;
  - a status line (repo@branch, commit, pool, build date) from status files written at publish.

  Public and private audiences get separate data under `/_ddm/`, so private addresses never reach public pages
  (tested).
- **`docs.cathe.dev` → `sites.cathe.dev`**, and a new `docs.cathe.dev` lists the `docs` taxon (D37).
  `site-directory` gained `taxon` and `title` params.

**Commits:** `cedaa24` (taxon pages), `769ad8e` (toolbar, pool host redirects), `9bf2baa` (sites.yaml), `a280f68`
(toolbar fixes), plus docs.

**Verified.**
- Tests pass: new tests cover the bar data's public/private split, the head injection and the taxon page.
- The prod Caddyfile adapts.
- In the preview:
  - cloakaml's `/index.html` and `/rocq-mebi/…` redirect to `ocaml.cathe.dev`;
  - the bar renders on mkdocs, odoc and forest pages (headless Firefox and Chromium screenshots);
  - on `the.cathe.dev/indimo/` it offers `public` and `notes`.

**Found.**
- **Chromium is removing XSLT**, which the forest needs. Its pages now show a removal banner. The lean is rendering
  the XSLT at build time; it is next on the roadmap.
- A classic `defer` script didn't run in XSLT-rendered pages. odoc's grid `<body>` displaced the bar. Both are fixed
  (D36).

**Addendum (later).** The toolbar's status line stuck to cloakaml on every `ocaml.cathe.dev` page, because all of
them belong to the repo-less index site. Pooled builds now record each member's opam packages, and the bar picks
the member from the page's first path segment, or shows the pool on the index (`fix/bar-pool-member`). Also
recorded:
- the XSLT options, plus the finding that your fork's XML output is already called "legacy";
- why the forest's views look alike today (no `forest-public.toml` yet, no notes filter);
- the lean to merge the notes view into public.

---

## 2026-10-17 (late): the public forest, notes and recents, build-time XSLT

Branches: `feat/forest-public` in the **forest** repo, and `feat/forest-public-views` here. **Feature** + **Design**.

**Decided with you:**
- Notes is a page of the public forest, not a view; a `private` tag still overrides (D38).
- `recents.cathe.dev` is the forest's timeline.
- XSLT runs at build time (D40).
- `forest-public.toml` starts now (D39).

**Built.**
- **Forest repo:**
  - `3ff7321` adds a `notes` tag page;
  - `8fbeda3` adds `forest-public.toml`, which is `forest.toml` with the public `url` and redirects followed.

  Merged into the forest's local `main`, not pushed.
- **Here:**
  - `9f6db44`: the public view uses `forest-public.toml`; the notes view is removed; `notes.cathe.dev` → `/tag-notes/`
    and `recents.cathe.dev` → `/timeline/`, both added to the toolbar;
  - `8cfec6f`: the `forest-html` step (`xsltproc` in a new `#xslt` devShell).

**Verified** (preview, forest built with `--src` from the branch):
- public has 135 pages and private 151;
- `/@lbocchi/` is a 404 publicly but a page privately, while `@jpears` is in both;
- INDIMO's public page links people to their own sites and lists only you in "Related", while the private page
  lists everyone (screenshots);
- forest pages are now plain HTML, and Chromium shows no XSLT banner;
- notes and recents redirect to their pages.

**Found.**
- D26's open question had a free answer: `@jpears` has no redirect meta, and `@cathe` is an `Alias`.
- Three people have no redirect meta and so stay public (to-dos).
- The forest has `taxon-notes` pages (`\taxon{Notes}`/`{Note}`) besides the `notes` tag (3 trees). `notes.cathe.dev`
  uses the tag.

---

## 2026-10-17 (later): the toolbar switch, private repos, unpublished people

Branches:
- here: `feat/bar-switch`;
- forest: `feat/people-private`;
- ocaml-forester: `feat/unpublished-taxon`;
- vscode-forester: `feat/unpublished-taxa`.

All merged locally; nothing pushed. **Feature** + **Design**.

**Toolbar (D41).**
- **Private pages** get a fixed `public` button, greyed out when the page has no public version. ddm checks it
  against the public build's page list, which is private-only data.
- **Public pages** get a `private` button only in a local preview; production can't produce it.
- **Status line:** public pages show the repo only for repos GitHub serves publicly (checked anonymously, failing
  closed), so `thecathe/forest` no longer appears on `indimo.cathe.dev`.
- Checked in Chromium screenshots: greyed on `the.cathe.dev/@lbocchi/`, active on `the.cathe.dev/indimo/`, preview
  button and no repo on `forest.cathe.dev/indimo/`.

**People private by default (D42).**
- **First try:** a `hide` tag on `trees/people`. It failed `--strict-private` on every paper and project naming a
  person, because hiding treats links as leaks and also overrides redirects.
- **The fork:** `[forest.taxon.<T>] unpublished = true, publish = [...]`. Precedence is: hide tag, then redirect,
  then unpublished. An unpublished tree is not built and is filtered from backmatter; links to it are plain text, a
  full transclusion of it is empty, and it isn't reported as a leak. 3 commits with tests.
- **`forest_gen.py`** leaves unpublished trees out of the timeline and the taxa index.
- **Forest:** `forest-public.toml` sets it for Person, with `publish = ["@jpears"]`.

  A local build of the forest (both inputs overridden) passes `--strict-private`, with only `@jpears` and `@cathe`
  pages left. Other names appear only as plain-text author lists on papers.
- **Project taxons** on `indimo` and `mebi`.

**For you:** push the three repos and bump the forest's flake lock (to-dos). Until then, the preview's public
forest is built with the old fork.

**Addendum.** You pushed `ocaml-forester`, `vscode-forester` and the forest (with its pins bumped) and confirmed the
public forest behaves correctly from GitHub, without `--src`.

---

## 2026-10-17 (end of day): the styling session opens; the state of things

Branch `docs/styling-session`. **Design** + **Docs**.

**Styling session.**
- **A style lab** ([artifact](https://claude.ai/artifact/Uw23oYRagRwf6ahtirxgcA), private to you) previews three
  palettes (Gruvbox, Nord, Everforest) and three type pairings (Inria, IBM Plex, Source) on real content (a forest
  page with the toolbar, an odoc signature, the AI marker), in light and dark.
- **Survey:** odoc is the easiest generator to theme (105 CSS variables, dark mode built in), mkdocs-material is
  close behind, and the forest is the hardest (hard-coded colours, no dark mode).
- **The delivery plan changed because of the toolbar.** Every host already serves `/_ddm/`, so the shared tokens,
  per-generator adapters and fonts can be linked same-origin and inserted at publish, rather than copied into each
  build. The leans are recorded in the open questions; your choices close the session.

**Docs brought up to date for the next session (M5, in a fresh session).**
- The overview's status box.
- The roadmap:
  - M4's done items (D38, D39, D42), and D25 resolved;
  - a list of what M5 must provide: the Caddy package and env, the tailscaleAuth socket, paths, the opam cache size,
    a token for private repos, DNS.

**Where the next session starts.** M5: `nixosConfigurations.droplet`. Everything ddm does today runs locally
(`bin/ddm preview`). The open to-dos that M5/M7 need from you are the Tailscale login, a Cloudflare token, and DNS.

**Session closed (D43).**
- **Your picks:** Gruvbox, and Source Serif + Sans (with Source Code Pro).
- **Leans accepted as decisions**, with the light/dark toggle confirmed as a widget in the toolbar: self-hosted
  fonts, a toggle shared across `*.cathe.dev`, colours/fonts/links only, the forest via an override sheet first,
  and a toolbar that scrolls away.
- **Implementation is M3b's first item.** Still to work out: making each generator honour the toggle's explicit
  choice, and giving the forest a dark mode.

---

## 2026-10-18: M5, the droplet's NixOS config

Branch `feat/nixos-host`. **Feature** + **Design**.

**Built.**
- **`ddm deploy`**: publishes every site for production. The preview's loop became `publishAll`, shared by both.
- **`nix/`**:
  - `package.nix`: ddm;
  - `modules/ddm.nix`: `services.ddm`;
  - `hosts/droplet/`: the host;
  - `tests/droplet.nix`: the VM test.
- **The flake** gains `disko` and `agenix`, plus `packages.ddm`, `nixosModules.ddm`, `nixosConfigurations.droplet`
  and `checks.droplet`.
- The droplet's system builds, and the VM test passes (7 subtests).

**Decisions.**
- **D44:** nixos-anywhere with disko; root SSH by key only.
- **D45:** the Caddyfile is generated and validated at build time, not written by ddm.
- **D46:** agenix, with the host key generated before the install; a secret is wired in only once its file exists.
- **D47:** `ddm deploy` on a timer until M6; Tailscale joined by hand.

**Found along the way.**
- `caddy validate` rejects an obviously fake Cloudflare token. The build uses 40 `x`s, which pass its format check.
- Caddy's `abort` shows up in curl as exit 92 over HTTP/2 (stream reset) and exit 52 over HTTP/1.1 (empty reply).
  The test forces HTTP/1.1 so a TLS failure can't pass for the gate working.
- `nix develop /nix/store/…-source#mkdocs` works. So workflows find `{{system}}` in the flake's store copy, and
  the server needs no checkout of this repo. Verified with a real `ddm deploy docs sites` from that copy.
- GitHub's `cathe-org/dots` `main` is still at `8ae185a`, so building the `ddm` site from GitHub fails until you push.

**Rejected.**
- ddm writing the Caddyfile at runtime and reloading (D4's plan). See D45.
- An auth key for Tailscale. See D47.
- nixos-infect. See D44.

**Open.**
- Whether DigitalOcean's DHCP gives the droplet its address (checked at install).
- The droplet's size.
- A trial droplet before M7.

**Commits** (this repo): `62517b5` (ddm deploy), `2813c56` (NixOS config), plus this docs commit.
