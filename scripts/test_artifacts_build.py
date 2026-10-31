"""Tests for artifacts-build.py (a spike). Run from the repo root:

    nix develop -c python3 -m unittest discover -s scripts -p 'test_*.py'
"""

import contextlib
import hashlib
import importlib.util
import io
import json
import os
import tempfile
import unittest

spec = importlib.util.spec_from_file_location(
    "artifacts_build", os.path.join(os.path.dirname(os.path.abspath(__file__)), "artifacts-build.py")
)
ab = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ab)


class Repo(unittest.TestCase):
    """A repo of kept artifacts in a temporary directory, built as deck-pdf keeps them."""

    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.repo = os.path.join(tmp.name, "repo")
        os.makedirs(self.repo)

    def put(self, rel, data=b"x"):
        path = os.path.join(self.repo, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "wb") as f:
            f.write(data)
        return path

    def keep(self, deck="1IJ0", view="public", commit="ab12cd3", dirty=False, notes=False, text=b"%PDF", manifest_view=None,
             manifest_dirty=None, claimed=None):
        """One kept deck as deck-pdf leaves it; the manifest's claims can be made false."""
        stem = f"{deck}-2026-10-31-{commit}{'-dirty' if dirty else ''}"
        pdf = f"{stem}{'-notes' if notes else ''}.pdf"
        base = f"{deck}/{view}"
        self.put(f"{base}/{pdf}", text)
        manifest = {"group": deck, "view": manifest_view or view, "date": "2026-10-31",
                    "forest": {"commit": commit + "0" * 33, "dirty": dirty if manifest_dirty is None else manifest_dirty},
                    "files": [{"name": pdf, "sha256": claimed or hashlib.sha256(text).hexdigest(), "bytes": len(text)}]}
        self.put(f"{base}/{stem}.json", json.dumps(manifest).encode())
        if dirty:
            self.put(f"{base}/{stem}.patch", b"diff")
        return f"{base}/{pdf}"

    def publish(self, *entries):
        self.put("publish.toml", ("publish = [\n" + "".join(f"  {json.dumps(e)},\n" for e in entries) + "]\n").encode())

    def build(self, view):
        dest = os.path.join(self.repo, "_site")
        err = io.StringIO()
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(err):
            ab.build(view, self.repo, dest)
        self.stderr = err.getvalue()
        return sorted(os.path.relpath(os.path.join(r, n), dest) for r, _, ns in os.walk(dest) for n in ns if n != "index.html")


class Eligibility(Repo):
    def test_a_clean_public_build_listed_is_shown(self):
        pdf = self.keep()
        self.publish(pdf)
        self.assertEqual(self.build("public"), [pdf])

    def test_a_file_that_is_not_listed_is_not_shown_however_public_it_is(self):
        self.keep()
        self.publish("other/**")
        self.assertEqual(self.build("public"), [])

    def test_no_publish_yaml_is_an_empty_public_site_and_says_so(self):
        self.keep()
        self.assertEqual(self.build("public"), [])
        self.assertIn("nothing is public", self.stderr)

    def test_listing_a_private_build_shows_nothing_and_says_why(self):
        pdf = self.keep(view="private")
        self.publish(pdf)
        self.assertEqual(self.build("public"), [])
        self.assertIn("not under <deck>/public/", self.stderr)

    def test_a_private_build_cannot_pass_as_public_by_being_put_in_the_public_folder(self):
        pdf = self.keep(view="public", manifest_view="private")
        self.publish(pdf)
        self.assertEqual(self.build("public"), [])
        self.assertIn("built for the private view", self.stderr)

    def test_the_speakers_notes_are_never_shown(self):
        pdf = self.keep(notes=True)
        self.publish(pdf)
        self.assertEqual(self.build("public"), [])
        self.assertIn("speaker's notes", self.stderr)

    def test_a_tree_with_changes_is_never_shown(self):
        pdf = self.keep(dirty=True)
        self.publish(pdf)
        self.assertEqual(self.build("public"), [])
        self.assertIn("changes that were not committed", self.stderr)

    def test_a_manifest_that_does_not_say_clean_is_not_enough(self):
        pdf = self.keep(manifest_dirty=None)  # says False: fine
        self.assertIsNone(ab.refusal(self.repo, pdf))
        manifest = pdf.rsplit(".", 1)[0] + ".json"
        data = json.load(open(os.path.join(self.repo, manifest)))
        del data["forest"]["dirty"]
        self.put(manifest, json.dumps(data).encode())
        self.assertIn("does not say the tree was clean", ab.refusal(self.repo, pdf))

    def test_a_file_changed_after_it_was_kept_is_not_shown(self):
        pdf = self.keep(text=b"%PDF-1", claimed=hashlib.sha256(b"%PDF-0").hexdigest())
        self.publish(pdf)
        self.assertEqual(self.build("public"), [])
        self.assertIn("sha256 differs", self.stderr)

    def test_no_manifest_no_publication(self):
        pdf = self.keep()
        os.remove(os.path.join(self.repo, pdf.rsplit(".", 1)[0] + ".json"))
        self.publish(pdf)
        self.assertEqual(self.build("public"), [])
        self.assertIn("no manifest", self.stderr)

    def test_only_pdfs_are_published_whatever_a_glob_sweeps_in(self):
        pdf = self.keep()
        self.publish("1IJ0/**")
        self.assertEqual(self.build("public"), [pdf])  # the manifest beside it is not in the public site

    def test_a_link_is_never_served(self):
        pdf = self.keep()
        secret = os.path.join(os.path.dirname(self.repo), "secret.pdf")
        open(secret, "wb").write(b"secret")
        os.symlink(secret, os.path.join(self.repo, "1IJ0", "public", "linked.pdf"))
        self.publish("1IJ0/**")
        self.assertEqual(self.build("public"), [pdf])
        self.assertIn("is a link", self.stderr)
        self.assertEqual(self.build("private").count("1IJ0/public/linked.pdf"), 0)

    def test_a_linked_folder_is_never_walked(self):
        pdf = self.keep()
        outside = os.path.join(os.path.dirname(self.repo), "elsewhere", "public")
        os.makedirs(outside)
        open(os.path.join(outside, "x.pdf"), "wb").write(b"secret")
        os.symlink(os.path.dirname(outside), os.path.join(self.repo, "linked-deck"))
        self.publish("**")
        self.assertEqual(self.build("public"), [pdf])
        self.assertFalse([p for p in self.build("private") if p.startswith("linked-deck")])

    def test_an_entry_cannot_reach_out_of_the_repo(self):
        self.keep()
        self.put("../outside.pdf") if False else None
        self.publish("../../etc/passwd", "/etc/passwd")
        self.assertEqual(self.build("public"), [])

    def test_an_unquoted_entry_is_an_error_that_says_so(self):
        self.keep()
        self.put("publish.toml", b"publish = [ */public/*.pdf ]\n")
        with self.assertRaises(SystemExit) as e:
            self.build("public")
        self.assertIn("each entry is a quoted string", str(e.exception))

    def test_a_folder_entry_is_an_error_not_everything(self):
        self.keep()
        self.publish("1IJ0/public/")
        with self.assertRaises(SystemExit):
            self.build("public")

    def test_star_stays_in_one_folder(self):
        a, b = self.keep(deck="a"), self.keep(deck="b")
        self.publish("*/public/*.pdf")
        self.assertEqual(self.build("public"), [a, b])
        self.publish("a/*.pdf")  # a/public/x.pdf is two folders down
        self.assertEqual(self.build("public"), [])


