# Tools for Vex IQ — Specification

## 1. Overview

**Purpose:** Tools for Vex IQ is a desktop app that helps VEX IQ developers work with their project
files. It is designed to hold multiple small tools over time.

### Tools

- **Python ↔ IQPython Sync:** VEXcode IQ stores a Python program inside a `.iqpython` project
  file, which makes it awkward to edit the code in a normal editor. This tool keeps each `.py`
  file and its matching `.iqpython` file (same basename) in step, in both directions
  - **Python → IQ:** whenever a `.py` file is saved, the matching `.iqpython` is regenerated so
    its code matches the `.py` source. All other settings in the `.iqpython` file are left
    untouched.
  - **IQ → Python:** whenever a `.iqpython` file is saved, the matching `.py` is regenerated with
    the code from its `textContent` field.

**Target platforms:** macOS, Windows, and Linux desktop.

## 2. Glossary

| Term | Meaning |
|---|---|
| `.py` file | A plain Python source file written by the user. |
| `.iqpython` file | VEXcode IQ project file (JSON) that wraps Python source in its `textContent` field. |
| Basename | File name without its extension. `Foo.py` and `Foo.iqpython` share basename `Foo`. |
| Watched directory | The single directory the user selects in the app. |
| Pair | The `.py` and `.iqpython` files that share a basename. |
| Sync direction | Either *Python → IQ* (source is `.py`, target is `.iqpython`) or *IQ → Python* (source is `.iqpython`, target is `.py`). Both are active while watching. A change in one file triggers a sync in one direction only (see §5.2). |
| Template | The built-in `.iqpython` structure used when a new `.iqpython` file is created (see §5.4). It is derived from `Sample.iqpython`. |

## 3. Goals and Non-Goals

### Goals
- Let the user pick one directory to watch.
- Keep both directions active while watching, with no direction picker.
- Python → IQ: whenever a `.py` file in that directory is saved or modified, regenerate the
  matching `.iqpython` file so VEXcode IQ reflects the latest Python source.
- IQ → Python: whenever a `.iqpython` file is saved or modified, regenerate the matching `.py`
  file from its `textContent`.
- Keep every other field of the `.iqpython` file unchanged in value.
- Present a very basic single-screen UI.
- Remember the directory and running state across launches.

### Non-Goals (v1)
- Recursive watching of subdirectories.
- Editing Python code inside the app.
- Multiple simultaneous watched directories.
- Auto-update and code signing.

## 4. User Experience

### 4.1 Single Screen Layout

The app opens to one window containing, top to bottom:

