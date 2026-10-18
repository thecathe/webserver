// ddm's split view for its generated directory pages (sites.cathe.dev,
// docs.cathe.dev, the private site map), D52. A plain click on a site's link
// (an <a data-split>) keeps the directory on the left and opens the site in a
// pane on the right. Between them, a divider: a grip to drag (or focus and
// move with the arrow keys), and buttons to close the pane, open its page in
// a tab of its own, and minimize the directory. The open page is kept in this
// page's address (#open=<url>), so reloading or sharing it reopens the split.
//
// Below 800px wide there is no split: a click navigates as usual. Without
// JavaScript, the links are ordinary links.
(() => {
  const root = document.documentElement;
  const wide = matchMedia("(min-width: 800px)");
  const MIN = 15, MAX = 85, DEFAULT = 100 / 3;
  const clamp = (w) => Math.min(MAX, Math.max(MIN, w));
  const stored = (() => { try { return Number(localStorage.getItem("ddm-split-width")); } catch { return 0; } })();
  let width = stored ? clamp(stored) : DEFAULT;
  let minimized = false;
  let current = null; // the pane's page, as it last reported itself

  // Only ddm's own pages open in the pane: the origins this page links to,
  // and every origin the toolbar's nav.json knows. So a crafted #open= can't
  // frame someone else's page inside the directory.
  const origins = new Set([location.origin]);
  for (const a of document.querySelectorAll("a[data-split]")) origins.add(a.origin);
  const known = fetch("/_ddm/nav.json", { cache: "no-cache" })
    .then((r) => (r.ok ? r.json() : null))
    .then((nav) => { for (const m of nav?.mounts || []) origins.add(m.origin); })
    .catch(() => {});
  const allowed = (url) => {
    try {
      const u = new URL(url, location.href);
      return /^https?:$/.test(u.protocol) && origins.has(u.origin);
    } catch {
      return false;
    }
  };

  const el = (tag, attrs = {}, ...kids) => {
    const e = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
    e.append(...kids);
    return e;
  };
  const button = (cls, label, text) =>
    el("button", { type: "button", class: cls, title: label, "aria-label": label }, text);
  const close = button("close", "Close the page", "✕");
  const tab = el("a", { class: "tab", target: "_blank", rel: "noopener", title: "Open the page in its own tab", "aria-label": "Open the page in its own tab" }, "↗");
  const min = button("min", "Minimize the directory", "◀");
  const grip = el("div", {
    class: "grip", role: "separator", tabindex: "0", "aria-orientation": "vertical",
    "aria-label": "Directory width", "aria-valuemin": MIN, "aria-valuemax": MAX,
    title: "Drag to resize; double-click for a third",
  });
  // The page in the pane says where it is (ddm's toolbar does this, in
  // bar.js), so the address and the ↗ link follow it as the reader clicks
  // around. Pages without the toolbar just don't report.
  const newFrame = () => {
    const f = el("iframe", { title: "Opened page" });
    f.addEventListener("load", () => f.contentWindow?.postMessage({ ddm: "hello" }, "*"));
    return f;
  };
  let frame = newFrame();
  const pane = el("div", { id: "ddm-split" }, el("div", { class: "divider" }, el("div", { class: "controls" }, close, tab, min), grip), frame);
  root.append(pane); // beside <body>, which becomes the left pane

  // The toolbar, if there is one, stays across the top of both panes.
  const bar = new ResizeObserver(() => {
    const b = document.getElementById("ddm-bar");
    root.style.setProperty("--top", (b ? b.getBoundingClientRect().height : 0) + "px");
  });
  const watchBar = () => { const b = document.getElementById("ddm-bar"); if (b) bar.observe(b); };
  new MutationObserver(watchBar).observe(root, { childList: true });
  watchBar();

  const layout = () => {
    root.style.setProperty("--left", minimized ? "0px" : width + "%");
    root.classList.toggle("min", minimized);
    min.textContent = minimized ? "▶" : "◀";
    const label = minimized ? "Show the directory" : "Minimize the directory";
    min.title = label;
    min.setAttribute("aria-label", label);
    grip.setAttribute("aria-valuenow", Math.round(minimized ? 0 : width));
    grip.setAttribute("aria-valuetext", minimized ? "minimized" : Math.round(width) + "% of the window");
  };
  const save = () => { try { localStorage.setItem("ddm-split-width", String(width)); } catch {} };

  const hashURL = () => {
    if (!location.hash.startsWith("#open=")) return null;
    try { return decodeURIComponent(location.hash.slice(6)); } catch { return null; }
  };
  const record = (url, push) => {
    const at = url ? "#open=" + url : location.pathname + location.search;
    if (hashURL() !== url) history[push ? "pushState" : "replaceState"](null, "", at);
  };

  const show = (url, push) => {
    if (!root.classList.contains("split")) {
      // Keep the directory where it was scrolled to, now inside its pane.
      const y = scrollY;
      root.classList.add("split");
      document.body.scrollTop = y;
    }
    if (url !== current) frame.src = url;
    current = url;
    tab.href = url;
    layout();
    record(url, push);
  };
  const hide = (push) => {
    if (root.classList.contains("split")) {
      const y = document.body.scrollTop;
      root.classList.remove("split", "min");
      scrollTo(0, y);
    }
    minimized = false;
    current = null;
    // A fresh frame, not an emptied one: emptying it would be a navigation,
    // a history entry that Back would return to.
    const f = newFrame();
    frame.replaceWith(f);
    frame = f;
    record(null, push);
  };

  document.addEventListener("click", (e) => {
    const a = e.target.closest?.("a[data-split]");
    if (!a || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    if (!wide.matches || !allowed(a.href)) return;
    e.preventDefault();
    show(a.href, true);
  });

  close.addEventListener("click", () => hide(true));
  min.addEventListener("click", () => { minimized = !minimized; layout(); });

  addEventListener("message", (e) => {
    if (current === null || e.source !== frame.contentWindow || e.data?.ddm !== "here" || !allowed(e.origin)) return;
    if (typeof e.data.href !== "string" || !allowed(e.data.href)) return;
    current = e.data.href;
    tab.href = current;
    if (typeof e.data.title === "string") frame.title = e.data.title || "Opened page";
    record(current, false);
  });

  addEventListener("popstate", () => {
    const url = hashURL();
    if (url && allowed(url)) show(url, false);
    else hide(false);
  });

  // Dragging the grip. The iframe would swallow the pointer, so it stops
  // taking events while a drag is on (the "dragging" class).
  grip.addEventListener("pointerdown", (e) => {
    if (e.button !== 0) return;
    e.preventDefault();
    grip.setPointerCapture(e.pointerId);
    root.classList.add("dragging");
  });
  grip.addEventListener("pointermove", (e) => {
    if (!grip.hasPointerCapture(e.pointerId)) return;
    minimized = false;
    width = clamp((e.clientX / innerWidth) * 100);
    layout();
  });
  const drop = (e) => {
    if (!grip.hasPointerCapture(e.pointerId)) return;
    grip.releasePointerCapture(e.pointerId);
    root.classList.remove("dragging");
    save();
  };
  grip.addEventListener("pointerup", drop);
  grip.addEventListener("pointercancel", drop);
  grip.addEventListener("dblclick", () => { minimized = false; width = DEFAULT; layout(); save(); });
  grip.addEventListener("keydown", (e) => {
    const step = e.shiftKey ? 10 : 2;
    const to = { ArrowLeft: width - step, ArrowRight: width + step, Home: MIN, End: MAX }[e.key];
    if (e.key === "Enter" || e.key === " ") {
      minimized = !minimized;
    } else if (to !== undefined) {
      minimized = false;
      width = clamp(to);
      save();
    } else {
      return;
    }
    e.preventDefault();
    layout();
  });

  layout();
  // Opened with #open=… (a reload, or a shared link): reopen the split, or
  // on a narrow screen go straight to the page.
  const start = hashURL();
  if (start) {
    known.then(() => {
      if (!allowed(start)) return;
      if (wide.matches) show(start, false);
      else location.replace(start);
    });
  }
})();
