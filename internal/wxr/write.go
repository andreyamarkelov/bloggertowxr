// Package wxr writes WordPress WXR 1.2 XML from model.Content slices.
// Output is compatible with common importers (WordPress, Publii): channel wp:tag
// blocks, per-item categories, and optional attachment items for remote <img> URLs.
package wxr

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	"bloggertowxr/internal/model"
)

// Options controls channel metadata in the export file.
type Options struct {
	SiteTitle string // rss/channel/title
	SiteURL   string // no trailing slash; used for link, guid, base_* urls
	// SkipAttachmentItems, when true, omits wp:post_type attachment rows for <img src="http(s)..."> URLs.
	// Tools such as Publii count “images” from those attachment items; inline HTML alone does not count.
	SkipAttachmentItems bool
}

var tagNicenameRe = regexp.MustCompile(`[^\p{L}\p{N}\-]+`)

// imgTagSrc matches Blogger-style <img ... src="..."> (single or double quotes).
var imgTagSrcRE = regexp.MustCompile(`(?i)<img\b[^>]*\bsrc\s*=\s*["']([^"']+)["']`)

// tagTerm is one channel-level <wp:tag> (distinct Blogger label / WordPress tag).
type tagTerm struct {
	TermID int
	Slug   string
	Name   string
}

// PreviewCounts returns how many distinct tags and distinct remote image URLs will appear in the WXR
// (tags as channel <wp:tag>; images as attachment items unless Options.SkipAttachmentItems).
func PreviewCounts(items []model.Content) (tagTerms int, imageAttachments int) {
	return len(collectTags(items)), len(collectImageURLs(items))
}

