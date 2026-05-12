package hugo

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"gopkg.in/yaml.v3"

	"bloggertohugo/internal/model"
)

// imgTagSrc matches <img ... src="..."> (same idea as bloggertowxr WXR writer).
var imgTagSrcRE = regexp.MustCompile(`(?i)<img\b([^>]*)\bsrc\s*=\s*["']([^"']+)["']([^>]*)>`)

// Options configures Hugo bundle export.
type Options struct {
	// BloggerSiteURL is the live blog base (e.g. https://myblog.blogspot.com) used to resolve
	// relative image paths in HTML (/img/foo.jpg or //lh3.googleusercontent.com/...).
	BloggerSiteURL string
	Concurrency    int
	HTTPTimeout    time.Duration
	Verbose        bool
	Logf           func(format string, args ...interface{})
}

// Stats summarizes an export run.
type Stats struct {
	PostsWritten  int
	PagesWritten  int
	ImagesOK      int
	ImagesFailed  int
	SlugsAdjusted int
}

// Export writes Hugo leaf bundles under siteRoot:
//
//	content/posts/<slug>/index.md + downloaded images
//	content/pages/<slug>/index.md + downloaded images
func Export(siteRoot string, items []model.Content, opts Options) (Stats, error) {
	var st Stats
	if opts.Concurrency < 1 {
		opts.Concurrency = 5
	}
	if opts.HTTPTimeout <= 0 {
		opts.HTTPTimeout = 60 * time.Second
	}
	if opts.Logf == nil {
		opts.Logf = func(string, ...interface{}) {}
	}

	client := &http.Client{Timeout: opts.HTTPTimeout}
	base := strings.TrimSuffix(strings.TrimSpace(opts.BloggerSiteURL), "/")

	postsDir := filepath.Join(siteRoot, "content", "posts")
	pagesDir := filepath.Join(siteRoot, "content", "pages")
	for _, d := range []string{postsDir, pagesDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return st, fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	slugUsed := make(map[string]int)
	sem := make(chan struct{}, opts.Concurrency)

	for _, it := range items {
		kindDir := postsDir
		if it.Kind == model.KindPage {
			kindDir = pagesDir
		}
		slug := uniquifySlug(slugUsed, it.Slug)
		if slug != it.Slug {
			st.SlugsAdjusted++
		}
		bundleDir := filepath.Join(kindDir, slug)
		if err := os.MkdirAll(bundleDir, 0o755); err != nil {
			return st, fmt.Errorf("mkdir bundle %s: %w", bundleDir, err)
		}

		resolved := collectResolvedImageURLs(it.HTML, base)
		urlToName, nOK, nFail := downloadImages(client, sem, bundleDir, resolved, opts)
		st.ImagesOK += nOK
		st.ImagesFailed += nFail

		htmlLocal := replaceImgSrc(it.HTML, urlToName)
		md, err := htmltomarkdown.ConvertString(htmlLocal)
		if err != nil {
			return st, fmt.Errorf("html→md %q: %w", it.Title, err)
		}
		md = strings.TrimSpace(md) + "\n"

		fm, err := frontMatterYAML(it, slug)
		if err != nil {
			return st, err
		}
		body := string(fm) + md

		outPath := filepath.Join(bundleDir, "index.md")
		if err := os.WriteFile(outPath, []byte(body), 0o644); err != nil {
			return st, fmt.Errorf("write %s: %w", outPath, err)
		}
		if it.Kind == model.KindPost {
			st.PostsWritten++
		} else {
			st.PagesWritten++
		}
		if opts.Verbose {
			opts.Logf("wrote %s\n", outPath)
		}
	}

	return st, nil
}

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

func frontMatterYAML(it model.Content, bundleSlug string) ([]byte, error) {
	m := map[string]interface{}{
		"title": it.Title,
		"date":  it.Published.UTC().Format(time.RFC3339),
		"draft": false,
		"slug":  bundleSlug,
	}
	if it.Creator != "" {
		m["author"] = it.Creator
	}
	if len(it.Labels) > 0 {
		tags := append([]string(nil), it.Labels...)
		sort.Strings(tags)
		m["tags"] = tags
	}
	if it.Kind == model.KindPage {
		m["type"] = "page"
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}
	return append([]byte("---\n"), append(out, []byte("---\n\n")...)...), nil
}

// collectResolvedImageURLs returns unique fetchable http(s) URLs in document order.
func collectResolvedImageURLs(html, base string) []string {
	seen := make(map[string]bool)
	var order []string
	for _, m := range imgTagSrcRE.FindAllStringSubmatch(html, -1) {
		if len(m) < 3 {
			continue
		}
		raw := strings.TrimSpace(m[2])
		full := resolveImageURL(raw, base)
		if full == "" {
			continue
		}
		if seen[full] {
			continue
		}
		seen[full] = true
		order = append(order, full)
	}
	return order
}

func resolveImageURL(raw, base string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(strings.ToLower(raw), "data:") {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if base == "" {
		return ""
	}
	if strings.HasPrefix(raw, "/") {
		return base + raw
	}
	if u, err := url.Parse(base); err == nil {
		u2, err := u.Parse(raw)
		if err == nil && (u2.Scheme == "http" || u2.Scheme == "https") {
			return u2.String()
		}
	}
	return ""
}

func replaceImgSrc(html string, urlToFile map[string]string) string {
	if len(urlToFile) == 0 {
		return html
	}
	type pair struct {
		u, f string
	}
	var ps []pair
	for u, f := range urlToFile {
		ps = append(ps, pair{u, f})
	}
	sort.Slice(ps, func(i, j int) bool { return len(ps[i].u) > len(ps[j].u) })

	out := html
	for _, p := range ps {
		if p.f == "" {
			continue
		}
		out = strings.ReplaceAll(out, `src="`+p.u+`"`, `src="`+p.f+`"`)
		out = strings.ReplaceAll(out, `src='`+p.u+`'`, `src='`+p.f+`'`)
	}
	return out
}

func downloadImages(client *http.Client, sem chan struct{}, bundleDir string, urls []string, opts Options) (map[string]string, int, int) {
	urlToFile := make(map[string]string)
	var mu sync.Mutex
	var nOK, nFail int
	var wg sync.WaitGroup

	for i, u := range urls {
		i, u := i, u
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			name := fmt.Sprintf("img-%03d%s", i+1, guessExtFromURL(u))
			destPath := filepath.Join(bundleDir, name)

			finalPath, err := downloadOne(client, u, destPath)
			if err != nil {
				mu.Lock()
				nFail++
				mu.Unlock()
				opts.Logf("image download failed %q: %v\n", u, err)
				return
			}
			rel := filepath.Base(finalPath)
			mu.Lock()
			urlToFile[u] = rel
			nOK++
			mu.Unlock()
		}()
	}
	wg.Wait()
	return urlToFile, nOK, nFail
}

func downloadOne(client *http.Client, srcURL, destPath string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, srcURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "bloggertohugo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 25<<20))
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(destPath)
	if ext == "" {
		if e := extFromContentType(resp.Header.Get("Content-Type")); e != "" {
			destPath = destPath + e
		}
	}
	f, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(body); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return destPath, nil
}

func guessExtFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	e := strings.ToLower(path.Ext(u.Path))
	switch e {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".avif", ".bmp":
		return e
	default:
		return ""
	}
}

func extFromContentType(ct string) string {
	ct = strings.TrimSpace(strings.Split(ct, ";")[0])
	switch strings.ToLower(ct) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "image/avif":
		return ".avif"
	default:
		return ""
	}
}
