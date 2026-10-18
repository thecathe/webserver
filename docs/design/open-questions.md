---
generated:
  by: Claude
  model: Opus 5.5
  effort: medium
  date: 2026-10-16
  edited: 2026-10-18
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
- **Answered (M3a):** forest redirect trees work. `\meta{redirect}{[text](site://<name>/)}` resolves in both the
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

**Decided 2026-10-17 (D38):** no notes view. `notes.cathe.dev` is the public forest's `notes` tag page, and a
`private` tag still hides a tree everywhere public. The keep-only-tagged fork filter is dropped.

## Forest configs: `forest-public.toml` now, `site.yaml` later?

- **Decided (D26):** start with a hand-made `forest-public.toml` next to `forest.toml`.
- **Answered (D39, D42):** following redirects keeps `@jpears` and `@cathe`; and unpublished taxa (a fork
  addition) keep every other person off the public forest, redirect or not, with `publish = ["@jpears"]` as the
  exception list.
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

## Bots: `robots.txt` and friends

- **Question (2026-10-18):** what may crawlers do with the public sites, now that far more is hosted? Each public
  host needs a `robots.txt`. Separate groups can cover search engines, AI-training crawlers (GPTBot, ClaudeBot,
  Google-Extended, CCBot, …) and assistants fetching a page for a user (ChatGPT-User, Claude-User, …). Cloudflare's
  Content Signals (`Content-Signal: search=yes, ai-train=no`) state the same policy in one line.
- **Policy decided and built (D50).** Kept in the spec, generated per host, served by Caddy at `/robots.txt`, so
  no site repo carries one. Left: `indexme` routes, which need the forest's build output (written at publish).
- **Answered (2026-10-18):** search engines get only the entry pages, for now. "Documentation" means generated
  program docs (`cloakaml`, `ocaml`); not `ddm` or `demos`. The CV is `cv.tree`, at `/cv/` (all in D50).
- **Answered (2026-10-18):** the CV's canonical host is `cv.jpears.me`. Search engines may index the program docs.
  `indexme` comes from `forest.json` after each build (D50).
- **Answered (2026-10-18):** the public hosts stay **proxied** through Cloudflare (orange cloud), and only
  Cloudflare's addresses reach the droplet's web ports (D53). The private domain must not be proxied:
  `test.the.cathe.dev` currently resolves through Cloudflare to the public address, where it is refused, which is
  safe but unusable.
- **Limits:** robots.txt is a request, not enforcement. Enforcement through Cloudflare's proxy is future work
  (roadmap, "Later").
- **Stake:** must be in place before going live (D49).

## ddm runtime details

- **Poll interval:** 10m? Per-site overrides?
- **Notifications:** these replace the dead `email_notifications` field. Email, ntfy, or a status page? The lean is
  a status page first, most naturally the private site map (`the.cathe.dev/sites/`, D29), extended with each
  mount's live commit and last build from `state.json` (D48).
  - **Email** (you asked, 2026-10-18), for at least: a build failing, the server building more than half the day
    (D48 already measures this), and a failed system rebuild. It needs an SMTP account (e.g. a Fastmail or Gmail
    app password, or a sending service), stored with agenix. On NixOS that's `msmtp`, triggered by systemd
    `OnFailure=` and by ddm itself. The lean is to add it in M6, with one digest a day rather than a mail per event.
- **Health check:** for rollback, what does "the new generation is healthy" mean? `caddy validate`, ddm answering on
  its socket, and a fetch of each mount's entry?
- **Retention:** how many builds to keep per site?
- **Rebuild hygiene:** cleaning stale `/var/www` roots left by sites removed from the spec.
- **The OCaml pool holds up everything else** (seen 2026-10-18). Any change to `sites.yaml` or to ddm rebuilds the
  pool ("the spec or ddm changed"): 12 to 30 minutes. The pool builds first (`@odoc-pool` sorts first), so every
  other site in that run waits, which is why `devlog` took so long to appear. And a system rebuild during a deploy
  restarts `ddm-deploy` and kills the run, so the pool starts over. Candidate fixes, smallest first:
  - don't restart a running deploy on a system switch (`restartIfChanged = false` on `ddm-deploy`); the next run
    uses the new ddm anyway;
  - build pools last, after the quick sites;
  - narrow the pool's trigger to its own part of the spec (its members, workflow and scripts), not the whole file;
  - find out why mebi_plugin's test files are compiled for its docs, and skip them if they aren't needed.

## Infrastructure

- **Droplet:** reinstalled in place (D21) with nixos-anywhere (D44). Still open: is the current droplet size enough
  for forester and odoc builds? nixos-anywhere's kexec step also needs about 1 GiB of RAM.
- **A trial install first?** No: you chose to install in place on the existing droplet (2026-10-18), resized to
  2 GB RAM for the install (CPU and RAM only, so it can be resized back).
- **Secrets:** agenix (D46).
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

**Decided 2026-10-17 (D43)** after the styling session and its
[style lab](https://claude.ai/artifact/Uw23oYRagRwf6ahtirxgcA):
- Gruvbox, with Source Serif 4, Source Sans 3 and Source Code Pro, self-hosted;
- a light/dark toggle in the toolbar, shared across `*.cathe.dev`;
- tokens, adapters and fonts linked from `/_ddm/` at publish;
- the forest via an override sheet first.

Still open, for M3b:
- **How the toggle reaches each generator.** mkdocs-material and odoc both follow `prefers-color-scheme`, while the
  toggle sets an explicit choice. Each adapter must honour a `data-theme` attribute (or class) that the toolbar sets
  on `<html>`. Material keeps its own palette toggle in sync via `data-md-color-scheme`.
- **The forest's dark mode.** The forest has none today, so its override sheet supplies it.

## Split view (D52)

- **Framing protection.** No host sends `frame-ancestors` (or `X-Frame-Options`), which is what lets the split
  view frame every site. It also lets any website frame them. For the static public sites that hardly matters. For
  the private host it is a small clickjacking surface: a page you visit could frame `the.cathe.dev` inside your
  tailnet. A `Content-Security-Policy: frame-ancestors` naming ddm's own hosts would close it. But a local preview
  serves under different names, so the header would have to follow the target, as `site://` links do.
- **The light/dark toggle** (D43, when built) should reach a page in the pane too: the toolbar's hello is the
  natural place to pass the choice along.
- **More pages with the split.** The forest's toolbar pages, or the toolbar's own pillar links, could open in a
  pane the same way. That waits until it proves itself on the directories.

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
