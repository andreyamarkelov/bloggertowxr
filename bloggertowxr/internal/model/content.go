// Package model holds the shared shape of one blog entry after Atom parsing
// and before WXR serialization. Keeping this small avoids coupling XML details
// across packages.
package model

import "time"

// Kind is Blogger entry type mapped to WordPress wp:post_type.
type Kind string

const (
	KindPost Kind = "post" // Blogger POST → WordPress post
	KindPage Kind = "page" // Blogger PAGE → WordPress page
)

// Content is one imported Blogger POST or PAGE entry (comments never become Content).
type Content struct {
	Kind      Kind
	BloggerID string   // Atom <id> (stable Blogger URI)
	Title     string   // Post/page title; empty titles become "Untitled" in blogger package
	HTML      string   // Decoded HTML for WXR content:encoded (CDATA in output)
	Labels    []string // Blogger <category term="...">; become WordPress tags
	Published time.Time
	Creator   string // Atom author name → dc:creator
	Slug      string // wp:post_name (URL slug); may be uniquified again in wxr package
}
