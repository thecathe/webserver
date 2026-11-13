package links

import "bytes"

// span is the byte range [start, end) of a file.
type span struct{ start, end int }

// findLinks returns where the links are in data, as linkRE.FindAllSubmatchIndex
// would: each match is a slice of offsets into data, and a group that did not
// take part is -1. For HTML (html is true) only linkRegions are searched; any
// other file type is searched whole.
func findLinks(data []byte, html bool) [][]int {
	if !html {
		return linkRE.FindAllSubmatchIndex(data, -1)
	}
	var all [][]int
	for _, s := range linkRegions(data) {
		for _, m := range linkRE.FindAllSubmatchIndex(data[s.start:s.end], -1) {
			for i := range m {
				if m[i] >= 0 {
					m[i] += s.start
				}
			}
			all = append(all, m)
		}
	}
	return all
}

// linkRegions returns, in order, the parts of an HTML file where a link may
// be: every tag (so every attribute value), and the content of <script> and
// <style> elements. Page text and comments are not, so an example of the link
// syntax in a <code> span or a paragraph is never a link. A link in a tag
// stays one wherever the tag sits, including inside <code>.
//
// It is a scanner, not a parser, and it fails toward the old behaviour: when a
// tag, comment, declaration or script is not terminated it cannot tell what
// the rest of the file is, so the rest is returned as a region, as if the file
// were not HTML. A link left as a raw address on a published page is a dead
// link nobody notices; a false positive is only what happened before.
//
// Not modelled: the text elements <title>, <textarea> and the like, whose
// content is text even if it looks like a tag (it is scanned as markup, which
// only matters for text that looks like a tag holding a link), and the
// "double escaped" script state, where a script's <!-- ... --> can hide a
// closing script tag.
func linkRegions(data []byte) []span {
	var out []span
	add := func(start, end int) {
		if end <= start {
			return
		}
		if n := len(out); n > 0 && out[n-1].end == start {
			out[n-1].end = end
			return
		}
		out = append(out, span{start, end})
	}
	i := 0
	for i < len(data) {
		j := bytes.IndexByte(data[i:], '<')
		if j < 0 {
			break
		}
		i += j
		rest := data[i:]
		var skip int // how far past i the thing at i ends, for a comment or declaration
		switch {
		case bytes.HasPrefix(rest, []byte("<!--")):
			// Searched from after "<!" so that "<!-->" and "<!--->" are
			// the empty comments they are.
			skip = indexEnd(rest, 2, "-->")
		case bytes.HasPrefix(rest, []byte("<![CDATA[")):
			skip = indexEnd(rest, 9, "]]>")
		case len(rest) > 1 && (rest[1] == '!' || rest[1] == '?'):
			// A doctype, or a processing instruction such as <?xml ?>.
			skip = indexEnd(rest, 2, ">")
		case isTagStart(rest):
			name, end, closed := scanTag(rest)
			if !closed {
				add(i, len(data))
				return out
			}
			add(i, i+end)
			i += end
			if rest[1] != '/' && (equalFold(name, "script") || equalFold(name, "style")) {
				c := closeTag(data[i:], name)
				if c < 0 {
					add(i, len(data))
					return out
				}
				add(i, i+c)
				i += c
			}
			continue
		default: // a "<" that starts nothing, as in "a < b"
			i++
			continue
		}
		if skip < 0 {
			add(i, len(data))
			return out
		}
		i += skip
	}
	return out
}

// indexEnd returns the offset just past the first end in b at or after from,
// or -1.
func indexEnd(b []byte, from int, end string) int {
	k := bytes.Index(b[from:], []byte(end))
	if k < 0 {
		return -1
	}
	return from + k + len(end)
}

// isTagStart reports whether b, which starts with "<", starts a start tag or
// an end tag.
func isTagStart(b []byte) bool {
	if len(b) > 1 && b[1] == '/' {
		b = b[1:]
	}
	return len(b) > 1 && (b[1]|0x20 >= 'a' && b[1]|0x20 <= 'z')
}

// scanTag scans the tag at the start of b ("<name attr=value ...>" or
// "</name>"). It returns the tag's name, the offset just past its ">", and
// whether it found one. A ">" inside a quoted attribute value does not end the
// tag, and an apostrophe or quote inside an unquoted value does not start one.
func scanTag(b []byte) (name []byte, end int, closed bool) {
	i := 1
	if b[i] == '/' {
		i++
	}
	start := i
	for i < len(b) && !isSpace(b[i]) && b[i] != '/' && b[i] != '>' {
		i++
	}
	name = b[start:i]
	for {
		for i < len(b) && (isSpace(b[i]) || b[i] == '/') {
			i++
		}
		if i >= len(b) {
			return name, len(b), false
		}
		if b[i] == '>' {
			return name, i + 1, true
		}
		// An attribute name. Its first character is part of it even if it
		// is "=".
		i++
		for i < len(b) && !isSpace(b[i]) && b[i] != '/' && b[i] != '>' && b[i] != '=' {
			i++
		}
		j := i
		for j < len(b) && isSpace(b[j]) {
			j++
		}
		if j >= len(b) || b[j] != '=' {
			continue // no value
		}
		j++
		for j < len(b) && isSpace(b[j]) {
			j++
		}
		if j >= len(b) {
			return name, len(b), false
		}
		switch q := b[j]; q {
		case '"', '\'':
			k := bytes.IndexByte(b[j+1:], q)
			if k < 0 {
				return name, len(b), false
			}
			i = j + 1 + k + 1
		case '>': // a missing value: the tag ends here
			i = j
		default: // unquoted
			for j < len(b) && !isSpace(b[j]) && b[j] != '>' {
				j++
			}
			i = j
		}
	}
}

// closeTag returns the offset in b of the "</name" that closes a raw-text
// element (script or style), compared without regard to case, or -1. Text such
// as "</scripty>" does not close it.
func closeTag(b []byte, name []byte) int {
	from := 0
	for {
		k := bytes.IndexByte(b[from:], '<')
		if k < 0 {
			return -1
		}
		k += from
		after := k + 2 + len(name)
		if after <= len(b) && b[k+1] == '/' && bytes.EqualFold(b[k+2:after], name) &&
			(after == len(b) || isSpace(b[after]) || b[after] == '/' || b[after] == '>') {
			return k
		}
		from = k + 1
	}
}

func equalFold(b []byte, s string) bool { return bytes.EqualFold(b, []byte(s)) }

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}
