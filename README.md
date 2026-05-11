# bloggertowxr

Small command-line tool: **Google Blogger Takeout** (unzipped) → **WordPress WXR** XML. Imports **posts** and **static pages**; **comments** are skipped.

## Requirements

- [Go](https://go.dev/dl/) 1.22 or newer (module uses `go 1.22`).

## Quick start

1. Download your blog from [Google Takeout](https://takeout.google.com/) and unzip it. You need the **`Blogger`** folder (it must contain **`Blogs/`**).

2. From this repository:

```bash
go build -o bloggertowxr ./cmd/bloggertowxr
./bloggertowxr -input /path/to/Blogger -output export.xml -site-url https://your-site.example
```

- If Takeout contains **more than one** blog under `Blogs/`, add **`-blog "Exact Folder Name"`** (same spelling as on disk).

3. Import the XML in **WordPress** (Tools → Import) or **Publii** (Tools → WP Import).

## Flags

| Flag | Required | Meaning |
|------|------------|---------|
| `-input` | yes | Path to the Takeout **`Blogger`** directory (contains `Blogs/`) |
| `-output` | yes | Path to write the `.xml` file |
| `-blog` | if multiple blogs | Subfolder name under `Blogs/` |
| `-site-url` | no | Base URL for `guid` / `link` (default `https://example.com`) |
| `-skip-remote-attachments` | no | Omit extra `<item>` rows for remote images (smaller file; [Publii](https://getpublii.com/)’s image count may show 0) |
| `-verbose` | no | Print counts (posts, pages, skips, tags, image URLs) to stderr |

Example with verbose logging:

```bash
./bloggertowxr -input ./Blogger -blog "Andrey_s Blog" -output export.xml -site-url https://example.com -verbose
```

## What gets imported

- **Posts** (`blogger:type` POST) and **pages** (PAGE) with status **LIVE**.
- **Labels** → WordPress tags (`<wp:tag>` in the channel + `<category domain="post_tag">` on each item).
- **Images**: unique `http(s)` URLs from `<img src="...">` in HTML get companion **attachment** `<item>` rows (for tools like Publii that count media that way). Images are **not** downloaded; URLs stay remote unless your importer fetches them.

## Developer docs

See **`docs/README.md`** for a detailed, novice-oriented walkthrough of the Go packages and how data flows from `feed.atom` to WXR.

## Tests

```bash
go test ./...
```

## What stays out of Git

The **`Blogger/`** Takeout tree (feeds, albums, profile CSVs) is **not** tracked: it is large and personal. Keep your export next to the clone or anywhere on disk and pass **`-input`** to that path. The built binary **`bloggertowxr`** and generated **`export.xml`** (and similar) are listed in **`.gitignore`** as well.

