# Developer guide: how `bloggertowxr` works

This is the **long, beginner-friendly** explanation of the project. For “how do I run it?”, see the repository root [`README.md`](../README.md).

**Assumed background:** you know what a **variable**, **function**, and **package** are in any language. You do **not** need to know XML, Blogger, or WordPress internals beforehand.

---

## Table of contents

1. [What problem this tool solves](#1-what-problem-this-tool-solves)
2. [Vocabulary (glossary)](#2-vocabulary-glossary)
3. [End-to-end data flow](#3-end-to-end-data-flow)
4. [Go module and folder layout](#4-go-module-and-folder-layout)
5. [Command-line program: `cmd/bloggertowxr/main.go`](#5-command-line-program-cmdbloggertowxrmaingo)
6. [Shared data model: `internal/model/content.go`](#6-shared-data-model-internalmodelcontentgo)
7. [Blogger Atom parser: `internal/blogger/parse.go`](#7-blogger-atom-parser-internalbloggerparsego)
8. [WXR writer: `internal/wxr/write.go`](#8-wxr-writer-internalwxrwritego)
9. [Tests: `*_test.go` files](#9-tests-_testgo-files)
10. [Ideas for extending the tool](#10-ideas-for-extending-the-tool)
11. [Further reading](#11-further-reading)

---

## 1. What problem this tool solves

**Google Blogger** lets you export your site via **Google Takeout**. That download is a folder of files, not a single “WordPress file.” The important blog text usually lives in **`Blogs/<Blog name>/feed.atom`**, which is an **Atom** XML feed: one file contains many `<entry>` elements—some are **posts**, some **static pages**, some **comments**, etc.

**WordPress** (and tools like **Publii** that import WordPress backups) often expect a **WXR** file: an XML document that looks like RSS but includes WordPress-specific tags (`wp:post_type`, `content:encoded`, and so on).

This program **reads** the Takeout layout, **filters** the entries you care about, and **writes** one `.xml` file you can import elsewhere.

---

## 2. Vocabulary (glossary)

| Term | Meaning here |
|------|----------------|
| **Takeout** | Google’s “download your data” export; you unzip it and point `-input` at the **`Blogger`** folder inside. |
| **Atom** | A standard XML format for feeds (`<feed>`, `<entry>`, …). Blogger’s export uses Atom plus extra tags in a **Blogger** namespace. |
| **Namespace** | XML tags can belong to a “family” identified by a URL string (the namespace URI). The same local name `type` can exist in different families; the URI tells them apart. |
| **`feed.atom`** | The main file this tool reads. It mixes posts, pages, and comments in one list of `<entry>` elements. |
| **WXR** | “WordPress eXtended RSS” — an XML interchange format for posts, pages, tags, sometimes media metadata. |
| **`wp:post_type`** | Inside WXR `<item>`, tells WordPress what kind of row this is: `post`, `page`, `attachment`, etc. |
| **CDATA** | A way to put raw text (including `<` and `>`) inside XML without breaking the parser. We wrap HTML post bodies in CDATA. |

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
  Write WXR XML  ──►  your -output file
```

- **Input** is a **directory** (`-input`), not the `.atom` file path. The code discovers `feed.atom` under `Blogs/`.
- **Output** is always a **single file** path (`-output`).

Between parse and write, everything is normal Go data (`structs`, `slices`, `time.Time`). That middle layer (`model.Content`) is what makes the code easier to follow: **parsing** and **WXR formatting** do not know about each other’s details.

---

## 4. Go module and folder layout

The module name is **`bloggertowxr`** (see `go.mod`). Imports look like:

```text
bloggertowxr/internal/blogger
bloggertowxr/internal/wxr
bloggertowxr/internal/model
```

| Path | Role |
|------|------|
| `cmd/bloggertowxr/main.go` | **Executable entry point** — parses CLI flags, calls blogger + wxr. |
| `internal/model/content.go` | **One struct** describing one post or page after parsing. |
| `internal/blogger/parse.go` | **Read** `feed.atom` → `[]model.Content`. |
| `internal/wxr/write.go` | **Write** `[]model.Content` → WXR bytes. |
| `internal/blogger/parse_test.go` | Tests for the Atom parser (small fake XML strings). |
| `internal/wxr/write_test.go` | Tests for WXR output shape. |

**Why `internal/`?** In Go, anything under a directory named `internal` can only be imported by code **inside this module**. That signals “library for this app, not a public API for other projects.”

**Why `cmd/`?** Convention: each subfolder of `cmd/` is one **main program** you can build with `go build ./cmd/...`.

---

## 5. Command-line program: `cmd/bloggertowxr/main.go`

### Package `main`

Only packages named **`main`** with a function **`func main()`** can be built as standalone executables.

### Imports

- **`flag`** — defines `-input`, `-output`, etc., and parses `os.Args`.
- **`log`** — `log.Fatal` prints a message and exits the process with a non-zero status.
- **`os`** — file creation, stderr, exit codes.
- **`strings`** — trim whitespace for titles.
- **`bloggertowxr/internal/blogger`** and **`.../wxr`** — your project packages.

### Pointers from `flag.String` / `flag.Bool`

`flag.String("input", "", "...")` returns `*string` (pointer to string), not a string. After `flag.Parse()`, you read the value with **`*input`**. Newcomers often forget the `*` and get a type error or wrong behavior.

### Order of operations in `main()` (conceptually line by line)

1. **`log.SetFlags(0)`** — Removes the default date/time prefix from log lines so errors look short.
2. **Register flags** — Each flag has a name, default, and help string.
3. **`flag.Usage = ...`** — Replaces the default `-h` text with a short description + example.
4. **`flag.Parse()`** — Fills in flag values from the command line.
5. **Validate** `-input` and `-output` are non-empty; if not, print usage and **`os.Exit(2)`** (common convention: 2 = misuse).
6. **`ResolveFeedPath`** — Validates the folder tree; returns `(blogFolderName, fullPathToFeedAtom, error)`.
7. **`ParseFeed`** — Reads `feed.atom`; returns `(feedTitle, items, stats, error)`.
8. **`os.Create(*output)`** — Creates or **truncates** the output file (overwrites previous export).
9. **`defer out.Close()`** — Schedules `Close` when `main` **returns normally**. Note: **`log.Fatal` calls `os.Exit`**, which **does not run deferred functions**, so if you add more `Fatal` calls after `defer out.Close()`, the file might not be closed via that defer (the OS still cleans up when the process exits).
10. **Build `wxr.Options`** — Site title from feed or folder name, site URL, attachment toggle.
11. **`wxr.Write`** — Streams the full XML document.
12. **`out.Sync()`** — Flushes buffers to disk (best effort).
13. **If `-verbose`** — Prints human-readable counts to **stderr** (`os.Stderr`) so you can still redirect **stdout** or the output file without mixing debug text into the XML file.

### `chooseSiteTitle`

Small helper: prefer the Atom `<title>` (blog title), else fall back to the blog directory name so the WXR `<channel><title>` is never empty.

---

## 6. Shared data model: `internal/model/content.go`

### Why a separate `model` package?

So **`blogger`** does not import **`wxr`** and **`wxr`** does not import **`blogger`**. Both depend on **`model`** only. That keeps dependency direction clear: **parse → model → write**.

### `Kind`

A string type with two constants, `KindPost` and `KindPage`. These strings match WordPress’s `wp:post_type` values.

### `Content` struct (field by field)

| Field | Meaning |
|-------|---------|
| `Kind` | Post vs page. |
| `BloggerID` | The Atom entry’s unique `id` string (useful for logs or future deduplication). |
| `Title` | Post/page title. |
| `HTML` | Full post body as **decoded** HTML (not Atom-escaped). The WXR layer will wrap it in CDATA. |
| `Labels` | Slice of label strings from Atom `<category term="..."/>`. |
| `Published` | When the entry was published, as `time.Time` (stored in UTC in practice). |
| `Creator` | Author display name for `dc:creator`. |
| `Slug` | URL slug (`wp:post_name`). May still be adjusted again in `wxr` if duplicates appear. |

---

## 7. Blogger Atom parser: `internal/blogger/parse.go`

### XML unmarshalling in Go

`encoding/xml` can fill Go structs from XML **if** struct field tags describe where each field comes from. Blogger mixes **Atom** and **Blogger** namespaces, so tags in the struct look verbose, for example:

```go
Title string `xml:"http://www.w3.org/2005/Atom title"`
```

That means: “when you see a `<title>` element in the Atom namespace under this entry, put its text here.”

### Main types

- **`atomFeed`** — Represents `<feed>…</feed>`: has `Title` and a slice of `Entry`.
- **`atomEntry`** — One `<entry>`. The field **`BloggerType`** holds values like `POST`, `PAGE`, `COMMENT`.
- **`atomContent`** — The `<content type="html">…</content>` body; `Body` is filled from character data inside the tag.
- **`atomCategory`** — `term` attribute → one label.
- **`atomAuthor`** — Nested `<author><name>`.

### `ParseReader` logic (the heart of filtering)

1. Read **all** bytes from the reader (`io.ReadAll`).
2. **`xml.Unmarshal`** into `atomFeed`. If the XML is malformed, return an error.
3. Loop **`for _, e := range feed.Entry`**:
   - **`COMMENT`** — Count as skipped comment, do not append to `items`.
   - **`POST` or `PAGE`** — If `status` is not **`LIVE`**, count as skipped draft and skip. (You could later add a flag to include drafts.)
   - **`POST` or `PAGE` + LIVE** — Convert to `model.Content`, append, increment post or page counter.
   - **Anything else** — Increment `SkippedOther` (unknown `blogger:type`).

### `atomEntryToContent`

- Chooses **`KindPage`** if type is PAGE, else post.
- Parses **`Published`** with `time.Parse(time.RFC3339, ...)`.
- Replaces empty titles with **`"Untitled"`**.
- Runs **`html.UnescapeString`** on the HTML body so `&lt;br/&gt;` becomes real `<br/>` for the export file.
- Builds **`Labels`** only from non-empty category terms.
- Computes **`Slug`** via `slugFromEntry`.

### Slug helpers

- **`slugFromEntry`** tries Blogger’s **`blogger:filename`** first (e.g. `/2012/09/my-post.html` → `my-post`), then slugified title, then a fallback derived from the Blogger id string.
- **`slugify`** lowercases and replaces “unsafe” characters with hyphens using a Unicode-aware regex so Cyrillic (and other scripts) still produce reasonable slugs.

### `ResolveFeedPath`

- Builds `BlogsDir = filepath.Join(bloggerRoot, "Blogs")`.
- Lists **subdirectories** only (ignores loose files).
- If **`-blog` is empty** and there is **exactly one** blog folder, pick it automatically.
- If **`-blog` is empty** and there are **multiple** folders, return an error listing valid names (user must disambiguate).
- If **`-blog` is set**, ensure that folder exists.
- Finally checks **`feed.atom`** exists inside the chosen folder.

---

## 8. WXR writer: `internal/wxr/write.go`

### Why string templates instead of `encoding/xml` marshalling?

WXR mixes namespaces, optional fields, and large HTML blobs. Building XML with **`fmt.Fprintf`** (or `strings.Builder` + `fmt.Fprintf`) keeps **CDATA** control explicit: WordPress expects post HTML inside `<content:encoded><![CDATA[ ... ]]></content:encoded>`.

### `Options`

- **`SiteTitle`** — Goes in `<channel><title>`.
- **`SiteURL`** — Base for `link`, `guid`, and `wp:base_*` URLs. Trailing slash is stripped for consistency.
- **`SkipAttachmentItems`** — If true, skip generating attachment `<item>` rows (smaller file; some importers’ **image counts** will drop to zero because they count `wp:post_type == attachment`).

### `Write` — strict output order

Many importers assume a **WordPress-like** ordering:

1. XML declaration + optional **generator** comment (some tools check for a WordPress-style marker).
2. `<rss …>` with **xmlns** declarations for `content`, `excerpt`, `dc`, `wp`, etc.
3. `<channel>` header: `title`, `link`, `wp:wxr_version`, `wp:base_site_url`, `wp:base_blog_url`.
4. **Channel-level tags** — For each distinct label, emit `<wp:tag>` with `wp:term_id`, `wp:tag_slug`, `wp:tag_name`. **Publii’s pre-import summary counts these** for “tags found.”
5. **One `<item>` per `model.Content`** — Sequential **`wp:post_id`** starting at 1. Each item gets **`uniquifySlug`** so two different posts never share the same `wp:post_name` (WordPress requires unique slugs).
6. **Attachment items** (unless skipped) — IDs continue after the last post/page. Each row references a **remote** image URL in `wp:attachment_url`. **No image bytes are downloaded** in this tool.
7. Close `</channel></rss>`.

### Per-post `<category>` rows

Even after emitting `<wp:tag>`, each post still lists its labels as:

```xml
<category domain="post_tag" nicename="linux"><![CDATA[Linux]]></category>
```

That mirrors real WordPress exports and helps importers associate posts with tags.

### `collectImageURLs`

Uses a regular expression to find `<img ... src="http...">` or `https...` with single or double quotes. Relative URLs and `data:` URLs are **skipped** on purpose (they would not work as standalone attachment URLs).

### `cdataWrap` and `xmlEscape`

- **`cdataWrap`** — Puts arbitrary text inside CDATA. If the text literally contains `]]>`, the function splits CDATA segments so the XML remains valid.
- **`xmlEscape`** — For titles, URLs in attributes, etc., where you must not inject raw `<` characters.

---

## 9. Tests: `*_test.go` files

Go discovers tests in files ending with **`_test.go`** in the same package (or `package foo_test` for “external” tests).

- **`parse_test.go`** — Feeds a **tiny inline Atom string** into `ParseReader` and asserts you get one post, one page, one skipped comment, correct slug, and HTML unescaping.
- **`parse_test.go`** also tests **`ResolveFeedPath`** using `t.TempDir()` to create a fake `Blogs/OnlyBlog/feed.atom` layout without shipping real Takeout data.
- **`write_test.go`** — Checks that output contains `post` and `page` types, channel `wp:tag`, attachment items, CDATA edge cases, duplicate slug suffixing, and the skip-attachments flag.

Run them from the repo root:

```bash
go test ./...
```

---

## 10. Ideas for extending the tool

These are **not** implemented here; they are learning exercises:

- **Include drafts** — Stop skipping `status != LIVE`, or add `-include-drafts` and map `wp:status` to `draft`.
- **Comments** — Parse `COMMENT` entries and emit `<wp:comment>` blocks (more complex threading).
- **Download images** — HTTP GET each URL, save under a folder, and adjust `wp:attachment_url` / file metadata (much larger project).
- **Background images** — Extend the regex to find `url(...)` in `style` attributes.
- **Categories vs tags** — Blogger labels are all treated as `post_tag`; you could heuristically split into `category` vs `post_tag`.

Each change usually touches **one** of: `blogger` (parse rules), `model` (new fields), `wxr` (new XML), `main` (new flags).

---

## 11. Further reading

- [WordPress Tools → Export](https://wordpress.org/documentation/article/tools-export-screen/) — official context for WXR.
- [Publii: import from WordPress](https://getpublii.com/docs/import-wordpress-into-static-html-site.html) — how Publii consumes WXR.
- [Go encoding/xml](https://pkg.go.dev/encoding/xml) — package documentation for unmarshalling.

---

*If anything in this guide drifts from the code, trust the source files and tests; this document is meant to teach intent, not replace reading the implementation.*
