"""Prepare a devlog (thecathe/devlog) for mkdocs, and check its public build (D54).

The devlog repo is private and holds the whole record. Its publish.yaml, read
from the checkout at build time and never published, says what the public
view may show:

    publish:      # the files the public view shows (globs); nothing else is
                  # (none at all: a placeholder page is built instead)
      - index.md
      - webserver/*.md
    pseudonyms:   # replaced in the public view, longest first (literal text)
      "Jane Doe": a colleague
    deny:         # the public build fails if any of these is anywhere in it
      - jdoe                         # a plain string: case-insensitive
      - regex: "cf[au]t_[A-Za-z0-9]"  # a Python regex

In a page, a private passage is dropped from the public view, and a public
stand-in (an HTML comment, so hidden everywhere else) takes its place:

    <!-- private -->the whole passage<!-- /private -->
    <!-- public: what the public view says instead -->

Usage:
    devlog-build.py build public|private <repo> <dest>   # dest: inside the repo
    devlog-build.py check <repo> <built site>
"""

import fnmatch
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
    globs = cfg.get("publish") or []
    published = {p for p in all_pages if any(fnmatch.fnmatch(p, g) for g in globs)}
    chosen = all_pages if view == "private" else sorted(published)
    for page in chosen:
        text = open(os.path.join(repo, page)).read()
        if view == "public":
            text = redact(text, page, published, cfg)
        out = os.path.join(dest, page)
        os.makedirs(os.path.dirname(out), exist_ok=True)
        open(out, "w").write(text)
    if not chosen:
        # mkdocs given no pages builds no home page; say so instead.
        os.makedirs(dest, exist_ok=True)
        with open(os.path.join(dest, "index.md"), "w") as f:
            f.write("# Development logs\n\nNothing is published here yet.\n")
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
