# Marko

A calm terminal Markdown editor.

Marko uses familiar, modeless editing. It keeps Markdown as plain text,
styles structure inline, and makes tables less awkward.

## Use case

Marko is for people who like writing Markdown in the terminal but do not want
to learn modal editing, use a separate preview pane, or launch a heavyweight
desktop editor.

It intentionally focuses on the parts of Markdown that provide most of the
value in everyday notes and documents:

- familiar, low-friction text editing
- undo/redo, search, selection, clipboard, mouse placement, and soft wrapping
- visually distinct headings
- clean rendered headings that reveal their Markdown markers when entered
- rendered bold, italic, and struck-through text that reveals its markers when entered
- quick table creation and pleasant table editing
- aligned, rendered tables that reveal their Markdown source when entered
- ordinary `.md` files that work everywhere

Marko is not intended to become an IDE, knowledge-management system, or
full Markdown previewer. Its aim is to make simple terminal documents easy to
write and easy to read.

## Start writing

```sh
# Create a new Markdown document
marko

# Open or create a named Markdown document
marko document.md
```

Running `marko` by itself opens a dated untitled document in the current
working directory, for example `20261230_untitled.md`. Further new documents
that day use `_2`, `_3`, and so on, so nothing is overwritten.

Named files are created automatically if they do not already exist.
Documents autosave to their normal path after two seconds without typing.
If another program changes the file, Marko refuses to overwrite it and asks
you to reopen it or use Save As.

## Install

Download the appropriate binary from
[GitHub Releases](https://github.com/anmacmillan/marko/releases), rename it to
`marko`, make it executable, and place it on your `PATH`.

Or build from source:

```sh
go install github.com/alexandermacmillan/little-marco@latest
```

## Themes

Marko includes four built-in themes:

```sh
marko document.md                  # calm, the default
MARKO_THEME=green marko document.md
MARKO_THEME=mono marko document.md
MARKO_THEME=light marko document.md
```

Set `MARKO_THEME` in your shell profile to make a theme permanent.
Press `Ctrl-G` inside Marko to cycle themes. The selected theme is remembered
for future launches.

## Keys

- Arrow keys, Home, End, Page Up, Page Down: move
- Type, Backspace, Delete, Enter: edit normally
- `Ctrl-T`: create a two-column table at the cursor
- `F1`: show or hide the shortcut help overlay
- `Ctrl-E`: open one of the five most recently used Markdown files
- `Ctrl-G`: cycle and remember the colour theme
- `Ctrl-Z` / `Ctrl-Y`: undo / redo
- `Ctrl-F`: live search; matches highlight while typing
- `Enter` / `Ctrl-N`: next search match
- `Ctrl-P`: previous search match
- `Ctrl-R`: replace current match; press `Ctrl-A` in the replacement prompt to replace all
- Shift + arrow keys or mouse drag: select text
- `Ctrl-C` / `Ctrl-X` / `Ctrl-V`: copy / cut / paste
- `Ctrl-Space`: toggle a checkbox on the current line
- `Ctrl-O`: open a Markdown or web link on the current line
- `Tab`: insert spaces, or move to the next cell inside a Markdown table
- `Enter` in a table: add a row
- `Enter` in a list: continue bullets, checkboxes, or numbered lists
- `Ctrl-S`: save
- `Ctrl-Shift-S`: Save As
- `Ctrl-Q`: quit

Prose wraps at word boundaries. Clipboard commands support macOS, Windows,
Wayland (`wl-copy` / `wl-paste`), and X11 (`xclip`), with terminal clipboard
fallback where supported.

After five seconds without input, Marko enters focus mode: the status bar
disappears and surrounding text becomes quieter. Any key or mouse action exits
focus mode immediately.
