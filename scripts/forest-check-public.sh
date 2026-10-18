#!/bin/sh
# Fail a public forest build that contains a tree tagged `private` (D51): the
# last line of defence if the forest's publish rule is ever lost or ignored.
#
#   forest-check-public.sh <output dir>
#
# forester writes forest.json (every built tree, with its tags) as one line.
set -eu
json="${1:-output}/forest.json"
if [ ! -f "$json" ]; then
  echo "forest-check-public: no $json, so the public build can't be checked" >&2
  exit 1
fi
leaks=$(grep -o '"uri":"[^"]*"[^}]*"tags":\[[^]]*"private"[^]]*\]' "$json" | sed 's/^"uri":"\([^"]*\)".*/\1/' || true)
if [ -n "$leaks" ]; then
  echo "forest-check-public: trees tagged private are in the public build:" >&2
  echo "$leaks" | sed 's/^/  /' >&2
  exit 1
fi
echo "forest-check-public: no private trees in the public build"
