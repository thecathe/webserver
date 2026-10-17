package main

import (
	"context"
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
	"time"

	"ddm/internal/bar"
	"ddm/internal/build"
	"ddm/internal/caddygen"
	"ddm/internal/links"
	"ddm/internal/publish"
	"ddm/internal/spec"
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
	dest, err := publish.Run(publish.Request{
		Site: res.Served, View: req.View, Commit: res.Commit, From: res.Dir, WWW: t.caddy.WWW,
		Links: links.Resolver{Spec: req.Spec, Base: t.base}, Strict: t.strict, Log: req.Log,
		Head: bar.Script,
	})
	if err != nil {
		return "", err
	}
	// The toolbar's files (nav.json follows the spec) and this publish's status.
	if err := bar.Write(t.caddy.WWW, req.Spec, t.base); err != nil {
		return "", err
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
					Spec: s, Site: name, View: view, System: system, Work: *work, Log: stdout,
					Src: src, Branch: branch,
				}, t)
				if err != nil {
					if ctx.Err() != nil {
						return 1
					}
					fmt.Fprintf(stderr, "ddm: %s (%s) failed: %v\n", name, view, err)
					failed = append(failed, name+" ("+view+")")
					continue
				}
				fmt.Fprintf(stdout, "    %s (%s) published in %s\n", name, view, time.Since(start).Round(time.Second))
			}
		}
	}

	if err := bar.Write(t.caddy.WWW, s, t.base); err != nil {
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