// Write emits a WordPress WXR 1.2 document (posts and pages, no comments).
// Order: channel head → wp:tag list → post/page items → optional attachment items → close channel/rss.
func Write(w io.Writer, opts Options, items []model.Content) error {
	siteURL := strings.TrimSuffix(strings.TrimSpace(opts.SiteURL), "/")
	if siteURL == "" {
		siteURL = "https://example.com"
	}
	title := strings.TrimSpace(opts.SiteTitle)
	if title == "" {
		title = "Imported blog"
	}

	head := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!-- generator="bloggertowxr WXR 1.2" -->
<rss version="2.0"
	xmlns:excerpt="http://wordpress.org/export/1.2/excerpt/"
	xmlns:content="http://purl.org/rss/1.0/modules/content/"
	xmlns:wfw="http://wellformedweb.org/CommentAPI/"
	xmlns:dc="http://purl.org/dc/elements/1.1/"
	xmlns:wp="http://wordpress.org/export/1.2/"
>
<channel>
	<title>%s</title>
	<link>%s</link>
	<description></description>
	<language>en-US</language>
	<wp:wxr_version>1.2</wp:wxr_version>
	<wp:base_site_url>%s</wp:base_site_url>
	<wp:base_blog_url>%s</wp:base_blog_url>
`, xmlEscape(title), xmlEscape(siteURL), xmlEscape(siteURL), xmlEscape(siteURL))

	if _, err := io.WriteString(w, head); err != nil {
		return err
	}

	for _, t := range collectTags(items) {
		block := fmt.Sprintf(
			"\t<wp:tag>\n\t\t<wp:term_id>%d</wp:term_id>\n\t\t<wp:tag_slug>%s</wp:tag_slug>\n\t\t<wp:tag_name>%s</wp:tag_name>\n\t</wp:tag>\n",
			t.TermID,
			xmlEscape(t.Slug),
			cdataWrap(t.Name),
		)
		if _, err := io.WriteString(w, block); err != nil {
			return err
		}
	}

	// WordPress post IDs must be unique; we assign sequential integers starting at 1.
	slugSeen := make(map[string]int)
	for i, it := range items {
		postID := i + 1
		slug := uniquifySlug(slugSeen, it.Slug)
		if err := writeItem(w, siteURL, postID, it, slug); err != nil {
			return err
		}
	}

	// Attachment items use IDs after the last post/page so wp:post_id stays unique.
	if !opts.SkipAttachmentItems {
		baseID := len(items) + 1
		attSlugSeen := make(map[string]int)
		for i, imgURL := range collectImageURLs(items) {
			if err := writeAttachmentItem(w, baseID+i, imgURL, attSlugSeen); err != nil {
				return err
			}
		}
	}

	if _, err := io.WriteString(w, "\n</channel>\n</rss>\n"); err != nil {
		return err
	}
	return nil
}

// collectTags deduplicates labels by nicename and assigns stable wp:term_id values (sorted by slug).
func collectTags(items []model.Content) []tagTerm {
	seen := make(map[string]string) // nicename -> display name (first seen)
	for _, it := range items {
		for _, label := range it.Labels {
			nn := tagNicename(label)
			if nn == "" {
				continue
			}
			if _, ok := seen[nn]; !ok {
				seen[nn] = strings.TrimSpace(label)
			}
		}
	}
	slugs := make([]string, 0, len(seen))
	for nn := range seen {
		slugs = append(slugs, nn)
	}
	sort.Strings(slugs)
	out := make([]tagTerm, len(slugs))
	for i, nn := range slugs {
		out[i] = tagTerm{
			TermID: i + 1,
			Slug:   nn,
			Name:   seen[nn],
		}
	}
	return out
}

// collectImageURLs returns unique http(s) URLs from <img src="..."> in post/page HTML (order preserved).
func collectImageURLs(items []model.Content) []string {
	seen := make(map[string]bool)
	var order []string
	for _, it := range items {
		for _, m := range imgTagSrcRE.FindAllStringSubmatch(it.HTML, -1) {
			if len(m) < 2 {
				continue
			}
			u := strings.TrimSpace(m[1])
			if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
				continue
			}
			if seen[u] {
				continue
			}
			seen[u] = true
			order = append(order, u)
		}
	}
	return order
}

// uniquifySlug returns base, or base-2, base-3, ... if the same slug was already used (WordPress uniqueness).
func uniquifySlug(seen map[string]int, base string) string {
	if strings.TrimSpace(base) == "" {
		base = "item"
	}
	n := seen[base]
	seen[base] = n + 1
	if n == 0 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, n+1)
}

// writeItem emits one <item> for a post or page (wp:post_type post|page).
func writeItem(w io.Writer, siteURL string, postID int, it model.Content, slug string) error {
	// Use UTC for both fields so output does not depend on the machine timezone.
	gmt := it.Published.UTC().Format("2006-01-02 15:04:05")
	local := gmt
	pubDate := it.Published.UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000")

	guid := fmt.Sprintf("%s/?p=%d", siteURL, postID)
	link := fmt.Sprintf("%s/%s/", strings.TrimSuffix(siteURL, "/"), strings.Trim(slug, "/"))

	postType := string(it.Kind)
	creator := it.Creator
	if creator == "" {
		creator = "blogger-import"
	}

	var b strings.Builder
	fmt.Fprintf(&b, `
	<item>
		<title>%s</title>
		<link>%s</link>
		<pubDate>%s</pubDate>
		<dc:creator>%s</dc:creator>
		<guid isPermaLink="false">%s</guid>
		<description></description>
		<content:encoded>%s</content:encoded>
		<excerpt:encoded><![CDATA[]]></excerpt:encoded>
		<wp:post_id>%d</wp:post_id>
		<wp:post_date>%s</wp:post_date>
		<wp:post_date_gmt>%s</wp:post_date_gmt>
		<wp:comment_status>closed</wp:comment_status>
		<wp:ping_status>closed</wp:ping_status>
		<wp:post_name>%s</wp:post_name>
		<wp:status>publish</wp:status>
		<wp:post_parent>0</wp:post_parent>
		<wp:menu_order>0</wp:menu_order>
		<wp:post_type>%s</wp:post_type>
		<wp:post_password></wp:post_password>
		<wp:is_sticky>0</wp:is_sticky>
`,
		xmlEscape(it.Title),
		xmlEscape(link),
		xmlEscape(pubDate),
		cdataWrap(creator),
		xmlEscape(guid),
		cdataWrap(it.HTML),
		postID,
		xmlEscape(local),
		xmlEscape(gmt),
		xmlEscape(slug),
		xmlEscape(postType),
	)

	for _, label := range it.Labels {
		nn := tagNicename(label)
		if nn == "" {
			continue
		}
		fmt.Fprintf(&b, "\t\t<category domain=\"post_tag\" nicename=\"%s\">%s</category>\n",
			xmlEscape(nn), cdataWrap(label))
	}

	b.WriteString("\t</item>\n")

	_, err := io.WriteString(w, b.String())
	return err
}

// attachmentTitleAndSlug derives a human title and wp:post_name from the image URL path.
func attachmentTitleAndSlug(rawURL string, slugSeen map[string]int) (title string, slug string) {
	parsed, err := url.Parse(rawURL)
	base := "image"
	if err == nil && parsed.Path != "" {
		base = path.Base(parsed.Path)
		base = strings.TrimSuffix(base, path.Ext(base))
	}
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == "/" {
		base = "image"
	}
	title = base
	slug = uniquifySlug(slugSeen, tagNicename(base))
	return title, slug
}

// writeAttachmentItem emits a minimal media-library style row; guid and wp:attachment_url point at the remote file.
func writeAttachmentItem(w io.Writer, postID int, attachmentURL string, slugSeen map[string]int) error {
	title, slug := attachmentTitleAndSlug(attachmentURL, slugSeen)
	guid := attachmentURL
	link := attachmentURL
	pubDate := "Thu, 01 Jan 1970 00:00:00 +0000"
	local := "1970-01-01 00:00:00"
	creator := "blogger-import"

	var b strings.Builder
	fmt.Fprintf(&b, `
	<item>
		<title>%s</title>
		<link>%s</link>
		<pubDate>%s</pubDate>
		<dc:creator>%s</dc:creator>
		<guid isPermaLink="false">%s</guid>
		<description></description>
		<content:encoded><![CDATA[]]></content:encoded>
		<excerpt:encoded><![CDATA[]]></excerpt:encoded>
		<wp:post_id>%d</wp:post_id>
		<wp:post_date>%s</wp:post_date>
		<wp:post_date_gmt>%s</wp:post_date_gmt>
		<wp:comment_status>open</wp:comment_status>
		<wp:ping_status>closed</wp:ping_status>
		<wp:post_name>%s</wp:post_name>
		<wp:status>inherit</wp:status>
		<wp:post_parent>0</wp:post_parent>
		<wp:menu_order>0</wp:menu_order>
		<wp:post_type>attachment</wp:post_type>
		<wp:post_password></wp:post_password>
		<wp:is_sticky>0</wp:is_sticky>
		<wp:attachment_url>%s</wp:attachment_url>
	</item>
`,
		xmlEscape(title),
		xmlEscape(link),
		xmlEscape(pubDate),
		cdataWrap(creator),
		xmlEscape(guid),
		postID,
		xmlEscape(local),
		xmlEscape(local),
		xmlEscape(slug),
		xmlEscape(attachmentURL),
	)

	_, err := io.WriteString(w, b.String())
	return err
}

// tagNicename normalizes a label to a WordPress-style nicename (lowercase, hyphenated).
func tagNicename(label string) string {
	s := strings.TrimSpace(strings.ToLower(label))
	s = strings.ReplaceAll(s, "_", "-")
	s = tagNicenameRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// cdataWrap wraps arbitrary text in CDATA; splits if the text contains the illegal sequence "]]>".
func cdataWrap(s string) string {
	escaped := s
	if strings.Contains(escaped, "]]>") {
		escaped = strings.ReplaceAll(escaped, "]]>", "]]]]><![CDATA[>")
	}
	return "<![CDATA[" + escaped + "]]>"
}

// xmlEscape escapes text for XML elements/attributes (not for raw HTML inside CDATA).
func xmlEscape(s string) string {
	var buf strings.Builder
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}
