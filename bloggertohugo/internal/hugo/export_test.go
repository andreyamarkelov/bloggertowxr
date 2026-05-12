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
