"""mkdocs hook: hand a page's `generated` front matter to ddm's AI-generated marker.

A page that says it was written by an AI carries front matter such as

    generated:
      by: Claude
      model: Opus 5.5
      effort: medium
      date: 2026-10-16
      edited: 2026-10-18

This hook turns that into <meta name="ddm:generated" ...> in the page's <head>,
which ddm's ai.js (ddm/internal/aimark) merges over the site's own `generated:`
block in sites.yaml and draws as a sticky yellow header. `generated: false` opts
a page out of a site-wide mark (content="none").

Used by the mkdocs-config builtin in the configs it generates, and listed in
this repo's own mkdocs.yml.
"""

import html


def _tag(g):
    if g is False:
        return '<meta name="ddm:generated" content="none"/>'
    if not isinstance(g, dict):
        return ""
    by = html.escape(str(g.get("by") or "Claude"))
    attrs = "".join(
        f' data-{key}="{html.escape(str(g[key]))}"'
        for key in ("model", "effort", "date", "edited")
        if g.get(key) not in (None, "")
    )
    return f'<meta name="ddm:generated" content="{by}"{attrs}/>'


def on_post_page(output, page, config):
    tag = _tag((page.meta or {}).get("generated"))
    i = output.lower().find("</head>")
    if not tag or i < 0:
        return output
    return output[:i] + tag + output[i:]
