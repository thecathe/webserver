#!/bin/sh
# Prepare one opam switch that can build every project in a workspace.
#
#   opam-pool-switch.sh <cache dir> <workspace dir>
#
# The opam root and switch live in the cache, so later builds only install
# what changed. Each project's lockfile (*.opam.locked) is tried first, for
# exact versions; if the lockfiles conflict with each other (they pin
# different compilers, say), one opam solve over every project's .opam
# constraints is used instead.
set -eu
cache=$1
ws=$2

export OPAMROOT="$cache/opam-root" OPAMYES=1 OPAMCOLOR=never OPAMNODEPEXTS=1
switch="$cache/switch"

if [ ! -f "$OPAMROOT/config" ]; then
  opam init --bare --disable-sandboxing --no-setup default https://opam.ocaml.org
fi
opam update --quiet default
if [ ! -d "$switch/_opam" ]; then
  mkdir -p "$switch"
  opam switch create "$switch" --empty
fi

projects=$(find "$ws" -mindepth 1 -maxdepth 1 -type d | sort)
echo "projects: $(echo $projects | xargs -n1 basename | tr '\n' ' ')"

if opam install --switch="$switch" --deps-only --with-doc --locked $projects; then
  echo "switch: installed from the lockfiles"
else
  echo "switch: the lockfiles conflict; solving over every project's .opam constraints instead"
  opam install --switch="$switch" --deps-only --with-doc $projects
fi
# odoc itself, if no project's doc dependencies brought it. Not on the line
# above: --deps-only would install only odoc's own dependencies.
opam install --switch="$switch" odoc
opam list --switch="$switch" --installed --columns=name,version --short | tr '\n' ' ' | head -c 4000
echo
du -sh "$OPAMROOT" "$switch" 2>/dev/null || true
