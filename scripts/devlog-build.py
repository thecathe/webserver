"""Prepare a devlog (thecathe/devlog) for mkdocs, and check its public build (D54).

The devlog repo is private and holds the whole record. Its publish.yaml, read
from the checkout at build time and never published, says what the public
view may show:

    publish:      # the files the public view shows; nothing else is
                  # (none at all: a placeholder page is built instead)
      - webserver/ddm-design.md          # a path: that file
      - webserver/*.md                   # `*` and `?` stay inside one folder
      - beam-bug-crawler/**              # `**` crosses folders: everything below
    pseudonyms:   # replaced in the public view, longest first (literal text)
      "Jane Doe": a colleague
    deny:         # the public build fails if any of these is anywhere in it
      - jdoe                         # a plain string: case-insensitive
      - regex: "cf[au]t_[A-Za-z0-9]"  # a Python regex

In a page, a private passage is dropped from the public view, and a public
stand-in (an HTML comment, so hidden everywhere else) takes its place:

    <!-- private -->the whole passage<!-- /private -->
    <!-- public: what the public view says instead -->

A page is private until a publish entry matches it. An entry is a path from
the repo root, with `*` and `?` standing for part of one name and `**` for any
number of folders (`a/*.md` is the pages directly in a/; `a/**` everything
below it). A page's address /a/b/ is the file a/b.md, or a/b/index.md. An entry
that matches nothing is reported, not fatal: it fails safe, to private. CLAUDE.md
files (the rules for whoever writes here) are never published. If index.md is
not published, the public home page is a list of the pages that are.

Usage:
    devlog-build.py build public|private <repo> <dest>   # dest: inside the repo
    devlog-build.py check <repo> <built site>
"""

import os
import re
import shutil
import sys

import yaml

PRIVATE = re.compile(r"<!--\s*private\s*-->(.*?)<!--\s*/private\s*-->", re.S)
STRAY = re.compile(r"<!--\s*/?private\s*-->")
PUBLIC = re.compile(r"<!--\s*public:\s*(.*?)\s*-->", re.S)
LINK = re.compile(r"(!?)\[([^\]]*)\]\(([^)\s]+)\)")
# Where page text can end up: the pages, the search index, the sitemap. Not
# the theme's own scripts and styles, which carry their authors' addresses.
TEXT = (".html", ".md", ".json", ".xml", ".txt", ".svg")


def fail(msg):
    sys.exit(f"devlog-build: {msg}")


def load(repo):
    path = os.path.join(repo, "publish.yaml")
    if not os.path.exists(path):
        fail(f"no publish.yaml in {repo}: nothing can be public without one")
    cfg = yaml.safe_load(open(path)) or {}
    unknown = set(cfg) - {"publish", "pseudonyms", "deny"}
    if unknown:
        fail(f"publish.yaml: unknown keys {sorted(unknown)}")
    return cfg


def compile_glob(entry):
    """A publish entry as a regex over a page's path from the repo root.

    Not fnmatch: there `*` also matches `/`, so `a/*.md` would publish every
    page below a/. Here `*` and `?` stay within one name, and only a `**`
    segment crosses folders."""
    entry = re.sub(r"^(\./|/)+", "", entry)
    if not entry or entry.endswith("/"):
        fail(f"publish.yaml: {entry!r} names a folder, not pages: write {entry}*.md or {entry}**")
    parts = entry.split("/")
    out = []
    for i, part in enumerate(parts):
        last = i == len(parts) - 1
        if part == "**":
            out.append(".+" if last else "(?:[^/]+/)*")
            continue
        out.append("".join({"*": "[^/]*", "?": "[^/]"}.get(c) or re.escape(c) for c in part))
        if not last:
            out.append("/")
    return re.compile("".join(out) + r"\Z")


def select(candidates, entries):
    """The candidate pages some publish entry matches."""
    if not isinstance(entries, list) or not all(isinstance(e, str) for e in entries):
        fail("publish.yaml: `publish` is a list of paths or globs")
    chosen = set()
    for entry in entries:
        matched = {p for p in candidates if compile_glob(entry).match(p)}
        if not matched:
            print(f"devlog-build: warning: publish entry {entry!r} matches no page", file=sys.stderr)
        chosen |= matched
    return chosen


def title_of(text, page):
    """A page's title as mkdocs would give it: front matter, first heading, file name."""
    m = re.match(r"---\n(.*?)\n---\n", text, re.S)
    if m:
        try:
            title = (yaml.safe_load(m.group(1)) or {}).get("title")
        except (yaml.YAMLError, AttributeError):
            title = None
        if title:
            return str(title)
        text = text[m.end():]
    m = re.search(r"^#\s+(.+?)\s*#*\s*$", text, re.M)
    if m:
        return m.group(1)
    name = os.path.splitext(os.path.basename(page))[0]
    return os.path.basename(os.path.dirname(page)) if name == "index" and os.path.dirname(page) else name.replace("-", " ")


