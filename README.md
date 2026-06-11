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
working directory, for example `20261230_untitled.md`.

Named files are created automatically if they do not already exist.
Documents autosave to their normal path after two seconds without typing.

## Themes

Marko includes three built-in themes:

```sh
marko document.md                  # calm, the default
MARKO_THEME=green marko document.md
MARKO_THEME=mono marko document.md
```

Set `MARKO_THEME` in your shell profile to make a theme permanent.
Press `Ctrl-G` inside Marko to cycle themes. The selected theme is remembered
for future launches.

## Keys

- Arrow keys, Home, End, Page Up, Page Down: move
- Type, Backspace, Delete, Enter: edit normally
- `Ctrl-T`: create a two-column table at the cursor
- `Ctrl-G`: cycle and remember the colour theme
- `Ctrl-Z` / `Ctrl-Y`: undo / redo
- `Ctrl-F`: find the next matching text
- Shift + arrow keys or mouse drag: select text
- `Ctrl-C` / `Ctrl-X` / `Ctrl-V`: copy / cut / paste
- `Tab`: insert spaces, or move to the next cell inside a Markdown table
- `Enter` in a table: add a row
- `Ctrl-S`: save
- `Ctrl-Q`: quit

After five seconds without input, Marko enters focus mode: the status bar
disappears and surrounding text becomes quieter. Any key or mouse action exits
focus mode immediately.
