// Command bloggertowxr converts a Google Takeout Blogger folder (Atom feed.atom)
// into a WordPress Extended RSS (WXR) XML file. See README.md for usage.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"bloggertowxr/internal/blogger"
	"bloggertowxr/internal/wxr"
)

func main() {
	// Plain log lines (no default timestamp prefix).
	log.SetFlags(0)

	// --- Command-line flags (stdlib flag package; pointers until Parse()) ---
	input := flag.String("input", "", "path to Google Takeout `Blogger` folder (must contain Blogs/)")
	blog := flag.String("blog", "", "blog directory name under Blogs/ (required when multiple blogs exist)")
	output := flag.String("output", "", "write WordPress WXR XML to this file")
	siteURL := flag.String("site-url", "https://example.com", "base URL for the WordPress site (guid/link prefix)")
	skipAtt := flag.Bool("skip-remote-attachments", false, "omit WXR attachment rows for remote <img> URLs (Publii and similar tools count images from those rows)")
	verbose := flag.Bool("verbose", false, "print import counts and skips")

	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Convert Blogger Takeout (feed.atom) to WordPress WXR (posts and pages; comments omitted).\n\n")
		fmt.Fprintf(out, "Usage:\n  bloggertowxr -input <path/to/Blogger> -output <export.xml> [options]\n\n")
		fmt.Fprintf(out, "Example:\n  bloggertowxr -input ./Takeout/Blogger -blog \"Andrey_s Blog\" -output ./wxr.xml -site-url https://mysite.example\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Minimum required arguments.
	if *input == "" || *output == "" {
		flag.Usage()
		os.Exit(2)
	}

	// Resolve Blogs/<name>/feed.atom (validates folder layout).
	blogName, feedPath, err := blogger.ResolveFeedPath(*input, *blog)
	if err != nil {
		log.Fatal(err)
	}

	// Atom → []model.Content (posts + pages only).
	feedTitle, items, st, err := blogger.ParseFeed(feedPath)
	if err != nil {
		log.Fatalf("parse %s: %v", feedPath, err)
	}

	// Truncate/create output path (user should use .gitignored names like export.xml for local runs).
	out, err := os.Create(*output)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	opts := wxr.Options{
		SiteTitle:           chooseSiteTitle(feedTitle, blogName),
		SiteURL:             *siteURL,
		SkipAttachmentItems: *skipAtt,
	}
	if err := wxr.Write(out, opts, items); err != nil {
		log.Fatal(err)
	}

	if err := out.Sync(); err != nil {
		log.Fatal(err)
	}

	if *verbose {
		tagN, imgN := wxr.PreviewCounts(items)
		fmt.Fprintf(os.Stderr, "blog: %s\nfeed: %s\nposts: %d\npages: %d\nskipped comments: %d\nskipped non-live: %d\nskipped other type: %d\ndistinct tags (channel wp:tag): %d\ndistinct remote image URLs (attachment items): %d\nwritten: %s\n",
			blogName, feedPath, st.PostsImported, st.PagesImported, st.SkippedComment, st.SkippedDraft, st.SkippedOther, tagN, imgN, *output)
		if *skipAtt {
			fmt.Fprintf(os.Stderr, "(attachment items omitted; use default behavior so analyzers count images)\n")
		}
	}
}

// chooseSiteTitle picks the RSS/ channel <title>: feed title from Atom, else blog folder name.
func chooseSiteTitle(feedTitle, blogDir string) string {
	if t := strings.TrimSpace(feedTitle); t != "" {
		return t
	}
	return strings.TrimSpace(blogDir)
}
