# Marko

**A calm, modeless Markdown editor for the terminal.**

Marko combines the best bits of `pencil`-style writing and Goyo-style focus
with **dynamic inline Markdown rendering**. Headings, emphasis, and tables
render cleanly inside the terminal; move into them and their editable Markdown
source reappears instantly.

There is no preview pane, no modal editing, no notes database, and no workspace
to manage. Your files remain ordinary Markdown.

![Simulated Marko terminal screenshot](docs/marko-screenshot.svg)

The screenshot shows both states: a rendered table in the editor and the raw
Markdown source that appears when you enter it. There is no preview pane.

## Why Marko?

Terminal Markdown tools often make you choose between a plain text editor,
a separate read-only viewer, or a powerful modal editor with a learning curve.
Marko fills the smaller gap between them.

- Type and navigate normally with arrow keys, mouse, selection, and clipboard
- Arrow movement follows wrapped visual lines, closer to `pencil` than a raw
  source-line editor
- Read dynamically rendered headings, emphasis, and aligned tables directly in
  the editable terminal buffer
- Enter any rendered structure to reveal and edit its ordinary Markdown source
- Write in a centered, word-wrapped, Goyo-style column
- Focus mode highlights the whole current paragraph instead of just one line
- Trackpad and mouse-wheel scrolling move the viewport naturally
- Autosave safely, detect external changes, and keep a recovery journal
- Keep every document portable as a plain `.md` file

Marko deliberately avoids becoming an IDE or knowledge-management system. It
aims to be a simple Markdown editor that keeps the writing surface calm,
focused, and portable.

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
- Word-aware wrapping and paragraph-based focus mode
- Visual-line navigation on wrapped prose
- Mouse-wheel and trackpad scrolling
- Four persistent themes: calm, green, mono, and light
- Recent-file picker with `Ctrl-E`

### Dynamic Markdown rendering

- Tables dynamically render as aligned terminal tables, then reveal their
  Markdown pipes when entered
- Headings, bold, italic, and strikethrough follow the same reveal-on-edit model
- Everything happens inside the editable buffer, never in a preview pane
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
