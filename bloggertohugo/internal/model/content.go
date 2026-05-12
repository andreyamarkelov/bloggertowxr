// Package model holds one Blogger POST or PAGE after Atom parsing.
package model

import "time"

// Kind is Blogger entry type.
type Kind string

const (
	KindPost Kind = "post"
	KindPage Kind = "page"
)

// Content is one imported Blogger POST or PAGE (comments are never Content).
type Content struct {
	Kind      Kind
	BloggerID string
	Title     string
	HTML      string
	Labels    []string
	Published time.Time
	Creator   string
	Slug      string
}
