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
  const navP = get("nav.json");

  // In ddm's split view, a directory page shows this page in a pane and says
  // hello. Answer with this page's address and title (again whenever they
  // change), so the directory can keep its own address and its "open in its
  // own tab" link current; and drop the pillar links, which the directory's
  // bar already shows. Only to a ddm origin from nav.json: a private page
  // tells no one else where it is.
  let framed = false;
  if (window.parent !== window) {
    addEventListener("message", async (e) => {
      if (framed || e.source !== window.parent || e.data?.ddm !== "hello") return;
      const nav = await navP;
      if (!nav || !nav.mounts.some((m) => m.origin === e.origin)) return;
      framed = true;
      document.getElementById("ddm-bar")?.setAttribute("data-framed", "");
      // A link that leaves ddm's sites leaves the pane too, into the whole
      // window: most sites (GitHub among them) refuse to be framed, and the
      // pane would show only an error.
      const ours = new Set(nav.mounts.map((m) => m.origin));
      document.addEventListener("click", (c) => {
        const a = c.target.closest?.("a[href]");
        if (!a || a.target || !/^https?:$/.test(a.protocol) || ours.has(a.origin)) return;
        a.target = "_top";
      }, true);
      const say = () => e.source.postMessage({ ddm: "here", href: location.href, title: document.title }, e.origin);
      addEventListener("hashchange", say);
      addEventListener("popstate", say);
      say();
    });
  }

  const nav = await navP;
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

  // The switch between this page's private and public versions. On a private
  // page it is always there, greyed out when the page has no public version
  // (no public mount, or this page isn't in the public build: a hidden tree,
  // a person's page). On a public page it only exists in a local preview,
  // for testing: in production a public page never learns private addresses.
  const views = el("span", { class: "views" });
  if (here) {
    const rel = here.path === "/" ? path : path.slice(here.path.length) || "/";
    const other = here.access === "private" ? "public" : "private";
    if (other === "public" || nav.preview) {
      const target = nav.mounts.find((m) => m.site === here.site && m.access === other);
      let ok = !!target;
      if (target && other === "public") {
        const pages = await get(`paths/public/${target.served}.json`);
        const page = rel.endsWith("/") ? rel + "index.html" : rel;
        ok = !pages || pages.includes(page) || pages.includes(page + "/index.html");
      }
      views.append(el("b", { class: here.access }, here.access));
      const label = other + (other === "private" ? " (preview only)" : "");
      if (ok) {
        const url = target.origin + (target.path === "/" ? "" : target.path) + rel + location.search + location.hash;
        views.append(el("a", { href: url, class: other, title: "This page in the " + other + " version" }, label));
      } else {
        views.append(el("span", { class: other + " off", title: "This page has no " + other + " version" }, label));
      }
    }
  }

  const status = el("span", { class: "status" });
  if (here) {
    const st = await get(`status/${here.view}/${here.served}.json`);
    if (st) {
      // In a pool, the page's first path segment names its package, and so
      // its repo (/rocq-mebi/… is mebi_plugin's). A page of no package, such
      // as the pool's index, shows the pool instead of one member.
      const rel = here.path === "/" ? path : path.slice(here.path.length);
      const seg = rel.split("/")[1] || "";
      const m = st.served.startsWith("@")
        ? st.members.find((x) => (x.packages || []).includes(seg))
        : st.members.find((x) => x.commit);
      const parts = [el("span", {}, m ? m.site : here.site)];
      if (!m && st.served.startsWith("@")) {
        parts.push(el("span", { title: "Built together" }, st.members.filter((x) => x.commit).map((x) => x.site).join(" + ")));
      }
      if (m && m.commit) {
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

  const host = el("div", { id: "ddm-bar", style: "display:block" });
  if (framed) host.setAttribute("data-framed", "");
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
      :host([data-framed]) .links { display: none; }
      .views { display: flex; gap: .5rem; align-items: center; }
      .views:not(:empty)::before { content: "view"; color: var(--muted); }
      .views b, .views a, .views span { padding: 0 .45rem; border: 1px solid currentColor; border-radius: 1rem; }
      .views .off { opacity: .4; cursor: not-allowed; }
      .views .public { color: var(--public); } .views .private { color: var(--private); }
      .views b { background: color-mix(in srgb, currentColor 15%, transparent); }
      .status { margin-left: auto; display: flex; gap: .6rem; color: var(--muted); font-family: ui-monospace, monospace; font-size: 12px; }
    `),
    el("div", { class: "bar" }, links, views, status),
  );
  // Before <body>, not in it, so no page's layout (odoc lays its body out as
  // a grid) can move or squeeze the bar.
  document.documentElement.insertBefore(host, document.body);
})();
