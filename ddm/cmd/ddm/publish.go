package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"ddm/internal/aimark"
	"ddm/internal/bar"
	"ddm/internal/build"
	"ddm/internal/caddygen"
	"ddm/internal/links"
	"ddm/internal/publish"
	"ddm/internal/spec"
	"ddm/internal/state"
)

// target is where publishing and the Caddyfile point: the server (prod) or
// a local preview.
type target struct {
	caddy  caddygen.Target
	base   links.BaseURL
	strict bool
}

type targetFlags struct {
	name   *string
	port   *int
	www    *string
	strict *bool
}

func addTargetFlags(fs *flag.FlagSet, def string) *targetFlags {
	return &targetFlags{
		name:   fs.String("target", def, "prod (https, the spec's paths.www) or preview (http://<host>.localhost:<port>, <work>/www)"),
		port:   fs.Int("port", 8080, "preview port"),
		www:    fs.String("www", "", "publish root (default: the target's)"),
		strict: fs.Bool("strict", false, "fail on an unresolved site:// link (always on for prod)"),
	}
}

func (f *targetFlags) resolve(s *spec.Spec, work string) (target, error) {
	switch *f.name {
	case "prod":
		www := *f.www
		if www == "" {
			www = s.Paths.WWW
		}
		return target{caddy: caddygen.Target{WWW: www}, base: links.Prod, strict: true}, nil
	case "preview":
		www := *f.www
		if www == "" {
			www = filepath.Join(work, "www")
		}
		www, err := filepath.Abs(www)
		if err != nil {
			return target{}, err
		}
		port := *f.port
		return target{
			caddy:  caddygen.Target{Preview: true, Port: port, WWW: www},
			base:   caddygen.Target{Preview: true, Port: port}.Base,
			strict: *f.strict,
		}, nil
	}
	return target{}, fmt.Errorf("--target must be prod or preview, not %q", *f.name)
}

// buildAndPublish builds one view and publishes it for t. It returns the
// publish directory, or "" for a dry run.
func buildAndPublish(ctx context.Context, req build.Request, t target) (string, error) {
	res, err := build.Run(ctx, req)
	if err != nil || req.DryRun {
		return "", err
	}
	fmt.Fprintf(req.Log, "==> publishing %s (%s) at %s\n", res.Served, req.View, res.Commit)
	pageHead, err := aimark.PageHead(req.Spec.Sites[req.Site].Generated)
	if err != nil {
		return "", err
	}
	dest, err := publish.Run(publish.Request{
		Site: res.Served, View: req.View, Commit: res.Commit, From: res.Dir, WWW: t.caddy.WWW,
		Links: links.Resolver{Spec: req.Spec, Base: t.base}, Strict: t.strict, Log: req.Log,
		Head: bar.Script + aimark.Tags, PageHead: pageHead,
	})
	if err != nil {
		return "", err
	}
	// The toolbar's files (nav.json follows the spec), the AI marker's, this
	// publish's status, and, for a public build, its page list.
	if err := bar.Write(t.caddy.WWW, req.Spec, t.base, t.caddy.Preview); err != nil {
		return "", err
	}
	if err := aimark.Write(t.caddy.WWW); err != nil {
		return "", err
	}
	if req.View == spec.DefaultView {
		if err := bar.WritePaths(t.caddy.WWW, req.View, res.Served, dest); err != nil {
			return "", err
		}
	}
	return dest, bar.WriteStatus(t.caddy.WWW, req.Spec, bar.Status{
		Served: res.Served, View: req.View, Commit: res.Commit, Published: time.Now().UTC(), Members: res.Members,
	})
}

