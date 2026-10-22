"""Tests for devlog-build.py. Run from the repo root:

    nix develop -c python3 -m unittest discover -s scripts -p 'test_*.py'
"""

import contextlib
import importlib.util
import io
import os
import shutil
import tempfile
import unittest

spec = importlib.util.spec_from_file_location(
    "devlog_build", os.path.join(os.path.dirname(os.path.abspath(__file__)), "devlog-build.py")
)
db = importlib.util.module_from_spec(spec)
spec.loader.exec_module(db)

PAGES = [
    "index.md",
    "CLAUDE.md",
    "dots/a.md",
    "dots/sub/b.md",
    "dots/sub/deeper/c.md",
    "proj/phase/funding.md",
    "proj/phase/open-items.md",
    "proj/index.md",
]


def read(repo, path):
    with open(os.path.join(repo, path)) as f:
        return f.read()


def matches(entry, page):
    return bool(db.compile_glob(entry).match(page))


class Globs(unittest.TestCase):
    def test_a_path_matches_only_itself(self):
        self.assertTrue(matches("proj/phase/funding.md", "proj/phase/funding.md"))
        self.assertFalse(matches("proj/phase/funding.md", "proj/phase/open-items.md"))
        self.assertFalse(matches("proj/phase/funding.md", "x/proj/phase/funding.md"))

    def test_a_star_stays_inside_one_folder(self):
        # fnmatch got this wrong: `dots/*.md` matched dots/sub/b.md as well.
        self.assertTrue(matches("dots/*.md", "dots/a.md"))
        self.assertFalse(matches("dots/*.md", "dots/sub/b.md"))
        self.assertFalse(matches("dots/*", "dots/sub/b.md"))
        self.assertTrue(matches("dots/*/b.md", "dots/sub/b.md"))
        self.assertFalse(matches("dots/*/b.md", "dots/sub/deeper/b.md"))

    def test_a_question_mark_is_one_character_of_one_name(self):
        self.assertTrue(matches("dots/?.md", "dots/a.md"))
        self.assertFalse(matches("dots/?.md", "dots/ab.md"))
        self.assertFalse(matches("dots?a.md", "dots/a.md"))

    def test_a_double_star_crosses_folders(self):
        for page in ("dots/a.md", "dots/sub/b.md", "dots/sub/deeper/c.md"):
            self.assertTrue(matches("dots/**", page))
        self.assertFalse(matches("dots/**", "proj/phase/funding.md"))
        self.assertTrue(matches("**/c.md", "dots/sub/deeper/c.md"))
        self.assertTrue(matches("**/index.md", "index.md"))
        self.assertTrue(matches("dots/**/c.md", "dots/sub/deeper/c.md"))
        self.assertFalse(matches("dots/**/c.md", "dots/sub/b.md"))

    def test_characters_with_a_meaning_in_regexes_are_literal(self):
        self.assertTrue(matches("a+b (1).md", "a+b (1).md"))
        self.assertFalse(matches("a.md", "aXmd"))

    def test_a_leading_slash_or_dot_slash_is_the_repo_root(self):
        self.assertTrue(matches("/proj/phase/funding.md", "proj/phase/funding.md"))
        self.assertTrue(matches("./proj/phase/funding.md", "proj/phase/funding.md"))

    def test_a_folder_is_refused_with_a_hint(self):
        with self.assertRaises(SystemExit) as e:
            db.compile_glob("proj/phase/")
        self.assertIn("proj/phase/**", str(e.exception))

    def test_publish_must_be_a_list(self):
        with self.assertRaises(SystemExit):
            db.select(PAGES, "index.md")
        with self.assertRaises(SystemExit):
            db.select(PAGES, [{"index.md": 1}])

    def test_an_entry_that_matches_nothing_warns_and_selects_nothing(self):
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            chosen = db.select(PAGES, ["proj/phase/funding-and-spend-control", "dots/a.md"])
        self.assertEqual(chosen, {"dots/a.md"})
        self.assertIn("funding-and-spend-control", err.getvalue())


class Build(unittest.TestCase):
    def build(self, publish, files=None):
        repo = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, repo)
        files = files or {}
        files.setdefault("publish.yaml", "publish: " + repr(publish) + "\ndeny: [secret-word]\n")
        for page in PAGES:
            files.setdefault(page, f"# Page {page}\n\nbody\n")
        for path, text in files.items():
            os.makedirs(os.path.dirname(os.path.join(repo, path)) or repo, exist_ok=True)
            with open(os.path.join(repo, path), "w") as f:
                f.write(text)
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            db.build("public", repo, "_site")
        built = []
        for root, _, names in os.walk(os.path.join(repo, "_site")):
            built += [os.path.relpath(os.path.join(root, n), os.path.join(repo, "_site")) for n in names]
        return repo, sorted(built)

    def test_nothing_is_published_by_default(self):
        repo, built = self.build([])
        self.assertEqual(built, ["index.md"])
        self.assertIn("Nothing is published", read(repo, "_site/index.md"))

    def test_one_page_is_published_and_the_home_page_lists_it(self):
        repo, built = self.build(["proj/phase/funding.md"])
        self.assertEqual(built, ["index.md", "proj/phase/funding.md"])
        home = read(repo, "_site/index.md")
        self.assertIn("[Page proj/phase/funding.md](proj/phase/funding.md)", home)
        self.assertIn("## proj / phase", home)
        self.assertNotIn("open-items", home)

    def test_a_published_index_is_not_replaced(self):
        repo, built = self.build(["index.md", "dots/*.md"])
        self.assertEqual(built, ["dots/a.md", "index.md"])
        self.assertEqual(read(repo, "_site/index.md"), "# Page index.md\n\nbody\n")

    def test_claude_md_is_never_published(self):
        _, built = self.build(["**"])
        self.assertNotIn("CLAUDE.md", built)
        self.assertIn("proj/phase/open-items.md", built)

    def test_a_link_to_a_private_page_becomes_its_text(self):
        repo, _ = self.build(
            ["dots/a.md"], {"dots/a.md": "# A\n\n[b](sub/b.md) and [home](../index.md) and [web](https://x.y/z.md)\n"}
        )
        text = read(repo, "_site/dots/a.md")
        self.assertIn("b and [home](../index.md)", text)
        self.assertIn("[web](https://x.y/z.md)", text)

    def test_titles(self):
        self.assertEqual(db.title_of("---\ntitle: From front matter\n---\n# Heading\n", "a/b.md"), "From front matter")
        self.assertEqual(db.title_of("---\ngenerated:\n  by: Claude\n---\n\n# Heading\n", "a/b.md"), "Heading")
        self.assertEqual(db.title_of("no heading", "a/some-page.md"), "some page")
        self.assertEqual(db.title_of("no heading", "a/folder/index.md"), "folder")


if __name__ == "__main__":
    unittest.main()
