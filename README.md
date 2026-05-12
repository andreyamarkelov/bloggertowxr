# Blogger export tools

This repository holds small **Go** command-line tools that read the same **Google Takeout → Blogger** layout (`Blogger/Blogs/<name>/feed.atom`) and convert **live posts and static pages** (comments are skipped).

| Tool | Output | Documentation |
|------|--------|----------------|
| [**bloggertowxr**](bloggertowxr/) | WordPress **WXR** XML (remote image URLs in HTML; optional attachment rows) | [bloggertowxr/README.md](bloggertowxr/README.md), [bloggertowxr/docs/README.md](bloggertowxr/docs/README.md) |
| [**bloggertohugo**](bloggertohugo/) | **[Hugo](https://gohugo.io/)** Markdown **leaf bundles** under `content/posts` and `content/pages`, with **images downloaded** next to each `index.md` | [bloggertohugo/README.md](bloggertohugo/README.md), [bloggertohugo/docs/README.md](bloggertohugo/docs/README.md) |

## Requirements

- [Go](https://go.dev/dl/) **1.22+** (each module pins `go 1.22` in its own `go.mod`).

## Repository layout

```text
.
├── .github/           # GitHub metadata (e.g. Dependabot for both Go modules)
├── .gitignore         # Monorepo-wide ignores (binaries, Takeout, generated XML, …)
├── bloggertowxr/      # Go module: Blogger → WordPress WXR
└── bloggertohugo/     # Go module: Blogger → Hugo bundles
```

Each tool is a **separate Go module**; build and test from its directory (or use `go build -C <dir> …`).

## Quick start

1. Export from [Google Takeout](https://takeout.google.com/) and unzip so you have a **`Blogger`** folder containing **`Blogs/`**.

2. **WordPress / Publii (WXR):**

```bash
cd bloggertowxr
go build -o bloggertowxr ./cmd/bloggertowxr
./bloggertowxr -input /path/to/Blogger -output export.xml -site-url https://your-site.example
```

3. **Hugo:**

```bash
cd bloggertohugo
go build -o bloggertohugo ./cmd/bloggertohugo
./bloggertohugo -input /path/to/Blogger -output /path/to/hugo-site -blogger-url https://your-public-blog.example
```

If Takeout has **more than one** blog under `Blogs/`, both tools accept **`-blog "Exact Folder Name"`** (same spelling as on disk).

## Tests

```bash
cd bloggertowxr && go test ./... && cd ..
cd bloggertohugo && go test ./... && cd ..
```

Or from the repository root:

```bash
go test -C bloggertowxr ./...
go test -C bloggertohugo ./...
```

## Developer guides

For a **line-by-line, beginner-oriented** tour of the Go packages (parsers, writers, tests), see each tool’s **`docs/README.md`** (linked in the table above).
