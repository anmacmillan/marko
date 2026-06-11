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
- visually distinct headings
- quick table creation and pleasant table editing
- aligned, rendered tables that reveal their Markdown source when entered
- ordinary `.md` files that work everywhere

Marko is not intended to become an IDE, knowledge-management system, or
full Markdown previewer. Its aim is to make simple terminal documents easy to
write and easy to read.

## Run

```sh
go run . document.md
```

## Keys

- Arrow keys, Home, End, Page Up, Page Down: move
- Type, Backspace, Delete, Enter: edit normally
- `Ctrl-T`: create a two-column table at the cursor
- `Tab`: insert spaces, or move to the next cell inside a Markdown table
- `Enter` in a table: add a row
- `Ctrl-S`: save
- `Ctrl-Q`: quit

This is an early prototype. It does not yet support selection, clipboard,
undo/redo, search, or soft wrapping.