class TheViews(Repo):
    def test_the_public_site_is_a_subset_of_the_private_one_at_the_same_paths(self):
        pub = self.keep()
        self.keep(view="private")
        self.keep(deck="2XYZ", dirty=True)
        self.publish(pub, "2XYZ/**")
        public, private = self.build("public"), None
        private = self.build("private")
        self.assertTrue(set(public) <= set(private))
        self.assertEqual(public, [pub])

    def test_the_private_site_has_everything_but_git_links_and_the_publish_list(self):
        self.keep()
        self.keep(view="private", dirty=True)
        self.put(".git/config", b"c")
        self.put("README.md", b"r")
        self.publish("1IJ0/**")
        got = self.build("private")
        self.assertIn("README.md", got)
        self.assertTrue(any(p.endswith("-dirty.patch") for p in got))
        self.assertTrue(any(p.endswith(".json") for p in got))
        self.assertFalse([p for p in got if p.startswith(".git") or p == "publish.toml"])

    def test_a_rebuild_leaves_nothing_of_the_last_build(self):
        pdf = self.keep()
        self.publish(pdf)
        self.build("public")
        self.publish()  # nothing listed any more
        self.assertEqual(self.build("public"), [])

    def test_the_index_names_what_is_there_and_escapes_it(self):
        pdf = self.keep(deck="a&b")
        self.publish(pdf)
        self.build("public")
        text = open(os.path.join(self.repo, "_site", "index.html")).read()
        self.assertIn("a&amp;b/public/", text)
        self.assertIn("commit ab12cd3", text)

    def test_dest_must_be_inside_the_repo(self):
        with self.assertRaises(SystemExit):
            ab.build("public", self.repo, os.path.dirname(self.repo))


class Checking(Repo):
    def built(self, view="public"):
        pdf = self.keep()
        self.publish(pdf)
        self.build(view)
        return pdf, os.path.join(self.repo, "_site")

    def test_what_was_built_passes(self):
        _, site = self.built()
        with contextlib.redirect_stdout(io.StringIO()):
            ab.check(self.repo, site)

    def test_a_planted_private_file_is_caught(self):
        _, site = self.built()
        os.makedirs(os.path.join(site, "1IJ0", "private"))
        open(os.path.join(site, "1IJ0", "private", "x.pdf"), "wb").write(b"p")
        with self.assertRaises(SystemExit) as e:
            ab.check(self.repo, site)
        self.assertIn("1IJ0/private/x.pdf", str(e.exception))

    def test_a_manifest_in_the_public_site_is_caught(self):
        pdf, site = self.built()
        manifest = pdf.rsplit(".", 1)[0] + ".json"
        open(os.path.join(site, manifest), "w").write("{}")
        with self.assertRaises(SystemExit):
            ab.check(self.repo, site)

    def test_a_link_in_the_built_site_is_caught(self):
        pdf, site = self.built()
        secret = os.path.join(os.path.dirname(self.repo), "secret.pdf")
        open(secret, "wb").write(b"%PDF")  # even a file with the same bytes
        os.remove(os.path.join(site, pdf))
        os.symlink(secret, os.path.join(site, pdf))
        with self.assertRaises(SystemExit):
            ab.check(self.repo, site)

    def test_a_file_that_differs_from_the_repos_is_caught(self):
        pdf, site = self.built()
        open(os.path.join(site, pdf), "wb").write(b"changed")
        with self.assertRaises(SystemExit) as e:
            ab.check(self.repo, site)
        self.assertIn("differs", str(e.exception))


if __name__ == "__main__":
    unittest.main()
