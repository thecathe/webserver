#!/bin/sh
# Render a forester build's XML pages to HTML with the forest's own XSLT.
#
#   forest-render-html.sh <output dir>
#
# Every tree is <dir>/<addr>/index.xml (styled by /default.xsl) plus an
# index.html that only redirects to it. Browsers are dropping XSLT, so this
# runs the same stylesheet here (xsltproc, XSLT 1.0) and writes the result
# over index.html: the same page, as plain HTML. index.xml stays, for
# anything that reads it.
set -eu
dir=$1
xsl="$dir/default.xsl"
[ -f "$xsl" ] || { echo "no $xsl: not a forester build?" >&2; exit 1; }

count=$(find "$dir" -name index.xml | wc -l)
find "$dir" -name index.xml -print0 |
  xargs -0 -P "$(nproc)" -I{} sh -c 'xsltproc --nonet -o "$(dirname "$1")/index.html" "$2" "$1"' _ {} "$xsl"
echo "rendered $count pages to HTML"
