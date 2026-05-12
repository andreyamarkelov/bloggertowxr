// Command bloggertohugo converts Google Takeout Blogger (feed.atom) into Hugo
// leaf bundles: Markdown under content/posts and content/pages, with images
// downloaded next to each index.md.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"bloggertohugo/internal/blogger"
	"bloggertohugo/internal/hugo"
)

func main() {
	log.SetFlags(0)

	input := flag.String("input", "", "path to Google Takeout `Blogger` folder (must contain Blogs/)")
	blog := flag.String("blog", "", "blog directory name under Blogs/ (required when multiple blogs exist)")
	output := flag.String("output", "", "Hugo site root (creates content/posts and content/pages)")
	bloggerURL := flag.String("blogger-url", "", "public blog base URL to resolve relative image paths (e.g. https://myblog.blogspot.com)")
	conc := flag.Int("concurrency", 5, "max parallel image downloads per export step")
	timeout := flag.Duration("http-timeout", 60*time.Second, "per-image HTTP client timeout")
	verbose := flag.Bool("verbose", false, "log each written file and image failures to stderr")

	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Convert Blogger Takeout (feed.atom) to Hugo Markdown bundles with local images.\n\n")
		fmt.Fprintf(out, "Usage:\n  bloggertohugo -input <path/to/Blogger> -output <hugo-site-root> [options]\n\n")
		fmt.Fprintf(out, "Example:\n  bloggertohugo -input ./Blogger -blog \"My Blog\" -output ./mysite -blogger-url https://myblog.blogspot.com -verbose\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *input == "" || *output == "" {
		flag.Usage()
		os.Exit(2)
	}

	blogName, feedPath, err := blogger.ResolveFeedPath(*input, *blog)
	if err != nil {
		log.Fatal(err)
	}

	_, items, st, err := blogger.ParseFeed(feedPath)
	if err != nil {
		log.Fatalf("parse %s: %v", feedPath, err)
	}

	opts := hugo.Options{
		BloggerSiteURL: *bloggerURL,
		Concurrency:    *conc,
		HTTPTimeout:    *timeout,
		Verbose:        *verbose,
		Logf:           log.Printf,
	}

	ex, err := hugo.Export(*output, items, opts)
	if err != nil {
		log.Fatal(err)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "blog: %s\nfeed: %s\nparse: posts=%d pages=%d skipped comments=%d drafts=%d other=%d\nexport: posts=%d pages=%d slug-adjusted=%d images_ok=%d images_failed=%d\nroot: %s\n",
			blogName, feedPath,
			st.PostsImported, st.PagesImported, st.SkippedComment, st.SkippedDraft, st.SkippedOther,
			ex.PostsWritten, ex.PagesWritten, ex.SlugsAdjusted, ex.ImagesOK, ex.ImagesFailed,
			*output,
		)
	}
}
