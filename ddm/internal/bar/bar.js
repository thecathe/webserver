// ddm's toolbar: pillar links, links to this page in the site's other views,
// and a status line saying what this page was built from. Added to every
// published page by ddm (see ddm/internal/bar). Drawn in a shadow root so no
// site's CSS can reach it. The look is a placeholder until the shared-styling
// design.
(async () => {
  const get = async (path) => {
    try {
      const r = await fetch("/_ddm/" + path, { cache: "no-cache" });
      return r.ok ? await r.json() : null;
    } catch {
      return null;
    }
  };
  const nav = await get("nav.json");
  if (!nav || document.getElementById("ddm-bar")) return;

  // Which mount is this page under? The longest path prefix on this origin.
  const path = location.pathname;
  const under = (m) => m.path === "/" || path === m.path || path.startsWith(m.path + "/");
  const here = nav.mounts
    .filter((m) => m.origin === location.origin && under(m))
    .sort((a, b) => b.path.length - a.path.length)[0];

  const el = (tag, attrs = {}, ...kids) => {
    const e = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
    for (const k of kids) e.append(k);
    return e;
  };

  const links = el("nav", { class: "links" });
  for (const l of nav.links) {
    const a = el("a", { href: l.url }, l.title);
    if (new URL(l.url, location.href).origin === location.origin) a.className = "current";
    links.append(a);
  }

  // The same page in the site's other views: private ↔ public ↔ notes.
  const views = el("span", { class: "views" });
  if (here) {
    const rel = here.path === "/" ? path : path.slice(here.path.length) || "/";
    const seen = new Set([here.view]);
    const alts = nav.mounts.filter((m) => m.site === here.site && !seen.has(m.view) && seen.add(m.view));
    if (alts.length) {
      views.append(el("b", { class: here.access }, here.view));
      for (const m of alts) {
        const url = m.origin + (m.path === "/" ? "" : m.path) + rel + location.search + location.hash;
        views.append(el("a", { href: url, class: m.access, title: "This page in the " + m.view + " view" }, m.view));
      }
    }
  }

  const status = el("span", { class: "status" });
  if (here) {
    const st = await get(`status/${here.view}/${here.served}.json`);
    if (st) {
      const m = st.members.find((x) => x.site === here.site && x.commit) || st.members.find((x) => x.commit);
      const parts = [el("span", {}, here.site)];
      if (m) {
        const repo = /^[\w.-]+\/[\w.-]+$/.test(m.repo)
          ? el("a", { href: `https://github.com/${m.repo}/tree/${m.branch}` }, `${m.repo}@${m.branch}`)
          : el("span", {}, `${m.repo}@${m.branch}`);
        const commit = /^[\w.-]+\/[\w.-]+$/.test(m.repo)
          ? el("a", { href: `https://github.com/${m.repo}/commit/${m.commit}` }, m.commit.slice(0, 7))
          : el("span", {}, m.commit.slice(0, 7));
        parts.push(repo, commit);
      }
      if (st.served.startsWith("@")) parts.push(el("span", { title: "Built together with the rest of its pool" }, st.served));
      const when = new Date(st.published);
      parts.push(el("span", { title: when.toISOString() }, "built " + when.toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" })));
      for (const p of parts) status.append(p);
    }
  }

  const host = el("div", { id: "ddm-bar" });
  const root = host.attachShadow({ mode: "open" });
  root.append(
    el("style", {}, `
      :host { all: initial; display: block; }
      .bar { --bg: #ebdbb2; --fg: #3c3836; --muted: #7c6f64; --link: #af3a03; --public: #79740e; --private: #9d0006;
             display: flex; flex-wrap: wrap; gap: .25rem 1.25rem; align-items: center; padding: .3rem .8rem;
             background: var(--bg); color: var(--fg); font: 13px/1.4 system-ui, sans-serif;
             border-bottom: 1px solid var(--muted); }
      @media (prefers-color-scheme: dark) {
        .bar { --bg: #282828; --fg: #ebdbb2; --muted: #a89984; --link: #fe8019; --public: #b8bb26; --private: #fb4934; }
      }
      a { color: var(--link); text-decoration: none; } a:hover { text-decoration: underline; }
      .links { display: flex; flex-wrap: wrap; gap: .9rem; font-weight: 600; }
      .links a.current { color: var(--fg); }
      .views { display: flex; gap: .5rem; align-items: center; }
      .views:not(:empty)::before { content: "view"; color: var(--muted); }
      .views b, .views a { padding: 0 .45rem; border: 1px solid currentColor; border-radius: 1rem; }
      .views .public { color: var(--public); } .views .private { color: var(--private); }
      .views b { background: color-mix(in srgb, currentColor 15%, transparent); }
      .status { margin-left: auto; display: flex; gap: .6rem; color: var(--muted); font-family: ui-monospace, monospace; font-size: 12px; }
    `),
    el("div", { class: "bar" }, links, views, status),
  );
  document.body.prepend(host);
})();