func cmdCaddyfile(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("caddyfile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	specPath := fs.String("spec", "sites.yaml", "path to the site spec")
	work := fs.String("work", ".ddm", "working directory (the preview's www is <work>/www)")
	tf := addTargetFlags(fs, "prod")
	fs.Usage = func() {
		fmt.Fprint(stderr, "usage: ddm caddyfile [flags]\n\nPrints the Caddyfile generated from the spec's mounts.\n\nflags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	s, ok := load(*specPath, stderr)
	if !ok {
		return 1
	}
	t, err := tf.resolve(s, *work)
	if err != nil {
		fmt.Fprintf(stderr, "ddm: %v\n", err)
		return 2
	}
	fmt.Fprint(stdout, caddygen.Generate(s, t.caddy))
	return 0
}

func cmdPreview(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.SetOutput(stderr)
	specPath := fs.String("spec", "sites.yaml", "path to the site spec")
	work := fs.String("work", ".ddm", "working directory for checkouts, builds and the preview's www")
	port := fs.Int("port", 8080, "port to serve on")
	serveOnly := fs.Bool("serve-only", false, "don't build: serve what was last published")
	noServe := fs.Bool("no-serve", false, "build and publish, but don't start Caddy")
	strict := fs.Bool("strict", false, "fail a site on an unresolved site:// link")
	srcs := setFlags{}
	fs.Var(srcs, "src", "build site from a local git repo instead of its own, name=path[@branch] (repeatable; committed state only)")
	fs.Usage = func() {
		fmt.Fprint(stderr, `usage: ddm preview [flags] [site ...]

Builds and publishes every view of the given sites (default: all) into
<work>/www, links resolved for the preview, then serves them all with Caddy
on http://<host>.localhost:<port>. A site that fails to build is reported
and skipped; the rest are still served. Private mounts are served without
the tailnet check.

flags:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	s, ok := load(*specPath, stderr)
	if !ok {
		return 1
	}
	system, err := filepath.Abs(filepath.Dir(*specPath))
	if err != nil {
		fmt.Fprintf(stderr, "ddm: %v\n", err)
		return 1
	}
	sites := fs.Args()
	if len(sites) == 0 {
		sites = s.SiteNames()
	}
	for _, name := range sites {
		if _, ok := s.Sites[name]; !ok {
			fmt.Fprintf(stderr, "ddm: no site %q (sites: %s)\n", name, strings.Join(s.SiteNames(), ", "))
			return 2
		}
	}
	tf := &targetFlags{name: ptr("preview"), port: port, www: ptr(""), strict: strict}
	t, err := tf.resolve(s, *work)
	if err != nil {
		fmt.Fprintf(stderr, "ddm: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var failed []string
	if !*serveOnly {
		if failed = publishAll(ctx, s, sites, system, *work, srcs, t, stdout, stderr); ctx.Err() != nil {
			return 1
		}
	}

	if err := bar.Write(t.caddy.WWW, s, t.base, true); err != nil {
		fmt.Fprintf(stderr, "ddm: %v\n", err)
		return 1
	}
	caddyfile := filepath.Join(*work, "Caddyfile.preview")
	if err := os.WriteFile(caddyfile, []byte(caddygen.Generate(s, t.caddy)), 0o644); err != nil {
		fmt.Fprintf(stderr, "ddm: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "\nPreview (Caddyfile: %s):\n", caddyfile)
	for _, name := range s.SiteNames() {
		for _, m := range s.Sites[name].Mounts {
			mark := ""
			if _, err := os.Stat(publish.Current(t.caddy.WWW, m.View, s.Served(name, m.View))); err != nil {
				mark = "   (not published)"
			}
			path := strings.TrimSuffix(m.Path, "/") + "/"
			fmt.Fprintf(stdout, "  %-8s %s%s%s\n", m.Access, t.base(m.Host), path, mark)
		}
	}
	if len(failed) > 0 {
		fmt.Fprintf(stdout, "\nFailed: %s\n", strings.Join(failed, ", "))
	}
	if *noServe {
		if len(failed) > 0 {
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "\nServing on port %d; Ctrl-C to stop.\n", *port)
	cmd := exec.CommandContext(ctx, "caddy", "run", "--adapter", "caddyfile", "--config", caddyfile)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			fmt.Fprintf(stderr, "ddm: running caddy (is it on PATH? use nix develop): %v\n", err)
		}
		return 1
	}
	return 0
}

// publishAll builds and publishes every built view of sites for t, and
// returns the ones that failed ("site (view)"). A failure doesn't stop the
// rest; an interrupt does (check ctx.Err()).
func publishAll(ctx context.Context, s *spec.Spec, sites []string, system, work string, srcs setFlags, t target, stdout, stderr io.Writer) []string {
	var failed []string
	done := map[string]bool{} // a pool is built once, whichever member comes first
	for _, name := range sites {
		for _, view := range builtViews(s.Sites[name]) {
			if key := s.Served(name, view) + "/" + view; done[key] {
				continue
			} else {
				done[key] = true
			}
			start := time.Now()
			src, branch, _ := strings.Cut(srcs[name], "@")
			_, err := buildAndPublish(ctx, build.Request{
				Spec: s, Site: name, View: view, System: system, Work: work, Log: stdout,
				Src: src, Branch: branch,
			}, t)
			if err != nil {
				if ctx.Err() != nil {
					return failed
				}
				fmt.Fprintf(stderr, "ddm: %s (%s) failed: %v\n", name, view, err)
				failed = append(failed, name+" ("+view+")")
				continue
			}
			fmt.Fprintf(stdout, "    %s (%s) published in %s\n", name, view, time.Since(start).Round(time.Second))
		}
	}
	return failed
}

// cmdDeploy publishes every site (or those named) for production: the
// server's stand-in for the daemon (M6), run by ddm-deploy.service.
func cmdDeploy(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("deploy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	specPath := fs.String("spec", "sites.yaml", "path to the site spec")
	work := fs.String("work", "", "working directory for checkouts and builds (default: the spec's paths.work)")
	www := fs.String("www", "", "publish root (default: the spec's paths.www)")
	force := fs.Bool("force", false, "build every view, even those up to date or cooling down")
	fs.Usage = func() {
		fmt.Fprint(stderr, `usage: ddm deploy [flags] [site ...]

Builds and publishes every view of the given sites (default: all) that
needs it, for production: links resolve to https://<host>, an unresolved
site:// link fails its site, and the publish root is the spec's paths.www.

A view is skipped when it was last built from its branch's current commit
and nothing else that goes into it (the spec, this repo's tools, ddm) has
changed. A new commit waits out the site's cooldown. A build that failed
is retried at the same commit after an hour, or the cooldown if longer.
<work>/state.json keeps what was built when.

A site that fails is reported and skipped; the exit status is 1 if any did.

flags:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	s, ok := load(*specPath, stderr)
	if !ok {
		return 1
	}
	if *work == "" {
		*work = s.Paths.Work
	}
	system, err := filepath.Abs(filepath.Dir(*specPath))
	if err != nil {
		fmt.Fprintf(stderr, "ddm: %v\n", err)
		return 1
	}
	sites := fs.Args()
	if len(sites) == 0 {
		sites = s.SiteNames()
	}
	for _, name := range sites {
		if _, ok := s.Sites[name]; !ok {
			fmt.Fprintf(stderr, "ddm: no site %q (sites: %s)\n", name, strings.Join(s.SiteNames(), ", "))
			return 2
		}
	}
	tf := &targetFlags{name: ptr("prod"), port: ptr(0), www: www, strict: ptr(true)}
	t, err := tf.resolve(s, *work)
	if err != nil {
		fmt.Fprintf(stderr, "ddm: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	st, err := state.Load(*work)
	if err != nil {
		fmt.Fprintf(stderr, "ddm: reading the state: %v\n", err)
		return 1
	}
	stamp, err := stampOf(*specPath, system)
	if err != nil {
		fmt.Fprintf(stderr, "ddm: %v\n", err)
		return 1
	}

	var failed []string
	done := map[string]bool{} // a pool is built once, whichever member comes first
	for _, name := range sites {
		for _, view := range builtViews(s.Sites[name]) {
			served := s.Served(name, view)
			key := state.Key(view, served)
			if done[key] {
				continue
			}
			done[key] = true
			req := build.Request{Spec: s, Site: name, View: view, System: system, Work: *work, Log: stdout}

			head, err := build.Head(req)
			if err != nil {
				fmt.Fprintf(stderr, "ddm: %s (%s): checking for new commits: %v\n", name, view, err)
				failed = append(failed, name+" ("+view+")")
				continue
			}
			_, perr := os.Stat(publish.Current(t.caddy.WWW, view, served))
			run, why := st.Builds[key].Why(head, stamp, perr == nil, s.Cooldown(name, view), time.Now())
			if !run && !*force {
				fmt.Fprintf(stdout, "    %s (%s): %s\n", served, view, why)
				continue
			}
			if *force {
				why = "forced"
			}
			fmt.Fprintf(stdout, "==> %s (%s): building: %s\n", served, view, why)

			start := time.Now()
			_, err = buildAndPublish(ctx, req, t)
			if ctx.Err() != nil {
				return 1
			}
			end := time.Now()
			b := state.Build{Commit: head, Stamp: stamp, At: end}
			if err != nil {
				fmt.Fprintf(stderr, "ddm: %s (%s) failed: %v\n", name, view, err)
				failed = append(failed, name+" ("+view+")")
				b.Error = err.Error()
			} else {
				fmt.Fprintf(stdout, "    %s (%s) published in %s\n", served, view, end.Sub(start).Round(time.Second))
			}
			st.Builds[key] = b
			st.Runs = append(st.Runs, state.Run{Start: start, End: end})
			if err := st.Save(*work, end); err != nil {
				fmt.Fprintf(stderr, "ddm: saving the state: %v\n", err)
				return 1
			}
		}
	}

	// A server that is nearly always building needs a longer cooldown, or
	// more CPU. Notifications will carry this (open questions).
	if busy := st.Busy(time.Now(), 24*time.Hour); busy >= busyWarning {
		fmt.Fprintf(stderr, "ddm: warning: building %.0f%% of the last 24 hours\n", 100*busy)
	} else {
		fmt.Fprintf(stdout, "\nbuilding %.0f%% of the last 24 hours\n", 100*busy)
	}
	if err := bar.Write(t.caddy.WWW, s, t.base, false); err != nil {
		fmt.Fprintf(stderr, "ddm: %v\n", err)
		return 1
	}
	if len(failed) > 0 {
		fmt.Fprintf(stdout, "\nFailed: %s\n", strings.Join(failed, ", "))
		return 1
	}
	return 0
}

// busyWarning is the share of a day spent building that deploy warns about.
const busyWarning = 0.5

// stampOf fingerprints what goes into every build besides a site's own
// commit: the spec, this repo's flake and scripts (workflow environments and
// steps), and the ddm binary (publishing, the toolbar). A change to any of
// them rebuilds everything once.
func stampOf(specPath, system string) (string, error) {
	h := sha256.New()
	add := func(path string) error {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s %d\n", path, len(data))
		h.Write(data)
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	paths := []string{specPath, exe, filepath.Join(system, "flake.nix"), filepath.Join(system, "flake.lock")}
	scripts, _ := filepath.Glob(filepath.Join(system, "scripts", "*"))
	for _, p := range append(paths, scripts...) {
		if err := add(p); err != nil {
			return "", fmt.Errorf("fingerprinting %s: %w", p, err)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// builtViews are the views of a site worth building in a preview: those a
// mount serves, or every view of a pool member with no mounts of its own.
func builtViews(site spec.Site) []string {
	var views []string
	for _, m := range site.Mounts {
		if !slices.Contains(views, m.View) {
			views = append(views, m.View)
		}
	}
	if len(site.Mounts) == 0 {
		for v := range site.Views {
			views = append(views, v)
		}
	}
	slices.Sort(views)
	return views
}

func ptr[T any](v T) *T { return &v }