def home_page(titles):
    """The public home page when index.md isn't published: the pages that are."""
    folders = {}
    for page in sorted(titles):
        folders.setdefault(os.path.dirname(page), []).append(page)
    lines = ["# Development logs", "",
             "Written with Claude, from a private record: only the pages listed here are public.", ""]
    for folder, members in sorted(folders.items()):
        if folder:
            lines += [f"## {folder.replace('/', ' / ')}", ""]
        for page in members:
            label = titles[page].replace("[", r"\[").replace("]", r"\]")
            lines.append(f"- [{label}]({page})")
        lines.append("")
    return "\n".join(lines)


def pages(repo, dest):
    skip = {".git", ".ddm", os.path.relpath(dest, repo).split(os.sep)[0]}
    for root, dirs, files in os.walk(repo):
        dirs[:] = sorted(d for d in dirs if d not in skip and not d.startswith((".", "_")))
        for f in sorted(files):
            if f.endswith(".md"):
                yield os.path.relpath(os.path.join(root, f), repo)


def redact(text, page, published, cfg):
    text = PRIVATE.sub("", text)
    if STRAY.search(text):
        fail(f"{page}: a <!-- private --> marker without its pair (blocks don't nest)")
    text = PUBLIC.sub(lambda m: m.group(1), text)
    for name in sorted(cfg.get("pseudonyms") or {}, key=len, reverse=True):
        text = text.replace(name, str(cfg["pseudonyms"][name]))

    # A link to a page the public view doesn't have becomes its text.
    def link(m):
        bang, label, target = m.groups()
        if bang or re.match(r"[a-z]+:|#|/", target):
            return m.group(0)
        path = os.path.normpath(os.path.join(os.path.dirname(page), target.split("#")[0]))
        return label if path.endswith(".md") and path not in published else m.group(0)

    return LINK.sub(link, text)


def build(view, repo, dest):
    # The private view shows everything, so it needs no publish.yaml.
    cfg = load(repo) if view == "public" else {}
    dest = os.path.join(repo, dest)
    if os.path.exists(dest):
        shutil.rmtree(dest)
    all_pages = list(pages(repo, dest))
    if view == "private":
        chosen = all_pages
    else:
        # The rules for writers aren't a page of the log, whatever a glob says.
        candidates = [p for p in all_pages if os.path.basename(p) != "CLAUDE.md"]
        chosen = sorted(select(candidates, cfg.get("publish") or []))
    # The public home page exists either way (published, or listed below), so
    # a link to it stays a link.
    published = set(chosen) | {"index.md"}
    titles = {}
    for page in chosen:
        text = open(os.path.join(repo, page)).read()
        if view == "public":
            text = redact(text, page, published, cfg)
            titles[page] = title_of(text, page)
        out = os.path.join(dest, page)
        os.makedirs(os.path.dirname(out), exist_ok=True)
        open(out, "w").write(text)
    # mkdocs builds no home page unless there is an index.md, so write one.
    home = None
    if not chosen:
        home = "# Development logs\n\nNothing is published here yet.\n"
    elif view == "public" and "index.md" not in chosen:
        home = home_page(titles)
    if home:
        os.makedirs(dest, exist_ok=True)
        with open(os.path.join(dest, "index.md"), "w") as f:
            f.write(home)
    print(f"devlog-build: {view} view, {len(chosen)} of {len(all_pages)} pages")
    if view == "public":
        check(repo, dest)


def check(repo, built):
    cfg = load(repo)
    rules = []
    for d in cfg.get("deny") or []:
        if isinstance(d, dict):
            rules.append((d["regex"], re.compile(d["regex"])))
        else:
            rules.append((str(d), re.compile(re.escape(str(d)), re.I)))
    rules.append(("private marker", STRAY))
    leaks = []
    for root, _, files in os.walk(built):
        for f in files:
            if not f.endswith(TEXT):
                continue
            path = os.path.join(root, f)
            text = open(path, errors="replace").read()
            leaks += [f"{os.path.relpath(path, built)}: {name}" for name, r in rules if r.search(text)]
    if leaks:
        fail("denied terms in the public build:\n  " + "\n  ".join(sorted(set(leaks))))
    print(f"devlog-build: no denied terms in {built}")


if __name__ == "__main__":
    args = sys.argv[1:]
    if len(args) == 4 and args[0] == "build" and args[1] in ("public", "private"):
        build(args[1], args[2], args[3])
    elif len(args) == 3 and args[0] == "check":
        check(args[1], args[2])
    else:
        fail(__doc__.split("Usage:")[1].strip())
