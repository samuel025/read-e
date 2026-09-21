# Product Requirements Document: EPUB Reader for Ubuntu

**Author:** Samuel
**Platform:** Ubuntu (Linux desktop)
**Stack:** Go (backend) + Wails (desktop shell) + WebKitGTK (render engine) + HTML/CSS/JS (frontend)
**Status:** Draft v1

---

## 1. Overview

A native desktop EPUB reader for Ubuntu, built with Go and Wails. The app parses EPUB files (container/OPF/spine/TOC), renders chapter content using the system webview (WebKitGTK), and provides a clean reading experience with library management, progress tracking, and basic customization (font size, theme).

## 2. Goals

- Ship a functional, installable Ubuntu desktop app that opens and reads real-world EPUB files correctly.
- Native-feeling performance: fast startup, instant chapter navigation, low memory footprint.
- Clean separation between parsing/business logic (Go) and presentation (HTML/CSS/JS).
- Persist reading state (last position, library) across sessions.

## 3. Non-Goals (v1)

- No EPUB *editing* or *creation* — read-only.
- No cloud sync / multi-device support.
- No DRM-protected EPUB support (Adobe DRM, etc.) — plain EPUB2/EPUB3 only.
- No mobile builds (Android/iOS) — Ubuntu desktop only for v1.
- No annotation/highlighting system in v1 (candidate for v2).

## 4. Target User

Primarily the developer themself — a technical reader on Ubuntu who wants a lightweight, no-frills EPUB reader without relying on Calibre's viewer or a browser extension.

## 5. Core Features

### 5.1 Library Management
- Point the app at a folder (or add files individually); scan for `.epub` files.
- Extract and cache metadata: title, author, cover image, file path.
- Display library as a grid/list view with cover thumbnails.
- Remove books from library (does not delete the underlying file).

### 5.2 EPUB Parsing
- Parse `META-INF/container.xml` to locate the OPF file.
- Parse OPF: manifest (item list), spine (reading order), metadata (title/author/language), cover reference.
- Parse navigation document (EPUB3 `nav.xhtml`) or NCX (EPUB2 fallback) for table of contents.
- Support both EPUB2 and EPUB3 structures.
- Handle malformed/non-standard EPUBs gracefully (common in the wild) — don't hard-crash on missing/invalid nav files.

### 5.3 Reading View
- Render chapter XHTML content in the Wails webview, with injected theme CSS.
- Chapter-to-chapter navigation (next/previous, respecting spine order).
- Table of contents sidebar — click to jump to a section.
- Progress indicator (e.g., "Chapter 4 of 12" or % through book).
- Remember and restore exact reading position (chapter + scroll offset) per book.

### 5.4 Customization
- Font size adjustment (increase/decrease, persisted per book or globally).
- Theme toggle: light / dark / sepia.
- Optional: font family selection (system fonts).

### 5.5 Persistence
- Local storage for library metadata and reading progress — SQLite or per-book JSON (decision in Section 8).
- No external network calls required for core functionality.

## 6. Non-Functional Requirements

- **Startup time:** App should be usable (library visible) within ~1 second on typical hardware.
- **Chapter load time:** Sub-200ms for typical chapter sizes.
- **Packaging:** Distributable as a `.deb` package and/or AppImage.
- **Dependencies:** Requires `libwebkit2gtk` and `libgtk-3` at runtime (standard Wails Linux requirement).
- **Resilience:** Should not crash on malformed EPUBs — fail gracefully with an error message per book.

## 7. Architecture

```
┌─────────────────────────────────────────┐
│              Wails Frontend              │
│   (HTML/CSS/JS or Svelte — library UI,   │
│    TOC sidebar, reading pane, settings)  │
└───────────────────┬───────────────────────┘
                     │ Wails-generated JS bindings
┌───────────────────┴───────────────────────┐
│               Go Backend                  │
│  ┌──────────────┐  ┌────────────────────┐ │
│  │ EPUB Parser  │  │ Library Manager     │ │
│  │ (zip/xml)    │  │ (scan, cache, meta) │ │
│  └──────────────┘  └────────────────────┘ │
│  ┌──────────────┐  ┌────────────────────┐ │
│  │ Progress /   │  │ Settings Store      │ │
│  │ State Store  │  │ (theme, font size)  │ │
│  └──────────────┘  └────────────────────┘ │
└─────────────────────────────────────────┘
```

### Suggested Go package structure
```
/internal/epub/       — container.xml, OPF, spine, TOC parsing
/internal/library/     — folder scanning, metadata caching
/internal/store/       — persistence (SQLite/JSON) for progress & settings
/frontend/             — Wails frontend (HTML/CSS/JS)
main.go, app.go        — Wails app entrypoint, bound methods
```

### Bound methods (Go → frontend)
- `ScanLibrary(folderPath string) ([]BookMeta, error)`
- `OpenBook(bookID string) (BookInfo, error)`
- `GetTOC(bookID string) ([]TOCEntry, error)`
- `GetChapter(bookID string, spineIndex int) (string, error)` — returns raw/sanitized XHTML
- `SaveProgress(bookID string, position ReadingPosition) error`
- `GetProgress(bookID string) (ReadingPosition, error)`
- `SaveSettings(settings AppSettings) error`

## 8. Open Decisions

| Decision | Options | Notes |
|---|---|---|
| Persistence layer | SQLite vs. flat JSON files | SQLite scales better if library grows large; JSON is simpler to start with |
| Frontend framework | Vanilla JS vs. Svelte | Svelte is the Wails default template and pairs well; vanilla keeps deps minimal |
| Chapter rendering | Direct iframe injection vs. sandboxed div | Iframe isolates EPUB's own CSS from app chrome CSS — likely the safer choice |
| Cover extraction | Parse from OPF manifest vs. heuristic (first image) | OPF-declared cover should be primary; fallback heuristic for malformed EPUBs |

## 9. Milestones

1. **M1 — Parser core:** container.xml → OPF → spine/manifest/TOC parsing, tested against a handful of real-world EPUBs (EPUB2 and EPUB3).
2. **M2 — Minimal Wails shell:** Open a single EPUB by file path, render chapters, next/prev navigation.
3. **M3 — Library view:** Folder scanning, cover/metadata caching, library grid UI.
4. **M4 — Persistence:** Reading position and settings survive app restart.
5. **M5 — Customization:** Font size, theme (light/dark/sepia), TOC sidebar navigation.
6. **M6 — Packaging:** `.deb` build, basic install/uninstall tested on a clean Ubuntu VM.

## 10. Success Criteria

- Successfully opens and renders 10+ real-world EPUB files (mix of EPUB2/EPUB3, varying publishers) without crashing.
- Reading position persists correctly across app restarts.
- App installs cleanly via `.deb` on a fresh Ubuntu install.
- Chapter navigation and TOC jumps feel instant (no visible lag).

## 11. Risks

- **Malformed EPUBs are common in the wild** — parser needs defensive handling, not just spec-compliant parsing.
- **CSS from EPUB content may conflict with app chrome** — mitigate via iframe isolation.
- **WebKitGTK version fragmentation across Ubuntu versions** — test on at least two LTS versions (e.g., 22.04, 24.04).
