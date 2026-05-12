// Tests for wxr.Write output shape (CDATA, wp:tag, attachments, slug dedup).
package wxr

import (
	"strings"
	"testing"
	"time"

	"bloggertowxr/internal/model"
)

func TestWrite_postAndPage(t *testing.T) {
	items := []model.Content{
		{
			Kind:      model.KindPost,
			BloggerID: "tag:x.post-1",
			Title:     "T1",
			HTML:      `<p><img src="https://cdn.example/x/foo.png" /></p>`,
			Labels:    []string{"Tag A"},
			Published: time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC),
			Creator:   "me",
			Slug:      "t1",
		},
		{
			Kind:      model.KindPage,
			BloggerID: "tag:x.post-2",
			Title:     "About",
			HTML:      "<p>p]]>break</p>",
			Published: time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC),
			Creator:   "me",
			Slug:      "about",
		},
	}
	var buf strings.Builder
	if err := Write(&buf, Options{SiteTitle: "S", SiteURL: "https://ex.example"}, items); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<wp:post_type>post</wp:post_type>") || !strings.Contains(out, "<wp:post_type>page</wp:post_type>") {
		t.Fatalf("missing post types in:\n%s", out)
	}
	if !strings.Contains(out, "<category domain=\"post_tag\" nicename=\"tag-a\">") {
		t.Fatal("expected post_tag category")
	}
	if !strings.Contains(out, "<wp:tag>") || !strings.Contains(out, "<wp:tag_slug>tag-a</wp:tag_slug>") {
		t.Fatal("expected channel-level wp:tag (required by many WXR analyzers)")
	}
	if !strings.Contains(out, "<wp:post_type>attachment</wp:post_type>") || !strings.Contains(out, "<wp:attachment_url>") {
		t.Fatal("expected attachment item for remote img src")
	}
	if !strings.Contains(out, "]]]]><![CDATA[>") {
		t.Fatal("expected split CDATA for ]]>")
	}
}

func TestWrite_skipAttachments(t *testing.T) {
	items := []model.Content{
		{Kind: model.KindPost, Title: "P", HTML: `<img src="https://x/y.jpg">`, Published: time.Unix(1, 0).UTC(), Slug: "p"},
	}
	var buf strings.Builder
	if err := Write(&buf, Options{SiteTitle: "S", SiteURL: "https://x.test", SkipAttachmentItems: true}, items); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "<wp:post_type>attachment</wp:post_type>") {
		t.Fatal("did not expect attachments when SkipAttachmentItems")
	}
}

func TestPreviewCounts(t *testing.T) {
	items := []model.Content{
		{Labels: []string{"A", "B"}, HTML: `<img src="https://u/1.png"><img src="https://u/1.png"><img src='https://u/2.png'>`},
	}
	tags, imgs := PreviewCounts(items)
	if tags != 2 || imgs != 2 {
		t.Fatalf("PreviewCounts: tags=%d imgs=%d", tags, imgs)
	}
}

func TestWrite_duplicateSlugs(t *testing.T) {
	items := []model.Content{
		{Kind: model.KindPost, Title: "A", HTML: "<p>1</p>", Published: time.Unix(1, 0).UTC(), Slug: "dup"},
		{Kind: model.KindPost, Title: "B", HTML: "<p>2</p>", Published: time.Unix(2, 0).UTC(), Slug: "dup"},
	}
	var buf strings.Builder
	if err := Write(&buf, Options{SiteTitle: "S", SiteURL: "https://x.test"}, items); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<wp:post_name>dup</wp:post_name>") || !strings.Contains(out, "<wp:post_name>dup-2</wp:post_name>") {
		t.Fatalf("got:\n%s", out)
	}
}
