# Marko

**A calm, modeless Markdown editor for the terminal.**

Marko combines Micro-style familiar editing with inline-rendered headings,
emphasis, and tables. There is no preview pane, no modal editing, no notes
database, and no workspace to manage. Your files remain ordinary Markdown.

![Simulated Marko terminal screenshot](docs/marko-screenshot.svg)

## Why Marko?

Terminal Markdown tools often make you choose between a plain text editor,
a separate read-only viewer, or a powerful modal editor with a learning curve.
Marko fills the smaller gap between them.

- Type and navigate normally with arrow keys, mouse, selection, and clipboard
- Read clean headings, emphasis, and aligned tables in the editable buffer
- Enter a rendered structure to reveal its ordinary Markdown source
- Write in a centered, word-wrapped, Goyo-style column
- Autosave safely, detect external changes, and keep a recovery journal
- Keep every document portable as a plain `.md` file

Marko deliberately avoids becoming an IDE or knowledge-management system. It
aims to make everyday terminal notes and documents pleasant to write.

## Quick Start

```sh
marko                 # new dated note in the current directory
marko document.md     # open or create a named Markdown file
```

New unnamed notes use `YYYYMMDD_untitled.md`, then `_2`, `_3`, and so on.
Documents autosave after two idle seconds.

Press `F1` inside Marko for the complete shortcut overlay.

## Highlights

### Comfortable writing

- Modeless editing with mouse support
- Centered 88-column writing area on wide terminals
- Word-aware wrapping and automatic focus mode
- Four persistent themes: calm, green, mono, and light
- Recent-file picker with `Ctrl-E`

### Markdown without friction

- Rendered headings, bold, italic, strikethrough, and tables
- `Ctrl-T` creates a table
- `Tab` moves through table cells
- `Enter` adds a row; on an empty final row, it leaves the table
- Lists, numbered lists, and checkboxes continue automatically
- `Ctrl-Space` toggles a checkbox

### Editor essentials

- Undo/redo, live search, replace, and replace all
- Mouse drag, double-click word, and triple-click line selection
- Typing, Backspace, or Delete replaces/removes selected text
- Cross-platform clipboard support
- Save As, rename, reload, external-change protection, and recovery journal

## Install

Download the appropriate binary from
[GitHub Releases](https://github.com/anmacmillan/marko/releases), rename it to
`marko`, make it executable, and place it on your `PATH`.

Or build from source:

```sh
go install github.com/alexandermacmillan/little-marco@latest
```

## Essential Keys

| Key | Action |
|---|---|
| `F1` | Show shortcut help |
| `Ctrl-S` / `Ctrl-Shift-S` | Save / Save As |
| `Ctrl-E` | Open recent Markdown file |
| `Ctrl-F` / `Ctrl-N` / `Ctrl-P` | Search / next / previous |
| `Ctrl-R` | Replace current; `Ctrl-A` in prompt replaces all |
| `Ctrl-Z` / `Ctrl-Y` | Undo / redo |
| `Ctrl-C` / `Ctrl-X` / `Ctrl-V` | Copy / cut / paste |
| `Ctrl-T` | Create table |
| `Ctrl-G` | Cycle theme |
| `Ctrl-Q` | Quit |

## Themes And Fonts

```sh
MARKO_THEME=green marko document.md
MARKO_THEME=mono marko document.md
MARKO_THEME=light marko document.md
```

Press `Ctrl-G` to cycle and remember themes. Marko inherits your terminal font;
for a warmer writing feel, try Berkeley Mono, iA Writer Mono, or Atkinson
Hyperlegible Mono in your terminal settings.

## Philosophy

Marko is intentionally small. It renders the Markdown structures that matter
most for everyday writing, while leaving the underlying text visible whenever
you edit it. Features that turn it into an IDE, file manager, or proprietary
notes system are outside its scope.