1. **Header** — app title "IQ Sync", with a short description in smaller text beneath it.
2. **Directory row** — a read-only label showing the selected directory path (or "No directory
   selected"), and a **Choose Directory…** button. When the selected directory is not inside a
   Git repository, a warning is shown below this row (FR-1.4).
3. **Control row** — a **Start Watching** / **Stop Watching** toggle button. Disabled until a
   directory is selected.
4. **Status line** — one of `Idle`, `Watching <path>`, or `Error: <message>`.
5. **Activity log** — a scrollable list (newest at bottom) of events, each with a timestamp:
   - `Updated Foo.iqpython from Foo.py` (Python → IQ)
   - `Updated Foo.py from Foo.iqpython` (IQ → Python)
   - `Created Foo.iqpython from template` (see §5.4)
   - `Created Foo.py from Foo.iqpython` (see §5.4)
   - `Ignored deletion of Foo.py` / `Ignored deletion of Foo.iqpython` (see FR-2.5)
   - `Error: <message>` for a file that could not be synced (for example invalid JSON)
   - Log is capped at the most recent 500 entries.

### 4.2 Interaction Rules
- Watching starts only when the user clicks **Start Watching**, or when the app launches and the
  previous session was watching (FR-1.5).
- The directory can only be changed while stopped.
- Status moves between `Idle` and `Watching`, and to `Error` on watcher-level failure (FR-5.2).
- Sync latency follows NFR-1. Usage instructions are in `README.md`.

## 5. Functional Requirements

### 5.1 Directory Selection
- FR-1.1: The user can select exactly one directory via a native folder picker.
- FR-1.2: The selected directory must exist and be readable and writable. Otherwise show an error
  and do not start watching.
- FR-1.3: Watching always syncs in both directions (see §5.2). There is no direction setting.
- FR-1.4: After a directory is selected, the app checks whether it is inside a Git repository,
  that is, whether it or any ancestor directory contains a `.git` entry (directory or file). If
  not, it shows a warning in the UI, for example "Not in a Git repository." The warning is
  informational only: it does not block watching and requires no confirmation. The warning is
  cleared when a directory inside a repository is selected.
- FR-1.5: The directory and whether watching was active are saved to a settings file in
  the user’s config directory whenever they change. On launch they are restored. If the saved
  directory still exists and is writable and watching was active, watching resumes. Otherwise the
  app shows the error from FR-1.2 and stays stopped. The Git check (FR-1.4) runs on restore too.

### 5.2 Watching
- FR-2.1: Watch only the top level of the selected directory (non-recursive).
- FR-2.2: React only to `.py` and `.iqpython` files (case-sensitive extension match). Events
  for all other files are ignored.
- FR-2.3: Handle these events: create, write/modify, and rename-into-directory. Editors that save
  atomically (write a temp file, then rename) must still trigger a sync. Permission changes alone
  do not trigger a sync.
- FR-2.4: Debounce events per file: wait 300 ms after the last event for a given file before
  syncing, so a single save causing several events produces one sync.
- FR-2.5: Deleting a file is never synced. The app logs `Ignored deletion of <file>` and leaves
  the other file of the pair untouched.
- FR-2.6: The app keeps a SHA-256 hash of the last known content of each watched file. It stores
  the hash when it reads a file, and when it writes one. Hashes are kept in memory. When a file
  event arrives and the file's current content hash equals the stored hash, the event is ignored.
  A save that does not change the content therefore triggers no conversion.
- FR-2.7: Writing a file in one direction never triggers a sync in the other direction. Because
  the app stores the hash of every file it writes (FR-2.6), the resulting event is ignored, so a
  Python → IQ update does not cause an IQ → Python update, and the reverse.
- FR-2.8: Starting to watch does not sync existing files. It only records their hashes (FR-2.6).
  Only later changes trigger syncs.
- FR-2.9: If both files of a pair change within the debounce window, the later write wins: the
  file that changed last is synced in its own direction, and the other file is then overwritten
  only if its content differs from the result. Each update is logged. This is a known limitation.

### 5.3 Pairing and Generation

#### Python → IQ direction
- FR-3.1: For a changed `Foo.py`, the target is `Foo.iqpython` in the same directory (same
  basename).
- FR-3.2: Read the `.py` file as UTF-8 text.
- FR-3.3: Build `textContent` from the Python source using the encoding rules in §6.
- FR-3.4: Update **only** the `textContent` field of the target. All other fields and their
  values are preserved (the existing `.iqpython` is the template). See §6.2 for how this is
  achieved.
- FR-3.5: Write the result atomically: write to a temp file in the same directory, then rename
  over the target. Never leave a partially written `.iqpython`.
- FR-3.6: Writing a file is always a result of a changed source. Unchanged source content never
  causes a write.

#### IQ → Python direction
- FR-3.7: For a changed `Foo.iqpython`, the target is `Foo.py` in the same directory (same
  basename).
- FR-3.8: Parse the `.iqpython` file as JSON and read its `textContent` string. Decode it per §6.1
  to obtain the Python source.
- FR-3.9: Write the Python source to `Foo.py` as UTF-8 without a BOM, with LF line endings,
  atomically (temp file plus rename), as in FR-3.5. If the existing `.py` has different bytes, for
  example CRLF line endings, it is replaced even when its text is otherwise unchanged.
- FR-3.10: When syncing IQ → Python, the `.iqpython` file is not modified.

### 5.4 Missing Target
- FR-4.1: If `Foo.iqpython` does not exist, create it from the built-in template, i.e. the
  structure of `Sample.iqpython` embedded in the app as Go source, with `textContent` set to the
  Python source. Log `Created Foo.iqpython from template`.
- FR-4.2: If `Foo.iqpython` exists but is not valid JSON, do not overwrite it. Log an error and
  skip.
- FR-4.3: When syncing IQ → Python and `Foo.py` does not exist, create it with the decoded
  `textContent`. Log `Created Foo.py from Foo.iqpython`. No template is needed.
- FR-4.4: When syncing IQ → Python, if `Foo.iqpython` is not valid JSON, or has no string
  `textContent` field, log an error and leave `Foo.py` untouched.

### 5.5 Status and Errors
- FR-5.1: Any failure (unreadable `.py`, unwritable target, invalid JSON, rename failure) is
  logged and does not stop the watcher.
- FR-5.2: Watcher-level failures (e.g., directory removed) set the status to `Error: <message>`
  and stop watching.
- FR-5.3: The app must not overwrite a file with an empty or partial result because of a read
  error.

## 6. Data Format

### 6.1 `textContent` encoding
The `textContent` value is a standard JSON string whose decoded value is the **exact content** of
the `.py` file, with these rules:

- Line endings: CRLF (`\r\n`) and lone CR are normalized to LF (`\n`) before encoding. The sample
  uses `\n`.
- Line breaks are encoded as `\n` (see sample: each line ends with `\n`, including the final line
  if the `.py` file ends with a newline).
- Tabs are encoded as `\t` (sample: `\tProject:`).
- Double quotes → `\"`, backslash → `\\`. Other control characters → `\uXXXX`.
- Non-ASCII characters are written as-is (UTF-8), not `\uXXXX`.
- `<`, `>`, and `&` are escaped as `\u003c`, `\u003e`, and `\u0026` by `encoding/json`. This is
  valid JSON that decodes to the same text, so it is accepted.
- An empty `.py` file produces `"textContent": ""`.

**Decoding (IQ → Python):** the `textContent` JSON string is decoded to its raw value, which is
then written to the `.py` file. Line endings in the output are LF (see FR-3.9). Escapes are
interpreted exactly as a standard JSON parser does, including `\uXXXX` for control characters.

### 6.2 Preserving the rest of the file
The app must not change the file beyond `textContent`:

- Parse the template into a Go struct that mirrors the fields of `Sample.iqpython`, then replace
  only the `TextContent` field.
- Serialize with `encoding/json` as compact JSON. Key order is not preserved and no indentation
  is written; the file is valid JSON with the same field names and values. Do not hand-roll
  escaping or indentation.
- Values such as `false`, `null`, `[]`, nested `aiVisionSettings`, `sdkVersion`, `appVersion`,
  `fileFormat`, `slot`, etc. are copied verbatim, including their types and empty arrays.
- Verify that parsing the regenerated file gives the same values as the original, apart from
  `textContent`.

### 6.3 Reference: `Sample.iqpython` fields
| Field | Behavior |
|---|---|
| `mode` | Keep. |
| `hardwareTarget` | Keep. |
| `textContent` | **Regenerated** from `.py`. |
| `textLanguage` | Keep. |
| `robotConfig`, `aiVisionSettings`, etc. | Keep as-is. |
| `slot`, `platform`, `sdkVersion`, `appVersion`, `fileFormat`, `targetBrainGen`, `v5SoundsEnabled` | Keep as-is. |

## 7. Architecture

### 7.0 Technology
- Language: Go (backend and HTML rendering).
- Desktop shell: [Wails v3](https://v3.wails.io/) hosts the UI in a native window.
- UI: [Templ](https://templ.guide/) for type-checked HTML components, [HTMX](https://htmx.org/)
  for partial page updates, and [Tailwind CSS](https://tailwindcss.com/) for styling. There is no
  JavaScript build step and no Node.js dependency.
- Other dependencies: `github.com/fsnotify/fsnotify` for file watching.

### 7.1 Project layout (proposed)
All Go and frontend sources live under `src/`. The repo root contains only the spec and the
top-level task file.
```
Tools for Vex IQ/
├── SPEC.md
├── Taskfile.yml             # Task definitions (see §11.1); delegates to src/
└── src/
    ├── main.go              # Wails v3 application setup, window config, asset handler
    ├── ui/                  # Templ components and HTTP handlers for HTMX
    │   ├── page.templ       # Single-screen layout and fragments
    │   └── handler.go       # Routes: page, static assets, HTMX endpoints
    ├── iqsync/              # Python ↔ IQPython sync tool
    │   ├── watcher.go       # fsnotify watcher, per-file debounce, content hashes
    │   ├── sync.go          # Pair sync in both directions (§5.3, §5.4)
    │   ├── iqpython.go      # IQPython struct, template, and textContent encoding (§6)
    │   └── atomic.go        # Atomic file writes (FR-3.5)
    ├── styles/input.css     # Tailwind input (produces static/app.css)
    ├── static/              # htmx.min.js (vendored); app.css (generated)
    ├── build/               # Platform metadata: Info.plist, icons, Windows manifest
    └── go.mod
```
Each tool lives in its own directory under `src/` (for example, `iqsync/`) and has its own section
on the main screen.

### 7.2 Backend (Go)
- Uses `github.com/fsnotify/fsnotify` for file watching.
- The Wails v3 asset handler serves the page, `/static/` files, and the HTMX endpoints. The
  endpoints return HTML fragments, not JSON:
  - `POST /pick-directory` — opens the native folder picker (`app.Dialog.OpenFile`) and returns
    the directory field. A cancel returns `204 No Content`, which leaves the field unchanged.
  - `POST /start` / `POST /stop` — start or stop watching both directions.
- Live status and log entries are pushed from the server with HTMX’s SSE extension.
- Watcher runs in its own goroutine; state is guarded by a mutex.
- Settings (FR-1.5) are stored as `settings.json` under `os.UserConfigDir()` in a folder named
  `Tools for Vex IQ`. Writes are atomic.

### 7.3 Frontend
- Single page with no routing. Each UI section is a Templ component that HTMX can replace.
- Controls use `hx-post`; the server re-renders the affected fragment.
- Styling: Tailwind utilities only; system font stack; no background colors.

## 8. Non-Functional Requirements
- NFR-1: A save-to-regeneration latency under ~1 s for typical files (< 1 MB).
- NFR-2: The watcher must not consume CPU while idle (event-driven, no polling).
- NFR-3: Writes are atomic (temp file plus rename) so that VEXcode IQ or any other reader never
  sees a partially written `.iqpython`.
- NFR-4: No network access is required.
- NFR-5: The app writes only the target of a change: `.iqpython` when a `.py` changed, and `.py`
  when an `.iqpython` changed (FR-2.7).

## 9. Error Handling Summary

| Situation | Behavior |
|---|---|
| Directory missing or not writable | Show error; watching does not start. |
| Directory not inside a Git repository | Show warning (FR-1.4); watching still starts. |
| `.py` unreadable | Log error; skip. |
| `.iqpython` invalid JSON | Log error; do not overwrite. |
| `.iqpython` missing (Python → IQ) | Create from template; log. |
| `.py` missing (IQ → Python) | Create from `textContent`; log. |
| `.iqpython` has no string `textContent` (IQ → Python) | Log error; leave `.py` untouched. |
| File deleted while watching | Log `Ignored deletion`; nothing else changes (FR-2.5). |
| Write/rename failure | Log error; remove temp file; original untouched. |
| Directory removed while watching | Status `Error`; stop watching. |
| Saved directory missing or not writable on launch | Show error; stay stopped. |
| Settings file missing, unreadable, or invalid | Use defaults (no directory, stopped); log. |

## 10. Testing

- **Unit tests (Go)**
  - `textContent` encoding: LF, CRLF, tabs, quotes, backslashes, unicode, `<`/`>`/`&`, empty file,
    file without trailing newline.
  - Round-trip: `textContent` from the template decodes to the same source, and encoding that
    source again gives the same JSON values.
  - Field preservation: all non-`textContent` fields keep their values.
  - Pairing: a `.py` event syncs `Foo.iqpython`, an `.iqpython` event syncs `Foo.py`, and events
    for other files are ignored.
  - Debounce: multiple rapid writes produce one sync.
  - Invalid JSON target is not overwritten.
  - IQ → Python decoding: `textContent` from the template decodes to the original Python source;
    escapes (`\n`, `\t`, `\"`, `\\`, `\uXXXX`) decode correctly.
  - Round-trip across directions: Python → IQ followed by IQ → Python reproduces the original `.py`
    (with LF line endings).
  - Loop prevention: a write by the app does not cause a sync in the other direction, and an event
    whose content hash is unchanged does not sync.
  - Startup: existing files are hashed but not synced when watching starts.
  - Deletion: deleting either file is logged and leaves the other file untouched.
  - Settings: save and restore round trip; a missing or invalid settings file falls back to defaults.
- **Manual tests**
  - Pick a directory, start, edit `.py` in an editor, confirm `.iqpython` opens correctly in
    VEXcode IQ.
  - Atomic save from an editor (e.g., VS Code) triggers sync.
  - Stop watching; further edits do nothing.
  - Quit and relaunch: the directory and watching state are restored.
  - Edit `.py` in an editor; confirm `.iqpython` updates and `.py` is not rewritten.
  - Edit `.iqpython` in VEXcode IQ; confirm the matching `.py` updates and `.iqpython` is not rewritten.
  - Save a file without changing it; confirm no log entry and no write.

## 11. Build and Run
- `task build` produces a native binary for the current platform. On macOS, `task app` also
  assembles a `.app` bundle. Windows produces an `.exe`. No installers are built.
- Behavior must be identical on all three platforms. Path handling uses `path/filepath`, and file
  watching uses fsnotify’s platform backends.
- Go version: as required by the Wails v3 release in use (see its `go.mod`).

### 11.1 Taskfile
`Taskfile.yml` at the repo root is the single Taskfile for the project; there must not be another
Taskfile in `src/` or elsewhere. It is written by hand, and each task delegates to `src/` where
needed.

| Task | Purpose | Platforms |
|---|---|---|
| `setup` | Installs all development dependencies on a development machine: Xcode Command Line Tools (if missing) and Homebrew packages `go`, `go-task`, `tailwindcss`, `librsvg`, and `imagemagick`. Safe to re-run. | macOS only (other platforms exit with a message) |
| `generate` | Generates Templ code, the Tailwind stylesheet, and app icons from `src/logo.svg`. | All |
| `dev` | Runs the app from source. | All |
| `test` | Runs Go unit tests. | All |
| `build` | Builds the binary for the current platform into `bin/`. | All |
| `app` | Assembles `bin/Tools for Vex IQ.app` from the binary and `build/darwin/`. | macOS only |

Each task must have a `desc` so that `task --list` is self-documenting.
