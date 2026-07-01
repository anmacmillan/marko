# Marko

**A calm, modeless Markdown editor for the terminal.**

Marko combines the best bits of `pencil`-style writing and Goyo-style focus with **dynamic inline Markdown rendering**. Headings, emphasis, lists, and tables render cleanly inside the terminal; move your cursor into them and their editable Markdown source reappears instantly.

There is no preview pane, no modal editing, no notes database, and no workspace to manage. Your files remain ordinary Markdown.

![Simulated Marko terminal screenshot](docs/marko-screenshot.svg)

### Sample graphics

![Rendered table sample](docs/table-sample.svg)

The screenshot shows both states: a rendered table in the editor and the raw Markdown source that appears when you enter it. There is no preview pane.

## Why Marko?

Terminal Markdown tools often force a compromise: a raw plain-text editor, a separate read-only viewer, or a powerful modal editor with a steep learning curve. Marko bridges the gap. It is built for writers who love terminal efficiency but want a calm, visually polished environment that respects the layout of their document.

- **Write in Place**: Read dynamically rendered headings, emphasis, list markers, and beautifully aligned tables directly in the editor buffer. Step into them to edit their raw Markdown syntax.
- **True Focus Mode**: Highlight the current active paragraph, table, list item, or code block while smoothly dimming the surrounding context with theme-aware Goyo/Limelight-style backgrounds.
- **Auto-Sync & Live Reload**: Perfect for hybrid workflows. Marko automatically detects external file changes (like edits made by AI tools or external scripts) and live-reloads your document instantly while preserving your precise scroll position and cursor location.
- **Natural Visual Navigation**: Arrow keys navigate visually on wrapped prose lines rather than physical source lines, matching the intuitive feel of GUI editors.
- **Rich Terminal Integration**: Full support for trackpad/mouse-wheel scrolling, double-click word selection, triple-click line selection, cross-platform clipboard syncing (`Ctrl-C`/`Ctrl-V`), and bracketed paste protection.

## Quick Start

```sh
marko                 # new dated note in the current directory (YYYYMMDD_untitled.md)
marko document.md     # open or create a named Markdown file
```

Documents autosave after two idle seconds. Press `F1` inside Marko for the complete shortcut overlay.

## Features We Are Proud Of

### 💡 Dynamic Inline Markdown Rendering
- **Aligned Tables**: Markdown tables instantly format into clean, boxed unicode tables. Pressing `Tab` moves through cells, `Enter` inserts new rows, and stepping out preserves the formatting.
- **Fenced Code Blocks**: Raw code blocks fold into visually distinct boxes with calm language labels. Stepping inside reveals the code, and stepping out hides the syntax.
- **Emphasis & Headers**: Headers are styled and clean, and bold, italic, underline, highlight, or strikethrough markdown markers disappear inline unless you are actively editing that line. Toggle inline formatting WYSIWYG-style with `Ctrl-B` (bold), `Ctrl-E` (italic), `Ctrl-U` (underline), and `Ctrl-H` (highlight); use `F5`, `F6`, and `F7` to toggle H1, H2, and H3 on the current line. H1 and H2 also get extra vertical breathing room to fake scale inside the terminal.
- **Interactive Checkboxes**: Toggle lists and markdown checkboxes `[ ]` / `[x]` instantly using `Ctrl-Space`.

### 🎯 Intelligent Focus Mode
Focus mode (toggled persistently using `Ctrl-K`) doesn't just dim lines blindly. It parses Markdown structure and uses each theme's own focus and dim backgrounds:
- If you're on a paragraph, it highlights the paragraph and dims everything else.
- If you enter a table, the entire table remains beautifully lit.
- If you write code, the entire code block stays visible.
- It respects headers, list structures, and blockquotes, keeping your immediate workspace clear and isolated.

### 🔄 Asynchronous External Syncing
Marko works perfectly alongside external automations, scripts, or AI assistants. The editor watches the underlying file on a 500ms heartbeat. If an AI writes an update to the file in the background, Marko instantly reloads the buffer and redraws your screen, keeping your cursor and scroll viewport exactly where you left them.


## Essential Keys

| Key | Action |
|---|---|
| `F1` | Show shortcut help |
| `Ctrl-S` / `F2` / `Ctrl-Shift-S` | Save / Save As (first save of an untitled doc always asks for a name) |
| `F4` | Open the Marko home screen |
| `Ctrl-B` | Bold (`**…**`) — wrap/unwrap selection, or insert empty markers |
| `Ctrl-E` | Italic (`*…*`) — wrap/unwrap selection, or insert empty markers |
| `Ctrl-U` | Underline (`<u>…</u>`) — wrap/unwrap selection, or insert empty markers |
| `Ctrl-H` | Highlight (`==…==`, theme-colored) — wrap/unwrap selection, or insert empty markers |
| `F5` / `F6` / `F7` | Toggle H1 / H2 / H3 on the current line |
| `Ctrl-A` | Select all |
| `F3` / `Ctrl-Shift-E` | Open recent Markdown file |
| `Ctrl-F` / `Ctrl-N` / `Ctrl-P` | Search / next / previous |
| `Ctrl-R` | Replace current; `Ctrl-A` in prompt replaces all |
| `Ctrl-Z` / `Ctrl-Y` | Undo / redo |
| `Ctrl-C` / `Ctrl-X` / `Ctrl-V` | Copy / cut / paste |
| `Ctrl-T` | Create table |
| `Ctrl-G` | Cycle theme |
| `Ctrl-K` | Toggle focus mode (Goyo-style) |
| `Ctrl-Q` | Quit |

> Italic is on `Ctrl-E` rather than `Ctrl-I` because in terminals `Ctrl-I` is
> the same key as `Tab`, which Marko uses to navigate table cells.

## Install

Download the appropriate binary from [GitHub Releases](https://github.com/anmacmillan/marko/releases), rename it to `marko`, make it executable, and place it on your `PATH`.

Or build from source:

```sh
go install github.com/alexandermacmillan/little-marco@latest
```

After cloning the repo, `make install` builds a stripped binary and installs it
to `~/.local/bin/marko`:

```sh
git clone https://github.com/anmacmillan/marko.git
cd marko
make install      # builds and installs ~/.local/bin/marko
make test         # optional: run the test suite
```

## Themes And Fonts

```sh
MARKO_THEME=matrix marko document.md
MARKO_THEME=midnight marko document.md
MARKO_THEME=paper marko document.md
MARKO_THEME=ember marko document.md
MARKO_THEME=green marko document.md
MARKO_THEME=mono marko document.md
MARKO_THEME=light marko document.md
```

Press `Ctrl-G` to cycle and remember themes. `matrix` is the phosphor terminal-focus palette; `midnight` is a quiet dark writing palette; `paper` is warm and bright; `ember` is a warmer evening dark theme. Marko inherits your terminal font; for a warmer writing feel, try Berkeley Mono, iA Writer Mono, or Atkinson Hyperlegible Mono in your terminal settings.

## Home Screen And Paths

Press `F4` to open the Marko home screen from any document. Use Up/Down and Enter to choose New document, Open path, Recent files, Theme, Return to document, or Quit. `Esc` returns to the current document.

Open and Save As prompts accept ordinary paths, `~/...`, and zoxide shortcuts:

```text
z briefs/draft.md
```

That expands `briefs` through `zoxide query briefs`, then appends `draft.md`.

## Philosophy

Marko is intentionally small. It focuses on rendering the Markdown structures that matter most for everyday writing while keeping your underlying text readable, portable, and standard. Features that turn it into an IDE, file manager, or proprietary notes system are outside its scope.
