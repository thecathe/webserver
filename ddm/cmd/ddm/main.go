// Command ddm is the deployment manager: it reads sites.yaml, and (in later
// milestones) builds each site with its workflow, publishes it and keeps the
// server's NixOS config current.
//
//	ddm check [spec]   validate the spec and summarise what it declares
//	ddm sites [spec]   list every mount: host, path, site, view, access
//
// spec defaults to sites.yaml in the current directory.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"ddm/internal/spec"
)

const usage = `usage: ddm <command> [spec]

commands:
  check   validate the spec and summarise what it declares
  sites   list every mount (host, path, site, view, access)

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
	path := "sites.yaml"
	if len(rest) > 0 {
		path = rest[0]
	}

	switch cmd {
	case "check", "sites":
	default:
		fmt.Fprintf(stderr, "ddm: unknown command %q\n\n%s", cmd, usage)
		return 2
	}

	s, err := spec.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "ddm: %s is invalid:\n", path)
		for _, line := range strings.Split(err.Error(), "\n") {
			fmt.Fprintf(stderr, "  - %s\n", line)
		}
		return 1
	}

	switch cmd {
	case "check":
		check(stdout, path, s)
	case "sites":
		sites(stdout, s)
	}
	return 0
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
