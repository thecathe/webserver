---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-17
---

# Open questions

The agenda for future design sessions. Each question has a **current lean** (what we'd do without further thought),
**what decides it**, and **what's at stake**, so a fresh session can pick any of them up cold. Leans are not
decisions: the landscape is expected to shift once real builds run, so revisit freely. When a question is settled,
move it to the [decision record](record.md) and note it in the [development log](../development.md).

## Spec naming

- **Question:** are `views`, `mounts`, `entry`, `access`, `taxa` and `site://` the right words and shapes?
- **Status (M1):** the first real `sites.yaml` was written with these names, and nothing felt wrong. They are now
  in code (`ddm/internal/spec`), but nothing depends on them outside this repo yet.
- **Lean:** keep them. Revisit if building M2/M3 makes any of them awkward.
- **Smaller points:**
  - Should `system.flake` be derived from the repo this spec lives in, rather than stated?
  - Should `params` be allowed on `use:` steps, to tweak an inlined workflow?
  - Should `repo` accept other forges, or stay GitHub-only?
- **Stake:** rising: renames get more expensive once the daemon and the Caddy generator read these names.

## `site://` survives every generator (spike S1)

- **Answered (M2):** forester and mkdocs (strict mode) both emit `site://cloakaml/…` verbatim, so the scheme stays.
- **Still open:** odoc docstring links (`{{:site://…}text}`) are untested. Test when a project first needs one.
- **Answered (M3a):** forest redirect trees work. `\meta{redirect}{[text](site://name/)}` resolves in both the
  `redirect` and `external` configs. The bare form only works with `redirect`; see
  [links between sites](../usage/spec.md#links-between-sites).

## One forest build on two hosts (spike S2)

- **Answered (M2):** links are root-absolute, so one build serves both hosts unchanged, and `/` → entry can be a
  plain redirect.
- **Follow-ups:**
  - The forest's `url` is still the template's `https://www.my-great-forest.net/`, used in `uri` attributes
    and some broken-link warnings. Set it per view in M4; the lean is `https://forest.cathe.dev/` for public.
  - Root-absolute sites can't be mounted under a path prefix. **Answered (M3a, D33):** workflows declare
    `root_only: true`, and validation rejects a `path:` on their mounts.

## The forest notes view

- **Question:** a fork addition ("keep only trees with any of these tags") or a script-level filter?
- **Lean:** the fork addition, small and tested next to `Test_redirect_meta.ml`. A script filter can't remove trees
  from forester's own indexes.
- **Also open:**
  - What happens to a notes tree that links to a non-notes tree? It should probably be a `--strict-private`-style
    failure, or a link to the public copy via `site://`.
  - Is the home tree the right exemption?

## Forest configs: `forest-public.toml` now, `site.yaml` later?

- **Decided (D26):** start with a hand-made `forest-public.toml` next to `forest.toml`.
- **Open: expressing "only these people".** Today the meta rules are per key and per taxon
  (`[forest.meta.redirect.Person]`), so the public config can redirect *every* `Person`. How do `@jpears` and
  `@cathe` stay visible? Options:
  - drop `\meta{redirect}` from those two trees, so the rule has nothing to act on;
  - a tag (e.g. `public-person`) with its own behaviour;
  - a small fork addition for per-address exceptions.

  The lean is the first, since it needs no code. Check what the redirect target on those trees is for before
  removing it.
- **Later question:** whether to generate the TOMLs from a forest `site.yaml` (D12). Only worth it if the two hand-kept
  files drift. If so, the yaml holds views and the content model, and every TOML (dev included) is generated.
- **Stake:** editor tooling (forest-keeper, vscode-forester) must keep finding a config. Check what it reads before
  moving anything.

## Workflow environments (spike S3)

- **Answered (M2):** the forest builds from a clean checkout in its own devShell. Repos without a usable devShell
  build in one of this repo's (`#mkdocs`, `#ocaml-docs`); cloakaml's pushed flake only has opam.
- **Open:** a site can't yet choose its env; it comes with the workflow. Should sites be able to override `env`, e.g.
  an odoc project with unusual libraries? The lean is yes, as a site-level `env:`, once a second OCaml project
  needs it. Until then, add libraries to `#ocaml-docs`.
- **Also open:** the first build of a new devShell is slow. Should ddm pre-build envs during the system loop?
- **Also open:** the mkdocs-material team warns that MkDocs 2.0 will break plugins and theme overrides, with no
  migration path. nixpkgs pins MkDocs 1.6, so nothing breaks yet, but the `mkdocs` workflow and this site's
  `overrides/` are exposed. Options: stay pinned, or move to a successor (e.g. Zensical) or the planned `md-ssg`.

## ddm runtime details

- **Poll interval:** 10m? Per-site overrides?
- **Notifications:** these replace the dead `email_notifications` field. Email, ntfy, or a status page? The lean is
  a status page first, most naturally the private site map (`the.cathe.dev/sites/`, D29), extended with each
  mount's live commit and last build once the daemon keeps state (M6).
- **Health check:** for rollback, what does "the new generation is healthy" mean? `caddy validate`, ddm answering on
  its socket, and a fetch of each mount's entry?
- **Retention:** how many builds to keep per site?
- **Rebuild hygiene:** cleaning stale `/var/www` roots left by sites removed from the spec.

## Infrastructure

- **Droplet:** reinstalled in place (D21). Open: nixos-anywhere (kexec into an installer, partition with disko,
  fully declarative) or DigitalOcean rebuild + nixos-infect (simpler, less declarative disk layout). The lean is
  nixos-anywhere. Also: is the current droplet size enough for forester and odoc builds?
- **Secrets:** agenix or sops-nix. The lean is agenix, which is simpler; the droplet's host key decrypts.
- **Repo name** after the transfer (`server-dots` is a placeholder).
- **DNS:** a wildcard `*.cathe.dev`, `cathe.dev` and `jpears.me` to the public IP, plus `the.cathe.dev` and
  `*.the.cathe.dev` to the tailnet IP (D29). Wildcard certificates for both come from the same Cloudflare token.
- **Remote access:** should `the.cathe.dev`'s tailnet IP be in public DNS at all, or only in Tailscale's split DNS?

## Private sites in a local preview

- **Question:** `ddm preview` serves private mounts without any check, on localhost only. Is that enough, or should
  a preview skip private views unless asked (`--private`)?
- **Lean:** fine as is: the preview is yours, on your machine. Revisit if previews are ever shared.

## Private-access widget (spike S4)

- **Question:** can a public page `fetch` a tailnet-only host in current browsers, given private-network-access
  rules? Is revealing the hostname in public JS acceptable?
- **Lean:** deferred (decision D10). If S4 is clean, `the.cathe.dev/whoami` behind `forward_auth`, with the widget
  inert unless the answer is your login.

## Team project sites

- **Goal:** project-specific sites that a team can use, hosting material or redirecting to it.
- **Questions:**
  - **Access:** public, shared via Tailscale (node sharing to teammates, with ACLs by login), or something else for
    people outside your tailnet?
  - **Redirect-only sites:** a mount kind like `redirect: https://…` with no repo and no build?
  - **Contribution:** can teammates trigger rebuilds, or only push to the content repo and wait for the poll?
  - **Branding:** per-project theming via workflow `params`?
- **Lean:** public sites and redirect mounts first, then Tailscale sharing for restricted ones.

## Shared styling across sites

- **Goal (D27):** a consistent look across the forest, mkdocs sites, odoc output and anything later, plus
  cross-linking between subdomains.
- **Questions:**
  - **Source of truth:** one CSS file of design tokens (palette, fonts, spacing) plus the AI-marker styles? Where is
    it served from: an `assets` site on its own host, or a path on an existing one?
  - **Delivery:** link to it at runtime (one fetch, always current, but cross-origin) or copy it into each build
    (self-contained and versioned with the build)? The lean is copying, via a `theme` built-in run after every
    build, which also suits `rewrite-links`.
  - **Per generator:**
    - mkdocs: `extra_css` plus Material palette overrides;
    - forester: the base-theme fork's CSS and XSLT;
    - odoc: replace `odoc.support/odoc.css`.

    How far can each go before fighting its generator's own design?
  - **Shared chrome:** a common header or footer linking `cathe.dev`, `jpears.me`, `docs.cathe.dev`?
  - **Light and dark:** all generators should honour `prefers-color-scheme`, with the same token names.
- **Stake:** the more sites go live with their own default looks, the more retrofitting there is. Worth designing
  before or alongside M3.

## OCaml docs: landing pages, a shared index, shared libraries

Raised 2026-10-17, looking at the preview.

- **The landing page.** `dune build @doc` writes a root `index.html` listing the packages **of that dune
  project**: one entry per `.opam` file in the repo, not dependencies. For a one-package repo like cloakaml it is
  always a one-item list.
  - It only grows if a repo defines several opam packages, e.g. if `Showable` and `Thing` were split out as
    packages *inside* the membranes repo.
  - **Done (2026-10-17):** odoc mounts land on the package with `entry: /cloakaml/`, which redirects `/` there. odoc's
    own index stays reachable at `/index.html`. Rejected: publishing only the package subdirectory, because its
    pages link to `../odoc.support/` for styles and search.
- **A collated "OCaml package documentation" page.** This is a generated index, like the directory: every site built
  with the `odoc` workflow (or tagged with a new taxon, e.g. `ocaml`), linking each package's own page.
  - **Lean:** a `site-directory` page filtered by taxon, served as one of the section indexes below (e.g.
    `sites.cathe.dev/ocaml/`). Each entry links with `mount://` straight to the package page.
- **Cross-linking: decided 2026-10-17 (D34), the pool.** Every OCaml docs site uses the pooled `odoc-pool`
  workflow: one dune workspace, one `dune build @doc`, in one opam switch built from the lockfiles. Being in the spec
  is the inclusion rule: a project left out stays separate, and references to it stay plain text, which is
  acceptable. Rejected: vendoring shared libraries into each project; ddm rewriting odoc's unresolved references
  (heuristic). odoc 3 linking to separately built docs stays unverified.
- **Still open:**
  - **The pool on the droplet.** mebi_plugin brings Rocq. It is in the pool now, on your machine, to measure the
    cost (see the development log). If the droplet can't afford it, mebi_plugin's docs would be built here and
    copied over, which ddm doesn't model yet. Options:
    - a site kind whose output is a directory ddm only publishes (it becomes the one push-shaped exception to D32);
    - the droplet pulls a release artifact you publish (still pull).

    Either way, the collated page should still list it.
  - **The collated page when not everything is in the pool.** odoc's root page only lists the pool's own
    packages. If mebi_plugin ever builds outside the pool, `ocaml.cathe.dev` would need a ddm-generated index
    instead, listing packages wherever they are built, and ideally each package's libraries (odoc's package pages
    list them; ddm could read them from the output).
  - **Splitting libraries out.** `Showable`/`Thing` (and some of mebi_plugin's libraries) may become their own
    repos. Each is then just another pool member, and its users depend on it as an opam package as usual. In the
    workspace, dune uses the pool's copy.
  - **Rebuild cost.** Any member's push rebuilds the pool. dune's cache keeps unchanged members fast, and the opam
    switch is reused, but a first build compiles Rocq.

## INDIMO's notes: port to forester, into the main forest?

Raised 2026-10-17. You're tempted to port `.md` notes (INDIMO first) to `.tree` files, and unsure whether that means
separate forests or one.

- **Lean: one forest**, with INDIMO's trees under a folder such as `trees/indimo/`, tagged `indimo`:
  - your `forest.toml` already declares an `indimo` tag page (a build warns that no tree carries it yet);
  - folder rules can tag everything in that folder automatically;
  - the notes view's tag filter (M4) and `private` tagging decide what is public;
  - links between INDIMO notes and the rest of your notes become ordinary forester links rather than `site://`;
  - one theme, so styling is solved once;
  - the build grows, but the forest takes about 10 s today.
- **`indimo.cathe.dev` can stay** as a mount of the forest landing on the tag page (`entry: /indimo/` or wherever the
  tag page lives), retiring the separate mkdocs site.
- **Separate forest: not needed.** Only you edit INDIMO's notes (answered 2026-10-17).
- **Privacy (decided 2026-10-17):**
  - a tree tagged `indimo` is public;
  - a tree tagged `indimo` and `private` stays private.

  This needs no new rule: the forest's existing `private` tag already hides a tree from the public and notes
  views, whatever its other tags.
- **Porting:** mechanical for plain markdown (headings, lists, links, maths), so it can be scripted (e.g. pandoc to
  a rough draft plus hand fixes) in a forest-repo branch. It belongs with the forest work (M4). The mkdocs
  `indimo` site is a useful sample for the styling session until then.
- **Open:**
  - Should INDIMO trees also appear in the notes view (`notes.cathe.dev`)? A folder rule could tag them `notes` as
    well.
  - Should other markdown note repos follow? That is: personal notes live in the forest, and mkdocs is only for
    project documentation.

## Browsers are removing XSLT, which the forest depends on

**Found 2026-10-17, worth acting on soon.** Chromium now shows a banner on forest pages: "This site uses XSLT; that
functionality is being removed from this browser very soon." Every forest page is XML that the browser turns into
HTML with `default.xsl`. Once Chrome drops XSLT, the forest stops rendering there (Firefox still works).

- **Lean:** run the XSLT **at build time**. A step after `build-forest` runs `xsltproc` over every `index.xml`,
  writing the result as the tree's `index.html` (today a redirect stub), so browsers get plain HTML.
  - The XSL files stay the source of the look, and the toolbar is injected into the result as for any HTML.
  - To check: anything the XSLT does that depends on the browser (it uses only XSLT 1.0, so `xsltproc` should
    handle it), and whether `forester.js` (search, KaTeX) works unchanged on the rendered pages.
- **Alternatives:**
  - forester's newer HTML output, if the fork's version has one;
  - a polyfill such as a JS XSLT engine (slow, fragile).
- **Where:** a step in the `forester` workflow (this repo), not a forest change.

## Site navigation: a shared header, and section indexes

Raised 2026-10-17. This belongs to the shared-styling session (D27), which it extends.

- **Status (2026-10-17, D36, D37):** a first toolbar is live in the preview on every site: nav links, view
  switches and a status line. `docs.cathe.dev` → `sites.cathe.dev` is done, and `docs.cathe.dev` is now the index
  of the `docs` taxon. Still open below: the look, section indexes beyond `docs`, and the "up to date" part of the
  status line.
- **Goal:** every public site carries the same header toolbar, linking the central pillars: `cathe.dev`,
  `jpears.me`, and one index page per **section** (docs, projects, demos, presentations, …). Private and personal
  sites are not in it.
- **Directory split.**
  - Today's `docs.cathe.dev` (everything, by taxon) becomes **`sites.cathe.dev`**.
  - `docs.cathe.dev` becomes the index of just the `docs` section. Each other section gets its own index page too.
- **Shape (lean):**
  - **Taxa grow into sections.** `taxa` entries can carry a title and an `index: true`, so the spec says which taxa
    are pillars. `personal` and `notes` stay plain taxa: not in the header, no index.
  - **Index pages** are generated by `site-directory` with a `taxon` param: one page per section.
  - **Where they live:**
    - option A: paths on `sites.cathe.dev` (`/docs/`, `/project/`, …), with `docs.cathe.dev` an alias mount;
    - option B: a subdomain each.

    Option B collides with real sites: `demos.cathe.dev` is already the demos site, so the "demo" section can't
    also be `demos.cathe.dev`. The lean is A, with subdomains only where a section name is free and memorable
    (`docs`).
  - **The header** is generated from the spec (pillars plus section indexes), so adding a section updates every
    site. Links are `site://`/`mount://`, so the preview works too.
- **How it gets into each generator** (the session's main question):
  - mkdocs: a theme override block (`overrides/main.html` already exists here);
  - forester: the base theme's XSLT;
  - odoc: odoc's HTML has no template hook, so either a publish-time injection (insert a `<header>` and a
    stylesheet link after `<body>`, like link rewriting) or leave odoc pages header-less with a link back.

  A publish-time injection works everywhere ddm publishes HTML, but is the most brittle against each generator's
  own CSS.
- **A status line** (raised 2026-10-17), like nvim's or VS Code's, at the edge of the header on generated sites:
  - the repo (linked), the branch and commit it was built from, and when;
  - if cheap, whether that is still the branch's latest commit.

  The first part is cheap: publishing already knows all of it (pools write it to `ddm-build.json`), so ddm can write
  a small status file next to each publish for the header to show.

  "Up to date" is live state: the branch may move after publishing. The cheap version: ddm's poll loop (M6) already
  checks every branch's head, so it can update a status file that the header fetches. Until the daemon exists, the
  status line can only say what was built.
- **Open:**
  - Section names and titles: is it "projects" or "project"? "presentations"?
  - Does `cathe.dev` itself, the forest's `@cathe` page, show the same header?
  - Is the header itself the AI-marker's home on non-mkdocs sites?

## AI marking beyond this site

- **Question:** how the yellow "generated by Claude" treatment looks in forest pages (`\meta{generated}{…}` plus the
  theme XSLT), odoc output and other generators.
- **Lean:** one shared CSS file, served from this site or a common assets mount, with each generator adding the
  class and data attribute.
- **Effort level:** answered: every session so far ran at `medium`, and every page now records it.

## `cathe.work` sunset

Dropped outright (D20): no redirects, and it disappears with the old droplet. Remaining, a personal checklist:
update anything that links to it (CV PDFs, profiles, email signatures, GitHub bios, papers) before it lapses.

## This site's host

- **Question:** is `ddm.cathe.dev` the right name? Or `dev.cathe.dev`, or a path under `docs.cathe.dev`?
