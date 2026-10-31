"""Build the site of a forest's kept artifacts (thecathe/forest-artifacts): what the public view may show, and the private view.

SPIKE: not wired into anything that is live. See the devlog, forest/artifacts-publishing.md.

The repo is private and holds every file the forest's deck-pdf kept, laid out as

    <deck>/<view>/<deck>-<date>-<commit>[-dirty][-notes].pdf   the file
    <deck>/<view>/<deck>-<date>-<commit>[-dirty].json          the manifest: where the files came from, and sha256 of each
    <deck>/<view>/<deck>-<date>-<commit>-dirty.patch           the changes to tracked files, when the tree had some

<view> says which build of the forest the file is of ("public" or "private"), a fact fixed when it was made. Whether anyone
may be given it is a decision, made in publish.toml, read from the checkout and never published:

    publish = [      # the files the public view shows; nothing else is (none: an empty site)
      "slides-demo/public/slides-demo-2026-10-31-ab12cd3.pdf",   # a path: that file
      "1IJ0/public/*-ab12cd3.pdf",                               # `*` and `?` stay inside one name
      "1IJ0/**",                                                 # `**` crosses folders
    ]

The public view shows a file only if it is BOTH listed AND eligible. Eligible, from the files themselves and not from anyone's
say-so:
  - it is a regular file (never a link) at <deck>/public/<name>.pdf: built for the public view;
  - it is not the speaker's notes (-notes.pdf) and not from a tree with changes (-dirty);
  - its manifest is beside it, says view "public" and dirty false, and names the file with the sha256 it has now.
A listed file that is not eligible is reported with the reason and left out: it fails safe, to a 404 (a file that was meant to be
public and is not is the mild mistake; the other is not possible by construction, and `check` makes sure of it on what was built).
The private view shows everything but .git, links and publish.toml, at the same paths, so the public view is a subset of it.

Usage:
    artifacts-build.py build public|private <repo> <dest>   # dest: inside the repo
    artifacts-build.py check <repo> <built public site>
"""

import hashlib
import html
import json
import os
import re
import shutil
import sys
import tomllib

PUBLISHABLE = (".pdf",)


def fail(msg):
    sys.exit(f"artifacts-build: {msg}")


def warn(msg):
    print(f"artifacts-build: warning: {msg}", file=sys.stderr)


def load(repo):
    path = os.path.join(repo, "publish.toml")
    if not os.path.exists(path):
        warn(f"no publish.toml in {repo}: nothing is public")
        return {}
    try:
        with open(path, "rb") as f:
            cfg = tomllib.load(f)
    except tomllib.TOMLDecodeError as e:
        fail(f"publish.toml is not valid TOML ({e}); each entry is a quoted string")
    unknown = set(cfg) - {"publish"}
    if unknown:
        fail(f"publish.toml: unknown keys {sorted(unknown)}")
    return cfg


def compile_glob(entry):
    """A publish entry as a regex over a file's path from the repo root (as devlog-build.py: `*` and `?` stay within one name,
    only a `**` segment crosses folders)."""
    entry = re.sub(r"^(\./|/)+", "", entry)
    if not entry or entry.endswith("/"):
        fail(f"publish.toml: {entry!r} names a folder, not files: write {entry}*.pdf or {entry}**")
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


def files(repo):
    """Every file of the repo but .git, as paths from the root (links are not files: they are skipped, and said)."""
    found = []
    for root, dirs, names in os.walk(repo):
        dirs[:] = sorted(d for d in dirs if d != ".git" and not os.path.islink(os.path.join(root, d)))
        for n in sorted(names):
            full = os.path.join(root, n)
            rel = os.path.relpath(full, repo)
            if os.path.islink(full):
                warn(f"{rel} is a link: never served")
            else:
                found.append(rel)
    return found


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def refusal(repo, rel):
    """Why a file may not be shown in the public view, or None if it may (listing it is a separate matter)."""
    parts = rel.split("/")
    name = parts[-1]
    if os.path.islink(os.path.join(repo, rel)):
        return "a link"
    if not name.endswith(PUBLISHABLE):
        return "not a kind of file that is published (only PDFs are)"
    if len(parts) != 3 or parts[1] != "public":
        return "not under <deck>/public/: it was built for the private view, or is not placed as a kept file is"
    stem = name[: -len(".pdf")]
    if stem.endswith("-notes"):
        return "the speaker's notes"
    if "-dirty" in stem:
        return "made from a tree with changes that were not committed"
    base = stem
    manifest_path = os.path.join(repo, parts[0], parts[1], base + ".json")
    if not os.path.isfile(manifest_path) or os.path.islink(manifest_path):
        return "no manifest beside it to say where it came from"
    try:
        manifest = json.load(open(manifest_path))
        view, dirty = manifest.get("view"), manifest.get("forest", {}).get("dirty")
        listed = {f["name"]: f["sha256"] for f in manifest.get("files", [])}
    except (ValueError, KeyError, TypeError, AttributeError):
        return "its manifest cannot be read"
    if view != "public":
        return f"its manifest says it was built for the {view} view"
    if dirty is not False:
        return "its manifest does not say the tree was clean"
    if name not in listed:
        return "its manifest does not name it"
    if listed[name] != sha256(os.path.join(repo, rel)):
        return "it is not the file its manifest names (sha256 differs)"
    return None


