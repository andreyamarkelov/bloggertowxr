// Package blogger reads Google Takeout Blogger Atom feeds (feed.atom) and
// produces model.Content values. It filters to LIVE POST and PAGE entries only.
package blogger

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"bloggertowxr/internal/model"
)

// Stats summarizes parsing decisions (useful with -verbose).
type Stats struct {
	PostsImported  int
	PagesImported  int
	SkippedComment int
	SkippedOther   int // unknown blogger:type
	SkippedDraft   int // status != LIVE
}

// atomFeed mirrors the root <feed xmlns="http://www.w3.org/2005/Atom">.
// Child elements from other namespaces use full URIs in struct tags (encoding/xml rule).
type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"http://www.w3.org/2005/Atom title"`
	Entry   []atomEntry `xml:"http://www.w3.org/2005/Atom entry"`
}

// atomEntry is one <entry>: posts, pages, and comments all use this shape; BloggerType discriminates.
type atomEntry struct {
	ID          string         `xml:"http://www.w3.org/2005/Atom id"`
	Title       string         `xml:"http://www.w3.org/2005/Atom title"`
	Published   string         `xml:"http://www.w3.org/2005/Atom published"`
	Content     atomContent    `xml:"http://www.w3.org/2005/Atom content"`
	Categories  []atomCategory `xml:"http://www.w3.org/2005/Atom category"`
	Filename    string         `xml:"http://schemas.google.com/blogger/2018 filename"`
	BloggerType string         `xml:"http://schemas.google.com/blogger/2018 type"`
	Status      string         `xml:"http://schemas.google.com/blogger/2018 status"`
	Author      atomAuthor     `xml:"http://www.w3.org/2005/Atom author"`
}

type atomContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type atomCategory struct {
	Term string `xml:"term,attr"`
}

type atomAuthor struct {
	Name string `xml:"http://www.w3.org/2005/Atom name"`
}

// ParseFeed reads Blogs/<blog>/feed.atom and returns POST and PAGE entries as model.Content.
func ParseFeed(feedPath string) (feedTitle string, items []model.Content, st Stats, err error) {
	f, err := os.Open(feedPath)
	if err != nil {
		return "", nil, st, err
	}
	defer f.Close()
	return ParseReader(f)
}

// ParseReader parses feed.atom from any reader (tests use strings.NewReader).
func ParseReader(r io.Reader) (feedTitle string, items []model.Content, st Stats, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, st, err
	}

	var feed atomFeed
	// Unmarshal loads the whole file into memory; Takeout feeds are usually fine for personal blogs.
	if err := xml.Unmarshal(data, &feed); err != nil {
		return "", nil, st, err
	}

	feedTitle = strings.TrimSpace(feed.Title)

	for _, e := range feed.Entry {
		switch strings.ToUpper(strings.TrimSpace(e.BloggerType)) {
		case "COMMENT":
			st.SkippedComment++
			continue
		case "POST", "PAGE":
			if strings.ToUpper(strings.TrimSpace(e.Status)) != "LIVE" {
				st.SkippedDraft++
				continue
			}
			item, err := atomEntryToContent(e)
			if err != nil {
				return feedTitle, items, st, err
			}
			items = append(items, item)
			if item.Kind == model.KindPost {
				st.PostsImported++
			} else {
				st.PagesImported++
			}
		default:
			st.SkippedOther++
		}
	}

	return feedTitle, items, st, nil
}

// atomEntryToContent maps one Atom entry to our neutral model (HTML unescaped, slug computed).
func atomEntryToContent(e atomEntry) (model.Content, error) {
	kind := model.KindPost
	if strings.ToUpper(strings.TrimSpace(e.BloggerType)) == "PAGE" {
		kind = model.KindPage
	}

	ts, err := time.Parse(time.RFC3339, strings.TrimSpace(e.Published))
	if err != nil {
		return model.Content{}, fmt.Errorf("entry %q: published time: %w", e.ID, err)
	}

	title := strings.TrimSpace(e.Title)
	if title == "" {
		title = "Untitled"
	}

	htmlDecoded := html.UnescapeString(e.Content.Body)

	var labels []string
	for _, c := range e.Categories {
		t := strings.TrimSpace(c.Term)
		if t != "" {
			labels = append(labels, t)
		}
	}

	creator := strings.TrimSpace(e.Author.Name)

	slug := slugFromEntry(strings.TrimSpace(e.Filename), title, e.ID)

	return model.Content{
		Kind:      kind,
		BloggerID: strings.TrimSpace(e.ID),
		Title:     title,
		HTML:      htmlDecoded,
		Labels:    labels,
		Published: ts.UTC(),
		Creator:   creator,
		Slug:      slug,
	}, nil
}

// Non letters/numbers/hyphen → hyphen (Unicode-aware for Cyrillic etc.).
var slugNoise = regexp.MustCompile(`[^\p{L}\p{N}\-]+`)

func slugFromEntry(filename, title, bloggerID string) string {
	if fn := strings.TrimSpace(filename); fn != "" {
		base := path.Base(fn)
		base = strings.TrimSuffix(strings.TrimSuffix(base, ".html"), ".htm")
		base = strings.Trim(base, "/")
		if s := slugify(base); s != "" {
			return s
		}
	}
	if s := slugify(title); s != "" {
		return s
	}
	return fallbackSlug(bloggerID)
}

func slugify(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "_", "-")
	s = slugNoise.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func fallbackSlug(bloggerID string) string {
	const needle = ".post-"
	idx := strings.LastIndex(bloggerID, needle)
	if idx >= 0 {
		return "blogger-post-" + bloggerID[idx+len(needle):]
	}
	return "blogger-imported"
}

// ResolveFeedPath returns the path to feed.atom for the Takeout Blogger folder bloggerRoot.
// blogName must match a subdirectory of Blogs/; if empty and exactly one blog exists, that blog is used.
func ResolveFeedPath(bloggerRoot, blogName string) (resolvedBlog string, feedPath string, err error) {
	blogsDir := filepath.Join(bloggerRoot, "Blogs")
	entries, err := os.ReadDir(blogsDir)
	if err != nil {
		return "", "", fmt.Errorf("read Blogs directory: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		return "", "", fmt.Errorf("no blogs found under %s", blogsDir)
	}

	chosen := strings.TrimSpace(blogName)
	if chosen == "" {
		if len(dirs) != 1 {
			return "", "", fmt.Errorf("multiple blogs in %s — set -blog to one of: %s", blogsDir, strings.Join(dirs, ", "))
		}
		chosen = dirs[0]
	} else {
		found := false
		for _, d := range dirs {
			if d == chosen {
				found = true
				break
			}
		}
		if !found {
			return "", "", fmt.Errorf("blog %q not found under %s (have: %s)", chosen, blogsDir, strings.Join(dirs, ", "))
		}
	}

	feed := filepath.Join(blogsDir, chosen, "feed.atom")
	if _, err := os.Stat(feed); err != nil {
		return "", "", fmt.Errorf("missing feed.atom at %s: %w", feed, err)
	}

	return chosen, feed, nil
}
