# Read-e: Desktop EPUB & PDF Reader

A desktop e-book and document reader and personal reading companion engineered with Go, Wails v2, and Svelte. Designed for speed, typography control, and friction-free note-taking with native export compatibility for knowledge management tools such as Obsidian, Notion, and Roam Research.

---

## Overview

Read-e provides a distraction-free, native desktop reading environment that balances performance with typographic polish. Built on top of a lightweight Go parsing engine and SQLite persistence layer, it handles complex EPUB 2/3 structures, rich media, and standard PDF documents without external cloud dependencies.

All data, including book metadata, reading progress, highlights, notes, and bookmarks, is stored locally in an embedded SQLite database configured with Write-Ahead Logging (WAL) and strict relational integrity.

---

## Key Features

### Document Support
- EPUB Format: Complete EPUB 2 and EPUB 3 support with chapter unpacking, spine sequencing, NCX/Nav table of contents extraction, and embedded asset serving.
- PDF Format: Native PDF document viewing with high-DPI canvas rendering, interactive Mozilla PDF.js text layer selection, outline extraction, and continuous scrolling.

### Reader Experience and Typography
- Layout and Width Controls: Custom reading width slider (560px to 960px) to adjust character line lengths for optimal eye tracking in EPUB documents.
- PDF Zoom Controls: Flexible document scaling (Fit to Width, Fit to Page, 50% to 250% zoom increments) accessible from the primary toolbar.
- Line Height and Spacing: Selectable line spacing presets (Compact 1.5, Standard 1.7, Relaxed 1.9).
- Text Alignment and Hyphenation: Toggle between ragged-right left alignment and justified text with automated browser hyphenation.
- Font Configuration: Choice of system, sans-serif (Inter), serif (Merriweather), and monospace typefaces with fine-grained font size scaling.
- Theme Palettes: Four color schemes engineered for reading comfort: Dark, Light, Sepia, and Nord.

### Annotations, Highlights, and Notes
- Multi-Color Text Highlighting: Highlight passages across single or multiple DOM nodes in five curated contrast colors (Yellow, Green, Blue, Purple, Pink) across both EPUB and PDF documents.
- Attached Annotations: Attach reflections and notes directly to any highlight via an inline editor or within the reader sidebar.
- Obsidian and Notion Markdown Export: Grouped export of all book highlights and notes structured chronologically by chapter or page, ready for immediate clipboard copy or `.md` file download.

### Navigation and Reading Tools
- In-Place Footnote Popovers: Clicking footnote or endnote anchors in EPUBs renders an Apple Books-inspired popover preview directly beside the text reference, preserving the reader's vertical scroll position.
- In-Book Fast Search: Global search overlay (`Ctrl+F` / `Cmd+F`) that indexes and searches across all EPUB chapters and PDF pages in memory with match previews and one-click jump navigation.
- Bookmark Management: Quick-toggle bookmark button and shortcut (`Ctrl+D` / `Cmd+D`) with exact scroll-offset restoration and a dedicated Bookmarks drawer.
- Reading Statistics and Pacing: Real-time chapter and document word-count calculation displaying estimated reading time remaining based on a standard 220 WPM pacing model.
- Table of Contents: Complete hierarchy navigation supporting nested sections, chapters, and PDF bookmarks.

### Library Management
- Local File Import: Add individual `.epub` or `.pdf` files, or recursively scan local directories.
- Automated Cover Extraction: Multi-tier cover discovery supporting OPF manifest properties, metadata identifiers, and structural heuristics.
- Format Indicators: Visual document format badges for clear distinction between EPUB and PDF library items.
- Progress Tracking: Continuous, background persistence of chapter or page index and vertical scroll offset.

---

## Architecture

The application is structured into decoupled layers communicating through Wails v2 IPC:

```
+------------------------------------------------------------------+
|                    Frontend (Svelte 4 + Vite)                    |
|  - App Router & Global Overlays (SearchModal, SettingsPanel)     |
|  - Reader View (Toolbar, TOCSidebar, Bookmarks, Highlights)      |
|  - Sandboxed Chapter Frame (In-place popovers, multi-node marks) |
+---------------------------------+--------------------------------+
                                  | Wails IPC Bridge
+---------------------------------v--------------------------------+
|                        Backend (Go 1.22+)                        |
|  - app.go: API Gateway and Application Lifecycle                 |
|  - internal/epub: EPUB 2/3 Unpacker, NCX/Nav Parser, Full Search |
|  - internal/library: Filesystem Scanner and SHA-256 Book Hasher  |
|  - internal/store: SQLite3 Database (WAL mode, Foreign Keys)     |
+---------------------------------+--------------------------------+
                                  |
+---------------------------------v--------------------------------+
|                   Local Storage & Persistence                    |
|  - SQLite Database: ~/.config/read-e/reader.db (Linux)           |
|    or AppData/Roaming (Windows) / Application Support (macOS)    |
+------------------------------------------------------------------+
```

---

## Getting Started

### Prerequisites

1. Go 1.21 or higher
2. Node.js 18.x or higher with npm
3. Wails CLI v2 (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

#### Platform-Specific Dependencies (Linux)

For Linux systems (Ubuntu, Debian, Fedora), WebKit2GTK and GTK3 development headers are required:

```bash
# Debian / Ubuntu
sudo apt-get update
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev build-essential
```

### Installation and Build

1. Clone the repository:
   ```bash
   git clone git@github.com:samuel025/read-e.git
   cd read-e
   ```

2. Install frontend dependencies:
   ```bash
   cd frontend
   npm install
   cd ..
   ```

3. Run in development mode with hot-reloading:
   ```bash
   wails dev
   ```

4. Compile the production desktop binary:
   ```bash
   # Linux (using WebKit2GTK 4.1)
   wails build -tags webkit2_41

   # macOS / Windows
   wails build
   ```

The compiled binary will be placed in the `build/bin/` directory.

---

## Keyboard Shortcuts

| Shortcut | Action |
| :--- | :--- |
| `Ctrl + F` / `Cmd + F` | Open in-book full-text search overlay |
| `Ctrl + D` / `Cmd + D` | Toggle bookmark at current reading position |
| `Left Arrow` / `Page Up` | Navigate to previous chapter |
| `Right Arrow` / `Page Down` | Navigate to next chapter |
| `Esc` | Close search modal, settings panel, or popovers |
| `Up / Down Arrow` (in Search) | Navigate search results |
| `Enter` (in Search) | Jump directly to selected search result |

---

## Database Schema

Read-e stores application state using embedded SQLite. The database schema includes:

- `books`: Unique books identified by SHA-256 hash, title, author, cover path, total spine chapters, and timestamps.
- `progress`: Active reading state per book (spine index, scroll position, and last read timestamp).
- `settings`: Serialized JSON configuration for themes, typography, and layout preferences.
- `highlights`: Text selections with color tags, spine chapter index, and attached notes.
- `bookmarks`: Saved reading positions with chapter references, title labels, and precise vertical scroll offsets.

All relational tables enforce `ON DELETE CASCADE` constraints linked to the primary `books(id)` table.

---

## License

This project is licensed under the MIT License. See the LICENSE file for details.
