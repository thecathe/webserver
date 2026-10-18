// Command ddm is the deployment manager: it reads sites.yaml, builds each site
// with its workflow, and (in later milestones) publishes it and keeps the
// server's NixOS config current.
//
//	ddm check [spec]            validate the spec and summarise what it declares
//	ddm sites [spec]            list every mount: host, path, site, view, access
//	ddm build [flags] <site>    build one view of a site locally, without publishing
//	ddm publish [flags] <site>  build one view of a site and publish it
//	ddm caddyfile [flags]       print the Caddyfile generated from the mounts
//	ddm preview [flags] [site…] build and publish sites locally, then serve them
//	ddm deploy [flags] [site…]  build and publish sites for production (the server)
//
// spec defaults to sites.yaml in the current directory.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"ddm/internal/build"
	"ddm/internal/spec"
)

const usage = `usage: ddm <command> ...

commands:
  check [spec]          validate the spec and summarise what it declares
  sites [spec]          list every mount (host, path, site, view, access)
  build [flags] <site>  build one view of a site, without publishing
  publish [flags] <site>
                        build one view of a site, then publish it
  caddyfile [flags]     print the Caddyfile generated from the mounts
  preview [flags] [site ...]
                        build and publish sites into .ddm/www, then serve
                        them all on http://<host>.localhost:8080
  deploy [flags] [site ...]
                        build and publish sites for production (on the
                        server: ddm-deploy.service runs this)

<command> -h lists a command's flags.

spec defaults to sites.yaml.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stderr, usage)
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	cmd, rest := args[0], args[1:]

	switch cmd {
	case "check", "sites":
		path := "sites.yaml"
		if len(rest) > 0 {
			path = rest[0]
		}
		s, ok := load(path, stderr)
		if !ok {
			return 1
		}
		if cmd == "check" {
			check(stdout, path, s)
		} else {
			sites(stdout, s)
		}
		return 0
	case "build":
		return cmdBuild(rest, stdout, stderr, false)
	case "publish":
		return cmdBuild(rest, stdout, stderr, true)
	case "caddyfile":
		return cmdCaddyfile(rest, stdout, stderr)
	case "preview":
		return cmdPreview(rest, stdout, stderr)
	case "deploy":
		return cmdDeploy(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "ddm: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
}

func load(path string, stderr io.Writer) (*spec.Spec, bool) {
	s, err := spec.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "ddm: %s is invalid:\n", path)
		for _, line := range strings.Split(err.Error(), "\n") {
			fmt.Fprintf(stderr, "  - %s\n", line)
		}
		return nil, false
	}
	return s, true
}

func check(w io.Writer, path string, s *spec.Spec) {
	mounts, views := 0, 0
	for _, name := range s.SiteNames() {
		mounts += len(s.Sites[name].Mounts)
		views += len(s.Sites[name].Views)
	}
	fmt.Fprintf(w, "%s is valid: %d sites (%d views, %d mounts), %d workflows, domains %s, poll %s\n",
		path, len(s.Sites), views, mounts, len(s.Workflows), strings.Join(s.Domains, ", "), s.System.Poll)
}

func sites(w io.Writer, s *spec.Spec) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tPATH\tSITE\tVIEW\tWORKFLOW\tACCESS\tENTRY")
	for _, name := range s.SiteNames() {
		site := s.Sites[name]
		for _, m := range site.Mounts {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				m.Host, m.Path, name, m.View, site.Views[m.View].Workflow, m.Access, orDash(m.Entry))
		}
	}
	tw.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// setFlags collects repeated --set k=v.
type setFlags map[string]string

func (s setFlags) String() string { return fmt.Sprint(map[string]string(s)) }
func (s setFlags) Set(v string) error {
	k, val, ok := strings.Cut(v, "=")
	if !ok || k == "" {
		return errors.New("want key=value")
	}
	s[k] = val
	return nil
}

func cmdBuild(args []string, stdout, stderr io.Writer, publishToo bool) int {
	name := "build"
	if publishToo {
		name = "publish"
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	specPath := fs.String("spec", "sites.yaml", "path to the site spec")
	view := fs.String("view", "public", "which view to build")
	src := fs.String("src", "", "build from this local git repo instead of the site's repo (committed state only)")
	branch := fs.String("branch", "", "branch to build (default: the site's branch)")
	work := fs.String("work", ".ddm", "working directory for checkouts and build output")
	dryRun := fs.Bool("dry-run", false, "print the resolved plan without running it")
	set := setFlags{}
	fs.Var(set, "set", "override a workflow param, key=value (repeatable)")
	var tf *targetFlags
	if publishToo {
		tf = addTargetFlags(fs, "preview")
	}
	fs.Usage = func() {
		if publishToo {
			fmt.Fprint(stderr, "usage: ddm publish [flags] <site>\n\nBuilds one view of a site, then publishes it to <www>/<view>/<site>/ and points current at it.\n\nflags:\n")
		} else {
			fmt.Fprint(stderr, "usage: ddm build [flags] <site>\n\nBuilds one view of a site into <work>/out/<site>/<view>/<commit>/. Nothing is published.\n\nflags:\n")
		}
		fs.PrintDefaults()
	}

	// Allow the site name before or after the flags.
	var site string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		site, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if site == "" && fs.NArg() > 0 {
		site = fs.Arg(0)
	}
	if site == "" {
		fs.Usage()
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	req := build.Request{
		Spec: s, Site: site, View: *view, System: system, Work: *work,
		Src: *src, Branch: *branch, Overrides: set, DryRun: *dryRun, Log: stdout,
	}
	if !publishToo {
		res, err := build.Run(ctx, req)
		if err != nil {
			fmt.Fprintf(stderr, "ddm: build %s (%s): %v\n", site, *view, err)
			return 1
		}
		if !*dryRun {
			fmt.Fprintf(stdout, "built %s (%s) at %s\n  -> %s\n", site, *view, res.Commit, res.Dir)
		}
		return 0
	}
	t, err := tf.resolve(s, *work)
	if err != nil {
		fmt.Fprintf(stderr, "ddm: %v\n", err)
		return 2
	}
	dest, err := buildAndPublish(ctx, req, t)
	if err != nil {
		fmt.Fprintf(stderr, "ddm: publish %s (%s): %v\n", site, *view, err)
		return 1
	}
	if dest != "" {
		fmt.Fprintf(stdout, "published %s (%s)\n  -> %s\n", site, *view, dest)
	}
	return 0
}
