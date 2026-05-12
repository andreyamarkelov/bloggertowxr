# Developer guide: how `bloggertohugo` works

This is the **long, beginner-friendly** explanation of the project. For “how do I run it?”, see the tool directory [`README.md`](../README.md).

**Assumed background:** you know what a **variable**, **function**, and **package** are in any language. You do **not** need to know XML, Blogger, or Hugo internals beforehand.

---

## Table of contents

1. [What problem this tool solves](#1-what-problem-this-tool-solves)
2. [Vocabulary (glossary)](#2-vocabulary-glossary)
3. [End-to-end data flow](#3-end-to-end-data-flow)
4. [Go module and folder layout](#4-go-module-and-folder-layout)
5. [Command-line program: `cmd/bloggertohugo/main.go`](#5-command-line-program-cmdbloggertohugomaingo)
6. [Shared data model: `internal/model/content.go`](#6-shared-data-model-internalmodelcontentgo)
7. [Blogger Atom parser: `internal/blogger/parse.go`](#7-blogger-atom-parser-internalbloggerparsego)
8. [Hugo export: `internal/hugo/export.go`](#8-hugo-export-internalhugoexportgo)
9. [Tests: `*_test.go` files](#9-tests-_testgo-files)
10. [Ideas for extending the tool](#10-ideas-for-extending-the-tool)
11. [Further reading](#11-further-reading)

---

## 1. What problem this tool solves

**Google Blogger** lets you export your site via **Google Takeout**. The useful text for migration usually lives in **`Blogs/<Blog name>/feed.atom`**, an **Atom** XML feed where `<entry>` elements mix **posts**, **static pages**, and **comments**.

**[Hugo](https://gohugo.io/)** is a static site generator that reads **Markdown** (and other formats) under a `content/` tree. A common pattern is a **leaf bundle**: one folder contains **`index.md`** plus **assets** (images) referenced with **relative** paths.

This program **reads** the same Takeout layout as [`bloggertowxr`](../bloggertowxr/README.md) (sibling tool), **filters** LIVE posts and pages, **downloads** remote images found in HTML, **rewrites** `<img src>` to local filenames, converts HTML to **Markdown**, and writes **YAML front matter** + body into bundle folders.

---

## 2. Vocabulary (glossary)

| Term | Meaning here |
|------|----------------|
| **Takeout** | Google’s “download your data” export; you unzip it and point `-input` at the **`Blogger`** folder inside. |
| **Atom** | XML feed format (`<feed>`, `<entry>`, …). Blogger adds tags in a **Blogger** namespace (`blogger:type`, `blogger:filename`, …). |
| **`feed.atom`** | The main file the parser reads. |
| **Leaf bundle** | A content directory containing **`index.md`** plus sibling files (here: downloaded images). Hugo treats the folder as one logical page. |
| **Front matter** | YAML between `---` lines at the top of `index.md`; sets `title`, `date`, `tags`, etc. |
| **WXR** | Not used here; the sibling project `bloggertowxr` writes WordPress WXR instead. |

---

## 3. End-to-end data flow

Think of the program as a **pipeline**:

```text
  Takeout folder
        │
        ▼
  Resolve path ──►  Blogs/<name>/feed.atom
        │
        ▼
  Parse Atom XML ──►  []model.Content   (only LIVE POST + PAGE)
        │
        ▼
  For each item:
        ├── collect <img src> URLs → resolve to https (optional -blogger-url)
        ├── download images → bundle/img-NNN.ext
        ├── rewrite HTML src= to local filenames
        ├── HTML → Markdown (library)
        └── write bundle/index.md (YAML + Markdown)
```

- **Input** is a **directory** (`-input`), not the `.atom` file path.
- **Output** is a **Hugo site root** (`-output`); the writer creates **`content/posts/<slug>/`** and **`content/pages/<slug>/`**.

Between parse and export, everything is normal Go data (`structs`, `slices`, `time.Time`). The **`model.Content`** type is the hand-off contract, same idea as in `bloggertowxr`.

---

## 4. Go module and folder layout

The module name is **`bloggertohugo`** (see `go.mod`). Imports look like:

```text
bloggertohugo/internal/blogger
bloggertohugo/internal/hugo
bloggertohugo/internal/model
```

| Path | Role |
|------|------|
| `cmd/bloggertohugo/main.go` | **Executable entry point** — flags, calls `blogger` + `hugo.Export`. |
| `internal/model/content.go` | **One struct** describing one post or page after parsing. |
| `internal/blogger/parse.go` | **Read** `feed.atom` → `[]model.Content` (mirrors `bloggertowxr`’s parser pattern). |
| `internal/hugo/export.go` | **Write** bundles: downloads, rewrite, Markdown, `index.md`. |
| `internal/hugo/markdown_postprocess.go` | Post-process Markdown (e.g. unwrap CDN links around local images). |
| `internal/blogger/parse_test.go` | Atom parser tests (inline fixture XML + tempdir layout). |
| `internal/hugo/export_test.go` | URL helpers + export against `httptest` image server. |

**Why `internal/`?** Same as in `bloggertowxr`: code there is **private to this module** and not meant as a reusable public library for other modules.

**Why `cmd/`?** One folder per buildable **main** program.

---

## 5. Command-line program: `cmd/bloggertohugo/main.go`

### Package `main`

Only packages named **`main`** with **`func main()`** build to standalone binaries.

### Imports

- **`flag`** — `-input`, `-output`, `-blog`, `-blogger-url`, `-concurrency`, `-http-timeout`, `-verbose`.
- **`log`** — `log.Fatal` for hard errors; **`log.Printf`** is passed into `hugo.Options.Logf` so **failed image downloads** always produce a log line on stderr (even when `-verbose` is false).
- **`time`** — default duration for `-http-timeout`.
- **`bloggertohugo/internal/blogger`** and **`.../hugo`** — project packages.

### Pointers from `flag.String` / `flag.Int`

`flag.String("input", "", "...")` returns `*string`. After `flag.Parse()`, read values with **`*input`**.

### Order of operations in `main()` (high level)

1. **`log.SetFlags(0)`** — short log lines (no default date prefix).
2. **Register flags** and custom **`flag.Usage`** with a short description + example.
3. **`flag.Parse()`** — read CLI args.
4. **Validate** `-input` and `-output`; if missing, print usage and **`os.Exit(2)`**.
5. **`ResolveFeedPath`** — same rules as `bloggertowxr`: discover `Blogs/…/feed.atom`.
6. **`ParseFeed`** — returns `(feedTitle, items, stats, error)`. The feed title is currently unused (reserved for future site title hints).
7. **Build `hugo.Options`** — blogger base URL, concurrency, HTTP timeout, verbose, and **`Logf: log.Printf`**.
8. **`hugo.Export(*output, items, opts)`** — creates directories and files under the Hugo root.
9. **If `-verbose`** — prints a **summary** block to stderr (parse stats + export stats + paths context).

---

## 6. Shared data model: `internal/model/content.go`

### Why a separate `model` package?

So **`blogger`** does not import **`hugo`** and **`hugo`** does not import **`blogger`**. Both depend on **`model`** only (same layering idea as `bloggertowxr`).

### `Kind`

`KindPost` vs `KindPage` — drives whether output goes under **`content/posts`** or **`content/pages`**.

### `Content` struct (field by field)

| Field | Meaning |
|-------|---------|
| `Kind` | Post vs page. |
| `BloggerID` | Atom entry `id` (stable URI string). |
| `Title` | Title; empty titles become `"Untitled"` in the parser. |
| `HTML` | Decoded HTML body (after `html.UnescapeString`). |
| `Labels` | Blogger categories → become Hugo **`tags`**. |
| `Published` | `time.Time` (UTC in practice). |
| `Creator` | Author display name → optional front matter **`author`**. |
| `Slug` | URL slug from filename/title/fallback; may be **uniquified** again during export if two items collide. |

---

## 7. Blogger Atom parser: `internal/blogger/parse.go`

This file follows the **same approach** as `bloggertowxr/internal/blogger/parse.go`:

- **`encoding/xml`** unmarshals Atom + Blogger-namespace fields via struct tags with full namespace URIs.
- **`ParseReader` / `ParseFeed`** filter **`COMMENT`**, non-**`LIVE`** entries, and unknown `blogger:type` values.
- **`atomEntryToContent`** builds `model.Content` with slug helpers **`slugFromEntry`**, **`slugify`**, **`fallbackSlug`**.
- **`ResolveFeedPath`** validates **`Blogs/`**, single vs multiple blog folders, and the presence of **`feed.atom`**.

If you fix a parsing bug, consider **porting the same fix** to `bloggertowxr` so the two tools stay aligned.

---

## 8. Hugo export: `internal/hugo/export.go`

### Dependencies (why not only the standard library?)

- **`github.com/JohannesKaufmann/html-to-markdown/v2`** — converts HTML bodies to Markdown (`ConvertString`). The wrapper registers the library’s default plugins.
- **`gopkg.in/yaml.v3`** — marshals front matter maps to YAML text.

### `Options`

| Field | Role |
|-------|------|
| `BloggerSiteURL` | Strip trailing `/`; used by **`resolveImageURL`** to turn `/path` or relative paths into absolute `https://…` download URLs. |
| `Concurrency` | Bounded parallelism via a **buffered channel** used as a semaphore (`sem <- struct{}{}` … `defer func() { <-sem }()`). |
| `HTTPTimeout` | Sets `http.Client.Timeout` for the whole client (per-request ceiling). |
| `Verbose` | Logs each successfully written `index.md` path. |
| `Logf` | printf-style logger; **`main`** wires **`log.Printf`** so download errors always surface. |

### `Export` loop (per `model.Content`)

1. Choose **`content/posts`** vs **`content/pages`** from `Kind`.
2. **`uniquifySlug`** — global `map` across **all** items so post and page slugs never collide on disk (`slug`, `slug-2`, …).
3. **`collectResolvedImageURLs`** — regex scan for `<img … src="…">` (same **idea** as `bloggertowxr`’s `imgTagSrcRE`, extended to capture full tags for future use). Deduplicate URLs while preserving order.
4. **`resolveImageURL`** — supports `http(s):`, `//host`, root-relative with base, and path-relative resolved with `url.Parse(base).Parse(raw)`.
5. **`downloadImages`** — goroutine per URL (up to **Concurrency**). Filenames are **`img-001` + optional extension** from the URL path; if missing, **`Content-Type`** may append an extension after the GET.
6. **`removeImgTagsForFailedDownloads`** — removes entire `<img …>` tags whose URL was attempted but **not** present in `urlToFile` (download error), so failed assets do not appear in Markdown.
7. **`removeEmptyAnchorTags`** — removes `<a …></a>` that only contain whitespace (common after stripping an image that lived inside a Blogger anchor).
8. **`replaceImgSrc`** — string replace on `src="URL"` / `src='URL'`; sorts URLs **longest first** to reduce accidental partial replacement edge cases.
9. **`htmltomarkdown.ConvertString`** — Markdown body.
10. **`stripMarkdownImageLinkWrappers`** (`markdown_postprocess.go`) — removes Markdown of the form `[![](local.jpg)](https://blogger.googleusercontent.com/…)` so only `![](local.jpg)` remains (typical when `<a><img></a>` pointed at the CDN while `src` was rewritten to a bundle file).
11. **`frontMatterYAML`** — builds a `map[string]interface{}` with `title`, `date` (RFC3339), `draft: false`, `slug` (**bundle** slug), optional `author`, optional sorted `tags`, and **`type: page`** for pages.
12. **`os.WriteFile`** — `index.md` with mode `0644`.

### Safety / limits

- Each image response body is capped with **`io.LimitReader(..., 25<<20)`** (25 MiB) to avoid accidental huge downloads.

---

## 9. Tests: `*_test.go` files

Go discovers tests in files ending with **`_test.go`**.

- **`internal/blogger/parse_test.go`** — Inline minimal Atom (copied from the `bloggertowxr` fixture style) verifies posts/pages/comments skipping and **`ResolveFeedPath`** with `t.TempDir()`.
- **`internal/hugo/export_test.go`** — Unit tests for **`resolveImageURL`** / **`replaceImgSrc`** / **`removeImgTagsForFailedDownloads`** / **`stripMarkdownImageLinkWrappers`**, plus an integration-style test that serves a tiny “image” from **`httptest.Server`**, runs **`Export`**, and asserts the generated Markdown references a local **`img-001`** filename.

Run from this module’s directory:

```bash
go test ./...
```

---

## 10. Ideas for extending the tool

These are **not** implemented here; they are learning exercises:

- **Include drafts** — Similar to a hypothetical `bloggertowxr` flag: stop skipping `status != LIVE` and set `draft: true` in front matter.
- **Comments** — Parse `COMMENT` entries into Hugo `content` or `data` (threading is non-trivial).
- **`srcset` / `<picture>`** — Extend URL extraction beyond a single `src` attribute.
- **Background images** — Parse `style="background-image:url(...)"`.
- **Archetypes / sections** — Emit different paths or taxonomies based on labels (e.g. categories vs tags).
- **Goldmark unsafe HTML** — Keep select HTML blocks instead of full conversion.

Each change usually touches **one** of: `blogger`, `model`, `hugo`, or `main` (new flags).

---

## 11. Further reading

- [Hugo content management](https://gohugo.io/content-management/) — official overview of sections and bundles.
- [Hugo page bundles](https://gohugo.io/content-management/page-bundles/) — why `index.md` + co-located assets work well.
- [Go encoding/xml](https://pkg.go.dev/encoding/xml) — unmarshalling details used in `internal/blogger`.
- [html-to-markdown v2](https://github.com/JohannesKaufmann/html-to-markdown) — converter behavior and plugins.

---

*If anything in this guide drifts from the code, trust the source files and tests; this document is meant to teach intent, not replace reading the implementation.*
