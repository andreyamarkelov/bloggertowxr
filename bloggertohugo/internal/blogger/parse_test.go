package blogger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bloggertohugo/internal/model"
)

const minimalAtom = `<?xml version='1.0' encoding='UTF-8'?>
<feed xmlns='http://www.w3.org/2005/Atom' xmlns:blogger='http://schemas.google.com/blogger/2018'>
  <title>Fixture Blog</title>
  <entry>
    <id>tag:blogger.com,1999:blog.post-c1</id>
    <blogger:type>COMMENT</blogger:type>
    <blogger:status>LIVE</blogger:status>
    <title></title>
    <content type="html">&lt;p&gt;c&lt;/p&gt;</content>
    <published>2020-01-02T03:04:05Z</published>
    <author><name>Commenter</name></author>
  </entry>
  <entry>
    <id>tag:blogger.com,1999:blog.post-p1</id>
    <blogger:type>POST</blogger:type>
    <blogger:status>LIVE</blogger:status>
    <title>Hello</title>
    <content type="html">&lt;p&gt;Body&amp;nbsp;x&lt;/p&gt;</content>
    <published>2020-01-02T03:04:05Z</published>
    <category scheme="tag:blogger.com,1999:blog-x" term="News"/>
    <blogger:filename>/2012/09/sluggy.html</blogger:filename>
    <author><name>Author</name></author>
  </entry>
  <entry>
    <id>tag:blogger.com,1999:blog.post-page1</id>
    <blogger:type>PAGE</blogger:type>
    <blogger:status>LIVE</blogger:status>
    <title>About</title>
    <content type="html">&lt;p&gt;Pg&lt;/p&gt;</content>
    <published>2021-06-07T08:09:10Z</published>
    <blogger:filename>/p/about-us.html</blogger:filename>
    <author><name>P</name></author>
  </entry>
</feed>`

func TestParseReader_postsPagesAndSkipsComments(t *testing.T) {
	title, items, st, err := ParseReader(strings.NewReader(minimalAtom))
	if err != nil {
		t.Fatal(err)
	}
	if title != "Fixture Blog" {
		t.Fatalf("title: got %q", title)
	}
	if st.PostsImported != 1 || st.PagesImported != 1 || st.SkippedComment != 1 {
		t.Fatalf("stats: %+v", st)
	}
	if len(items) != 2 {
		t.Fatalf("items len: %d", len(items))
	}
	if items[0].Kind != model.KindPost || items[0].Slug != "sluggy" {
		t.Fatalf("post: %+v", items[0])
	}
	wantHTML := "<p>Body\u00a0x</p>"
	if items[0].HTML != wantHTML {
		t.Fatalf("html decode: %q want %q", items[0].HTML, wantHTML)
	}
	if len(items[0].Labels) != 1 || items[0].Labels[0] != "News" {
		t.Fatalf("labels: %#v", items[0].Labels)
	}
	if items[1].Kind != model.KindPage || items[1].Slug != "about-us" {
		t.Fatalf("page: %+v", items[1])
	}
}

func TestResolveFeedPath_singleBlog(t *testing.T) {
	root := t.TempDir()
	blogs := filepath.Join(root, "Blogs", "OnlyBlog")
	if err := os.MkdirAll(blogs, 0o755); err != nil {
		t.Fatal(err)
	}
	feed := filepath.Join(blogs, "feed.atom")
	if err := os.WriteFile(feed, []byte("<feed xmlns='http://www.w3.org/2005/Atom'></feed>"), 0o644); err != nil {
		t.Fatal(err)
	}
	name, path, err := ResolveFeedPath(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if name != "OnlyBlog" || filepath.Base(path) != "feed.atom" {
		t.Fatalf("got %q %q", name, path)
	}
}
