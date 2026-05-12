package hugo

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bloggertohugo/internal/model"
)

func TestResolveImageURL(t *testing.T) {
	if got := resolveImageURL("https://x/y.png", ""); got != "https://x/y.png" {
		t.Fatalf("abs: %q", got)
	}
	if got := resolveImageURL("//x/y.png", ""); got != "https://x/y.png" {
		t.Fatalf("//: %q", got)
	}
	if got := resolveImageURL("/a/b.jpg", "https://blog.example"); got != "https://blog.example/a/b.jpg" {
		t.Fatalf("rel root: %q", got)
	}
	if resolveImageURL("/a.jpg", "") != "" {
		t.Fatal("expected empty without base")
	}
}

func TestRemoveImgTagsForFailedDownloads(t *testing.T) {
	html := `<p>x <img src="https://bad.example/missing.png" alt="a"> y</p>`
	attempted := map[string]bool{"https://bad.example/missing.png": true}
	out := removeImgTagsForFailedDownloads(html, "", attempted, map[string]string{})
	if strings.Contains(out, "<img") {
		t.Fatalf("expected img removed, got %q", out)
	}
	if !strings.Contains(out, "<p>x") || !strings.Contains(out, "y</p>") {
		t.Fatalf("expected surrounding text kept, got %q", out)
	}
	// Successful download: do not remove before replace
	html2 := `<p><img src="https://ok/x.png"></p>`
	attempted2 := map[string]bool{"https://ok/x.png": true}
	okMap := map[string]string{"https://ok/x.png": "img-001.png"}
	out2 := removeImgTagsForFailedDownloads(html2, "", attempted2, okMap)
	if !strings.Contains(out2, "https://ok/x.png") {
		t.Fatalf("successful URL img should remain until replaceImgSrc: %q", out2)
	}
}

func TestRemoveEmptyAnchorTags(t *testing.T) {
	in := "<p><a href=\"https://blogger.googleusercontent.com/x\">\n\t</a>y</p>"
	out := removeEmptyAnchorTags(in)
	if strings.Contains(out, "<a ") {
		t.Fatalf("got %q", out)
	}
	if !strings.Contains(out, "y</p>") {
		t.Fatalf("got %q", out)
	}
}

func TestStripMarkdownImageLinkWrappers(t *testing.T) {
	in := `[![](img-001.jpg)](https://blogger.googleusercontent.com/img/b/R29vZ2xl/AVvXsEgIFhWThYHIBIQxhirkxGuwDpByfgug6tavVYxzeiaa6yVCvn9EgNy003OvxxAIyh6eK9DlSkcXCRt1ndl2bVCfQMje7uILO4wpFe6w9jAjUZul4VcKOGSFJdPQ4tLRGXtZGND0xtMr7I8u/s1600-h/window_tax.jpg)`
	want := `![](img-001.jpg)`
	if got := stripMarkdownImageLinkWrappers(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	in2 := `[![caption](img-002.png)](https://example.com/original)`
	want2 := `![caption](img-002.png)`
	if got := stripMarkdownImageLinkWrappers(in2); got != want2 {
		t.Fatalf("got %q want %q", got, want2)
	}
}

func TestReplaceImgSrc(t *testing.T) {
	html := `<p><img src="https://ex/u.png" alt="x"></p>`
	out := replaceImgSrc(html, map[string]string{"https://ex/u.png": "img-001.png"})
	if !strings.Contains(out, `src="img-001.png"`) {
		t.Fatalf("got %q", out)
	}
}

func TestExport_downloadsImageAndWritesBundle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) // minimal PNG signature-ish
	}))
	defer srv.Close()

	root := t.TempDir()
	items := []model.Content{{
		Kind:      model.KindPost,
		Title:     "Pic",
		HTML:      `<p><img src="` + srv.URL + `/shot.png" /></p>`,
		Published: time.Date(2022, 3, 4, 5, 6, 7, 0, time.UTC),
		Slug:      "pic-post",
	}}

	st, err := Export(root, items, Options{Concurrency: 2, HTTPTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if st.PostsWritten != 1 || st.ImagesOK < 1 {
		t.Fatalf("stats %+v", st)
	}
	mdPath := filepath.Join(root, "content", "posts", "pic-post", "index.md")
	b, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "slug: pic-post") {
		t.Fatalf("front matter: %s", s)
	}
	if !strings.Contains(s, "img-001") {
		t.Fatalf("expected local image ref in md, got:\n%s", s)
	}
}