def public_files(repo, entries):
    """The files to show in the public view: those a publish entry names that may be shown."""
    if not isinstance(entries, list) or not all(isinstance(e, str) for e in entries):
        fail("publish.toml: `publish` is a list of paths or globs")
    every = files(repo)
    chosen = set()
    for entry in entries:
        rx = compile_glob(entry)
        matched = [p for p in every if rx.match(p) and p != "publish.toml"]
        if not matched:
            warn(f"publish entry {entry!r} matches no file")
        for p in matched:
            why = refusal(repo, p)
            if why is None:
                chosen.add(p)
            elif p.endswith(PUBLISHABLE):  # a manifest or patch swept in by a glob is not worth a line each
                warn(f"{p} is listed but not shown: {why}")
    return sorted(chosen)


def describe(repo, rel):
    """A line for the index: what the file is, from its manifest if it has one."""
    stem = os.path.basename(rel).rsplit(".", 1)[0].removesuffix("-notes")
    manifest = os.path.join(repo, os.path.dirname(rel), stem + ".json")
    try:
        m = json.load(open(manifest))
        return f"{m['date']}, commit {m['forest']['commit'][:7]}, {m['view']} view"
    except (OSError, ValueError, KeyError, TypeError):
        return ""


def index_page(repo, shown, title):
    rows = "\n".join(
        f'<tr><td><a href="{html.escape(p)}">{html.escape(p)}</a></td><td>{html.escape(describe(repo, p))}</td></tr>' for p in shown
    )
    body = f"<table>\n{rows}\n</table>" if shown else "<p>Nothing here yet.</p>"
    return f"<!doctype html>\n<meta charset=utf-8>\n<title>{html.escape(title)}</title>\n<h1>{html.escape(title)}</h1>\n{body}\n"


def copy(repo, dest, rels):
    for rel in rels:
        target = os.path.join(dest, rel)
        os.makedirs(os.path.dirname(target), exist_ok=True)
        shutil.copyfile(os.path.join(repo, rel), target)


def build(view, repo, dest):
    repo, dest = os.path.abspath(repo), os.path.abspath(dest)
    if os.path.commonpath([repo, dest]) != repo or dest == repo:
        fail(f"{dest} must be a folder inside {repo}")
    shutil.rmtree(dest, ignore_errors=True)
    os.makedirs(dest)
    if view == "public":
        shown = public_files(repo, load(repo).get("publish") or [])
    else:
        shown = [p for p in files(repo) if p != "publish.toml"]
    copy(repo, dest, shown)
    with open(os.path.join(dest, "index.html"), "w") as f:
        f.write(index_page(repo, shown, "Artifacts" if view == "private" else "Files"))
    print(f"artifacts-build: {view}: {len(shown)} files")


def check(repo, built):
    """What the public view built holds nothing it may not: every file is an eligible PDF, byte for byte the repo's."""
    repo, built = os.path.abspath(repo), os.path.abspath(built)
    bad = []
    for root, dirs, names in os.walk(built):
        for n in names:
            full = os.path.join(root, n)
            rel = os.path.relpath(full, built)
            if rel == "index.html":
                continue
            why = refusal(repo, rel) or (None if not os.path.islink(full) and sha256(full) == sha256(os.path.join(repo, rel)) else "differs from the repo's file")
            if why:
                bad.append(f"{rel}: {why}")
    if bad:
        fail("the public build holds files it may not:\n  " + "\n  ".join(sorted(bad)))
    print(f"artifacts-build: {built} holds only files that may be public")


if __name__ == "__main__":
    args = sys.argv[1:]
    if len(args) == 4 and args[0] == "build" and args[1] in ("public", "private"):
        build(args[1], args[2], args[3])
    elif len(args) == 3 and args[0] == "check":
        check(args[1], args[2])
    else:
        fail(__doc__.split("Usage:")[1].strip())
