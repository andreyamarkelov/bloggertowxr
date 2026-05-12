package hugo

import "regexp"

// mdImageWrappedInLink matches a Markdown link whose label is exactly one image:
//
//	[![alt](path)](https://...)
//
// Typical html-to-markdown output when <a><img></a> pointed at the original CDN
// while the image src was rewritten to a local bundle file.
var mdImageWrappedInLink = regexp.MustCompile(`\[(!\[[^\]]*\]\([^)]+\))\]\([^)]+\)`)

// stripMarkdownImageLinkWrappers replaces link-wrapped images with the inner image
// markdown only, so old Blogger / googleusercontent URLs do not remain as link targets.
func stripMarkdownImageLinkWrappers(md string) string {
	prev := ""
	for i := 0; i < 20 && md != prev; i++ {
		prev = md
		md = mdImageWrappedInLink.ReplaceAllString(md, "$1")
	}
	return md
}
