# bloggertohugo

Small command-line tool: **Google Blogger Takeout** (unzipped) → **[Hugo](https://gohugo.io/)** content as **Markdown leaf bundles**. Imports **posts** and **static pages**; **comments** are skipped. **Images** referenced from `<img src="...">` are **downloaded** into each bundle next to `index.md`, and the Markdown points at those **local** files when the download succeeds.

## Requirements

- [Go](https://go.dev/dl/) 1.22 or newer (module uses `go 1.22`).

## Quick start

1. Download your blog from [Google Takeout](https://takeout.google.com/) and unzip it. You need the **`Blogger`** folder (it must contain **`Blogs/`**).

2. From this directory:

```bash
go build -o bloggertohugo ./cmd/bloggertohugo
./bloggertohugo -input /path/to/Blogger -output /path/to/hugo-site -blogger-url https://your-public-blog.example
```

- If Takeout contains **more than one** blog under `Blogs/`, add **`-blog "Exact Folder Name"`** (same spelling as on disk).
- Use **`-blogger-url`** with the **live** site base when post HTML uses **root-relative** or **path-relative** image URLs (for example `/img/foo.jpg`). Full `https://…` image URLs in the HTML work without it.

3. Open your Hugo site root in a terminal and run **`hugo`** (or integrate the generated `content/posts` and `content/pages` trees into an existing site).

## Flags

| Flag | Required | Meaning |
|------|----------|---------|
| `-input` | yes | Path to the Takeout **`Blogger`** directory (contains `Blogs/`) |
| `-output` | yes | Hugo site root; the tool creates **`content/posts/`** and **`content/pages/`** under this path |
| `-blog` | if multiple blogs | Subfolder name under `Blogs/` |
| `-blogger-url` | no | Public blog base URL to resolve **relative** `<img src>` values (e.g. `https://myblog.blogspot.com`) |
| `-concurrency` | no | Max parallel image downloads (default `5`) |
| `-http-timeout` | no | HTTP client timeout **per image** request (default `60s`) |
| `-verbose` | no | Print per-written `index.md` paths and an end-of-run summary to stderr |

Example with verbose logging:

```bash
./bloggertohugo -input ./Blogger -blog "Andrey_s Blog" -output ./mysite -blogger-url https://example.blogspot.com -verbose
```

## What gets imported

- **Posts** (`blogger:type` POST) and **pages** (PAGE) with status **LIVE**.
- **Labels** → Hugo front matter **`tags:`** (sorted).
- **Author** name from Atom → front matter **`author:`** when present.
- **Images**: `http://`, `https://`, and protocol-relative `//…` URLs in `<img src="…">` are fetched and saved as `img-001.ext`, … beside **`index.md`**. If a download **fails**, that `<img>` is **dropped** from the HTML before Markdown is generated, and empty **`<a>…</a>`** wrappers left behind are removed so CDN links do not linger. After conversion, **link-wrapped images** like `[![](img-001.jpg)](https://blogger.googleusercontent.com/…)` are reduced to **`![](img-001.jpg)`** so old CDN URLs are not kept as link targets.

## Output layout (Hugo leaf bundles)

| Kind | Directory | Typical URL (depends on your `hugo.toml`) |
|------|-----------|---------------------------------------------|
| Post | `content/posts/<slug>/index.md` + images | often `/posts/<slug>/` |
| Page | `content/pages/<slug>/index.md` + images | often `/pages/<slug>/` |

Pages include front matter **`type: page`**.

## Developer docs

See **`docs/README.md`** for a detailed, novice-oriented walkthrough of the Go packages and how data flows from `feed.atom` to Markdown bundles.

## Tests

```bash
go test ./...
```
