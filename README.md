# Marko

A calm terminal Markdown editor.

Marko uses familiar, modeless editing. It keeps Markdown as plain text,
styles structure inline, and makes tables less awkward.

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
