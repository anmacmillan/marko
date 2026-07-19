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
marko                 # new dated note in your notes directory (~/Notes by default)
marko document.md     # open or create a named Markdown file
```

Documents autosave after two idle seconds. Press `F1` inside Marko for the complete shortcut overlay.

## Features We Are Proud Of

### 💡 Dynamic Inline Markdown Rendering
- **Aligned Tables**: Markdown tables instantly format into clean, boxed unicode tables. `Ctrl-T` opens a Word-style grid picker to insert a table of any size; pressing `Tab` moves through cells, `Enter` inserts new rows, and stepping out preserves the formatting. Inside a table, `Alt`-arrows add or delete rows and columns, and `Ctrl-T` cycles the current column's alignment (`:---` / `:---:` / `---:`) — the alignment is respected in both the raw source and the rendered view.
- **Fenced Code Blocks**: Raw code blocks fold into visually distinct boxes with calm language labels. Stepping inside reveals the code, and stepping out hides the syntax.
- **Emphasis & Headers**: Headers are styled and clean, and bold, italic, underline, highlight, or strikethrough markdown markers disappear inline unless you are actively editing that line. Toggle inline formatting WYSIWYG-style with `Ctrl-B` (bold), `Ctrl-E` (italic), `Ctrl-U` (underline), and `Ctrl-H` (highlight); use `F5`, `F6`, and `F7` to toggle H1, H2, and H3 on the current line. H1 and H2 also get extra vertical breathing room to fake scale inside the terminal.
- **Interactive Checkboxes**: Toggle lists and markdown checkboxes `[ ]` / `[x]` instantly using `Ctrl-Space`.

### 🎯 Intelligent Focus Mode
Focus mode (toggled persistently using `Ctrl-K`) doesn't just dim lines blindly. It parses Markdown structure and recedes everything else Limelight-style:
- If you're on a paragraph, it highlights the paragraph and dims everything else.
- If you enter a table, the entire table remains beautifully lit.
- If you write code, the entire code block stays visible.
- It respects headers, list structures, and blockquotes, keeping your immediate workspace clear and isolated.
- **True dimming**: surrounding text is blended toward the page colour rather than repainted flat grey, so headings and accents keep their hue while receding. The lines immediately around your paragraph get a half-strength dim for a soft edge.
- **Typewriter scrolling**: in focus mode the cursor line stays vertically centred, so your eyes never chase the cursor down the screen.
- **Focus scope**: `Ctrl-Shift-K` cycles between paragraph focus and section focus (everything under the current heading stays lit — lovely for editing).

### 🔄 Asynchronous External Syncing
Marko works perfectly alongside external automations, scripts, or AI assistants. The editor watches the underlying file on a 500ms heartbeat. If an AI writes an update to the file in the background, Marko instantly reloads the buffer and redraws your screen, keeping your cursor and scroll viewport exactly where you left them.


## Essential Keys

| Key | Action |
|---|---|
| `F1` | Show shortcut help |
| `Ctrl-S` / `F2` / `Ctrl-Shift-S` | Save / Save As (first save of an untitled doc always asks for a name; save status flashes in focus mode) |
| `F4` | Open the Marko home screen |
| `Ctrl-B` | Bold (`**…**`) — wrap/unwrap selection, or insert empty markers |
| `Ctrl-E` | Italic (`*…*`) — wrap/unwrap selection, or insert empty markers |
| `Ctrl-U` | Underline (`<u>…</u>`) — wrap/unwrap selection, or insert empty markers |
| `Ctrl-H` | Highlight (`==…==`, theme-colored) — wrap/unwrap selection, or insert empty markers |
| `F5` / `F6` / `F7` | Toggle H1 / H2 / H3 on the current line |
| `Ctrl-A` | Select all |
| `F3` / `Ctrl-Shift-E` | Open the file picker (recents + browse, fuzzy filter) |
| `Ctrl-Shift-R` | Rename the current file (stays in its own folder) |
| `Ctrl-F` / `Ctrl-N` / `Ctrl-P` | Search / next / previous |
| `Ctrl-Shift-F` / `F8` | Search all notes — filenames **and** content, matching line shown |
| `Ctrl-W` | Set a session word goal (progress in the status bar, flash on reach) |
| `Ctrl-R` | Replace current; `Ctrl-A` in prompt replaces all |
| `Ctrl-Z` / `Ctrl-Y` | Undo / redo |
| `Ctrl-C` / `Ctrl-X` / `Ctrl-V` | Copy / cut / paste |
| `Ctrl-T` | Insert table (grid size picker); inside a table, cycle the current column's alignment |
| `Alt-Down` / `Alt-Up` | Inside a table: add / delete a row |
| `Alt-Right` / `Alt-Left` | Inside a table: add / delete a column |
| `Ctrl-G` | Cycle theme |
| `Ctrl-K` | Toggle focus mode (Goyo-style) |
| `Ctrl-Shift-K` | Cycle focus scope: paragraph / section |
| `Ctrl-Q` | Quit |

> Italic is on `Ctrl-E` rather than `Ctrl-I` because in terminals `Ctrl-I` is
> the same key as `Tab`, which Marko uses to navigate table cells.

## Install

Download the appropriate binary from [GitHub Releases](https://github.com/anmacmillan/marko/releases), rename it to `marko`, make it executable, and place it on your `PATH`.

If you installed from a GitHub release binary, run `marko update` to download and replace it with the latest release for your platform.

Official release binaries are published for macOS (`arm64` and `amd64`), Linux (`arm64` and `amd64`), and Windows (`amd64`).

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

## File Picker, Home Screen And Paths

`F3` (or `Ctrl-Shift-E`) opens the file picker: your recent files (up to 50) followed by the Markdown files and folders of the current directory, newest first. Type to fuzzy-filter (`mtng` matches `meeting-notes.md`), `Enter` opens the highlighted file or steps into a folder, `Tab` steps into a highlighted folder, and `Backspace` on an empty filter goes up one level. Typing a name that matches nothing and pressing `Enter` creates that file — the picker doubles as "new named note here".

Save As (`F2`) and Rename (`Ctrl-Shift-R`) use the same panel: the filename is pre-selected for instant retyping, the target folder's contents are listed below so you can see collisions (with an explicit "overwrites…" warning), and you can browse into a different folder before committing. Rename is anchored to the file's own directory, so renaming never silently moves the file to wherever you launched Marko from.

Save As is a real folder navigator: a `..` row leads up, `Right` on a highlighted folder unfolds its contents inline as an indented tree (`Left` folds it again, or jumps a nested entry back to its parent folder), and `Enter` steps in. Above the listing, a **Recent folders** section offers the directories you last saved into — so the second note for a matter is `Down`, `Enter`, `Enter`.

Press `F4` to open the Marko home screen from any document. Use Up/Down and Enter to choose New document, Open / browse, a recent file, Theme, Return to document, or Quit. `Esc` returns to the current document. Recent files are grouped by age on the home screen: past 48 hours, past week, older, and older than 2 weeks.

Launching `marko` with no file shows the same home screen over a fresh note — but there, **just start typing**: the first character dismisses the menu and lands in the note (the footer shows where it will save). Letter accelerators only apply to the `F4`-opened menu.

### Quick captures land in one place

`marko` with no arguments creates the note in your **notes directory** — `~/Notes` by default, overridable with the `MARKO_NOTES_DIR` environment variable or a path written to the `marko/notes-dir` config file. Autosave writes there too, so a note dashed off mid-phone-call is always in the same folder, never scattered across whatever directory Marko happened to be launched from. The status bar shows the file's directory, and Save/Autosave flashes the full path.

### Smart save names

Save As for an untitled note pre-fills a filename derived from the note's first line: type `Call with Smith re settlement`, hit `Ctrl-S`, and the picker suggests `YYYYMMDD_call-with-smith-re-settlement.md` — press `Enter` to accept or just type to replace it (the suggestion is pre-selected).

### Search every note

`Ctrl-Shift-F` (or `F8`, or `[S]` on the home screen) searches the whole notes directory, subfolders included. Every space-separated word must appear in the filename or the file's content — so `vyas settlement` finds the call note even if the filename only says `20260601_untitled.md`. Matches show the matching line next to the filename; with no query, notes are listed newest first, so it doubles as a "recent captures" browser. `Enter` opens the highlighted note; typing an unmatched name and pressing `Enter` creates it in the notes directory.

### Session word goal

`Ctrl-W` sets a word target counted from that moment (blank clears it). Progress appears in the status bar as `goal 340/500`, and Marko flashes a message the moment you cross the target — visible even in focus mode.

The picker input also understands zoxide shortcuts and ordinary paths. Type a zoxide query — multi-word queries work, each word is a zoxide keyword — followed by a filename, all in one go:

```text
z little marco draft
```

If the whole input has no zoxide match, the last word is treated as the filename, so this opens or saves `draft.md` in the `little-marco` directory. A slash also separates the filename explicitly (useful for names with spaces):

```text
z briefs/meeting notes.md
```

Or press `Tab` to expand the folder first, then type the filename:

```text
z briefs<Tab>
/Users/you/Documents/Briefs/
```

If the final path is a directory, Marko uses the suggested name from the note's first line; a missing extension becomes `.md`.

## Philosophy

Marko is intentionally small. It focuses on rendering the Markdown structures that matter most for everyday writing while keeping your underlying text readable, portable, and standard. Features that turn it into an IDE, file manager, or proprietary notes system are outside its scope.
