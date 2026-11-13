package workflow

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// A step with an env runs in `nix develop <env>`. Nothing keeps that
// toolchain alive afterwards, so the weekly nix-gc deleted it (the forest's
// alone is about 5 GB, built from source) and the next deploy rebuilt it. To
// stop that, the step runs with `--profile <cache>/gcroots/<key>`: the profile
// is a GC root whose closure holds the toolchain.
//
// A profile keeps every generation, and an environment that changed (a bumped
// flake input) adds one, so the superseded toolchain would stay rooted for
// ever. After each step the old generations are wiped, which leaves the
// current one rooted and lets the next nix-gc collect the rest.
//
// The roots live in the site's cache directory, so removing a site's cache
// removes its roots too: nix drops a root whose link has gone.

// rootsDir is where a plan's profiles live, under its cache.
const rootsDir = "gcroots"

// rootKey names an env for its profile: what it is, not where it is, because
// the checkout and this repo are at a new path or commit every time.
//
//	<this repo>#mkdocs  →  system-mkdocs
//	<the checkout>      →  repo
//
// Any other flake ref is kept as written, made safe to use as a file name.
func (v Vars) rootKey(env string) string {
	for _, c := range []struct{ path, name string }{{v.System, "system"}, {v.Repo, "repo"}} {
		if rest, ok := strings.CutPrefix(env, c.path); ok && c.path != "" && (rest == "" || rest[0] == '#' || rest[0] == '?') {
			return shorten(fileSafe(c.name+rest), env)
		}
	}
	return shorten(fileSafe("other-"+env), env)
}

// fileSafe keeps letters, digits, '.', '_' and '-', turns every run of
// anything else into one '-', and trims '-' and '.' from the ends.
func fileSafe(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_':
			b.WriteRune(r)
			dash = false
		default:
			if !dash {
				b.WriteByte('-')
			}
			dash = true
		}
	}
	return strings.Trim(b.String(), "-.")
}

// shorten cuts a long key to a fixed length that is still distinct, by adding
// a hash of the env it came from.
func shorten(key, env string) string {
	if len(key) <= 80 {
		return key
	}
	return fmt.Sprintf("%s-%x", key[:60], sha256.Sum256([]byte(env)))[:69]
}

// gcRoot returns the profile path to run env under, creating its directory, or
// "" if there is no cache to put it in (a plan made without one) or the
// directory cannot be made: the step then runs unrooted, as before.
func (p *Plan) gcRoot(env string, log io.Writer) string {
	if p.Vars.Cache == "" {
		return ""
	}
	dir := filepath.Join(p.Vars.Cache, rootsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(log, "    warning: no GC root for the toolchain (%v); it may be collected\n", err)
		return ""
	}
	return filepath.Join(dir, p.Vars.rootKey(env))
}

// dropOldGenerations wipes the profile's generations but the current one, if
// it has more than one. A failure is a warning: the toolchain stays rooted,
// only a superseded one lingers.
func dropOldGenerations(ctx context.Context, root string, log io.Writer) {
	entries, err := os.ReadDir(filepath.Dir(root))
	if err != nil || ctx.Err() != nil { // a cancelled build has nothing to say
		return
	}
	prefix, n := filepath.Base(root)+"-", 0
	for _, e := range entries { // <key>-<number>-link, one per generation
		mid, ok := strings.CutPrefix(e.Name(), prefix)
		if !ok {
			continue
		}
		num, ok := strings.CutSuffix(mid, "-link")
		if !ok {
			continue
		}
		if _, err := strconv.Atoi(num); err == nil {
			n++
		}
	}
	if n < 2 {
		return
	}
	if out, err := exec.CommandContext(ctx, "nix", "profile", "wipe-history", "--profile", root).CombinedOutput(); err != nil {
		fmt.Fprintf(log, "    warning: dropping old toolchain generations of %s: %v\n%s", filepath.Base(root), err, out)
	}
}
