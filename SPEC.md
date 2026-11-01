# Tools for Vex IQ — Specification

## 1. Overview

**Purpose:** Tools for Vex IQ is a desktop app that helps VEX IQ developers work with their
project files. It is designed to hold multiple small tools over time.

### Tools

- **IQ Sync:** VEXcode IQ stores a Python program inside a `.iqpython` project file, which makes
  it awkward to edit the code in a normal editor. This tool keeps each `.py` file and its
  matching `.iqpython` file (same basename) in step, in both directions.
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
| `.iqpython` file | VEXcode IQ project file (JSON) that wraps Python source in `textContent`. |
| Basename | File name without its extension. `Foo.py` and `Foo.iqpython` share basename `Foo`. |
| Watched directory | The single directory the user selects in the app. |
| Pair | The `.py` and `.iqpython` files that share a basename. |
| Sync direction | Python → IQ or IQ → Python. Both are active while watching (§5.2). |
| Template | Built-in `.iqpython` structure for new files, derived from `Sample.iqpython` (§5.4). |

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
- Remember the directory and running state across launches (FR-1.5).

### Non-Goals (v1)
- Recursive watching of subdirectories.
- Editing Python code inside the app.
- Multiple simultaneous watched directories.
- Auto-update and code signing.

## 4. User Experience

### 4.1 Single Screen Layout

The app opens to one window titled "Tools for Vex IQ". The window contains, top to bottom:

1. **Header:** the tool name "IQ Sync", with a short description in smaller text beneath it.
2. **Directory row:** a read-only label showing the selected directory path (or "No directory
   selected"), and a **Choose Directory…** button. When the selected directory is not inside a
   Git repository, a warning is shown below this row (FR-1.4).
3. **Control row:** a **Start Watching** / **Stop Watching** toggle button. Disabled until a
   directory is selected.
4. **Status line:** one of `Idle`, `Watching <path>`, or `Error: <message>`.
5. **Activity log:** a scrollable list (newest at bottom) of events, each with a timestamp:
   - `Started watching <path>` and `Stopped watching`
   - `Updated Foo.iqpython from Foo.py` (Python → IQ)
   - `Updated Foo.py from Foo.iqpython` (IQ → Python)
   - `Created Foo.iqpython from template` (see §5.4)
   - `Created Foo.py from Foo.iqpython` (see §5.4)
   - `Ignored deletion of Foo.py` / `Ignored deletion of Foo.iqpython` (see FR-2.5)
   - `Error: <message>` for a file that could not be synced (for example invalid JSON)
   - The log is capped at the most recent 500 entries.

### 4.2 Interaction Rules
- Watching starts when the user clicks **Start Watching**, or on launch if it was active when
  the app last quit (FR-1.5).
- The directory can only be changed while stopped.
- Status moves between `Idle` and `Watching`, and to `Error` on watcher-level failure (FR-5.2).
- Sync latency follows NFR-1. Usage instructions are in `README.md`.

## 5. Functional Requirements

### 5.1 Directory Selection
- FR-1.1: The user can select exactly one directory via a native folder picker.
- FR-1.2: The selected directory must exist and be writable. The app checks this with a
  temporary probe file. Otherwise it shows an error and does not start watching.
- FR-1.3: Watching always syncs in both directions (see §5.2). There is no direction setting.
- FR-1.4: After a directory is selected, the app checks whether it is inside a Git
  repository, that is, whether it or any ancestor directory contains a `.git` entry (directory
  or file). If not, it shows a warning in the UI, for example "Not in a Git repository." The
  warning is informational only: it does not block watching and requires no confirmation. The
  warning is cleared when a directory inside a repository is selected.
- FR-1.5: The directory and whether watching was active are saved to a settings file
  in the user’s config directory whenever they change. On launch they are restored. If the saved
  directory still exists and is writable and watching was active, watching resumes. Otherwise
  the app shows the error from FR-1.2 and stays stopped. The Git check (FR-1.4) runs on restore
  too.

### 5.2 Watching
- FR-2.1: Watch only the top level of the selected directory (non-recursive).
- FR-2.2: React only to `.py` and `.iqpython` files (case-sensitive extension match, with a
  non-empty name before the extension). Events for all other files are ignored.
- FR-2.3: Handle these events: create, write/modify, and rename-into-directory. Editors that
  save atomically (write a temp file, then rename) must still trigger a sync. Permission changes
  alone do not trigger a sync.
- FR-2.4: Debounce events per file: wait 300 ms after the last event for a given file before
  syncing, so a single save causing several events produces one sync.
- FR-2.5: Deleting a file is never synced. The app logs `Ignored deletion of <file>` and leaves
  the other file of the pair untouched.
- FR-2.6: The app keeps a SHA-256 hash of the last known content of each watched file. It stores
  the hash when it reads a file, and when it writes one. Hashes are kept in memory. When a file
  event arrives and the file’s current content hash equals the stored hash, the event is
  ignored. A save that does not change the content therefore triggers no conversion.
- FR-2.7: Writing a file in one direction never triggers a sync in the other direction. Because
  the app stores the hash of every file it writes (FR-2.6), the resulting event is ignored, so a
  Python → IQ update does not cause an IQ → Python update, and the reverse.
- FR-2.8: Starting to watch does not sync existing files. It only records their hashes
  (FR-2.6). Only later changes trigger syncs.
- FR-2.9: If both files of a pair change within the debounce window, the later change wins. The
  earlier file's sync is skipped while the other file is still pending, and the later file syncs
  and logs its own message. A later event with unchanged content (such as a touch) does not
  count, so the earlier change still syncs.

### 5.3 Pairing and Generation

#### Python → IQ direction
- FR-3.1: For a changed `Foo.py`, the target is `Foo.iqpython` in the same directory (same
  basename).
- FR-3.2: Read the `.py` file as UTF-8 text.
- FR-3.3: Build `textContent` from the Python source using the encoding rules in §6.
- FR-3.4: Update **only** the `textContent` field of the target. All other fields keep their
  values. The existing `.iqpython` is the template. See §6.2 for the limit on unknown fields.
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
  atomically (temp file plus rename), as in FR-3.5. If the existing `.py` has different bytes,
  for example CRLF line endings, it is replaced even when its text is otherwise unchanged.
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
- FR-5.2: Watcher-level failures (for example, the directory is removed) set the status to
  `Error: <message>` and stop watching.
- FR-5.3: The app must not overwrite a file with an empty or partial result because of a read
  error.

## 6. Data Format

### 6.1 `textContent` encoding
The `textContent` value is a standard JSON string whose decoded value is the **exact content** of
the `.py` file, with these rules:

- Line endings: CRLF (`\r\n`) and lone CR are normalized to LF (`\n`) before encoding. The
  sample uses `\n`.
- Line breaks are encoded as `\n` (see sample: each line ends with `\n`, including the final
  line if the `.py` file ends with a newline).
- Tabs are encoded as `\t` (sample: `\tProject:`).
- Double quotes → `\"`, backslash → `\\`. Other control characters → `\uXXXX`.
- Non-ASCII characters are written as-is (UTF-8), not `\uXXXX`. The exception is U+2028 and
  U+2029, which `encoding/json` escapes as `\u2028` and `\u2029`.
- `<`, `>`, and `&` are escaped as `\u003c`, `\u003e`, and `\u0026` by `encoding/json`. This is
  valid JSON that decodes to the same text, so it is accepted.
- An empty `.py` file produces `"textContent": ""`.

**Decoding (IQ → Python):** the `textContent` JSON string is decoded to its raw value, which is
then written to the `.py` file. Line endings in the output are LF (see FR-3.9). Escapes are
interpreted exactly as a standard JSON parser does, including `\uXXXX` for control characters.

### 6.2 Preserving the rest of the file
The app must not change the file beyond `textContent`:

- Parse the file into the `IQPython` Go struct, which mirrors the fields of `Sample.iqpython`,
  then replace only the `TextContent` field.
- Serialize with `encoding/json` as compact JSON. Keys are written in struct order, which matches
  `Sample.iqpython`. No indentation is written. Do not hand-roll escaping or indentation.
- Values such as `false`, `null`, `[]`, and the nested `aiVisionSettings` object keep their types
  and empty arrays.
- **Limit:** a key that is not in the struct is dropped on the next write, including keys nested
  in `aiVisionSettings`. If VEXcode adds fields, they are lost. Keeping unknown keys would need
  a different parse (for example, `map[string]json.RawMessage`). This is an open decision.
- Tests check that the regenerated file has the same field values as the original, apart from
  `textContent`.

### 6.3 Reference: `Sample.iqpython` fields

| Field | Behavior |
|---|---|
| `textContent` | **Regenerated** from `.py`. |
| `mode`, `hardwareTarget`, `textLanguage` | Keep. |
| `robotConfig`, `aiVisionSettings` | Keep as-is, including nested values. |
| `slot`, `platform`, `sdkVersion`, `appVersion`, `fileFormat` | Keep as-is. |
| `targetBrainGen`, `v5SoundsEnabled` | Keep as-is. |

## 7. Architecture

### 7.0 Technology
- Language: Go (backend and HTML rendering). Go 1.26 (see `src/go.mod`).
- Desktop shell: [Wails v3](https://v3.wails.io/) hosts the UI in a native window.
- UI: [Templ](https://templ.guide/) for type-checked HTML components, [HTMX](https://htmx.org/)
  with its SSE extension for partial page updates, and [Tailwind CSS](https://tailwindcss.com/)
  for styling. There is no JavaScript build step and no Node.js dependency.
- Other dependencies: `github.com/fsnotify/fsnotify` for file watching.

### 7.1 Project layout
All Go and frontend sources live under `src/`. The repo root holds the docs, the task file, and
the reference `Sample.iqpython`. Generated files are listed in `.gitignore`.
```
VexIQ Tools/
├── SPEC.md
├── README.md
├── Sample.iqpython          # Reference VEXcode IQ project (see §6.3)
├── Taskfile.yml             # Task definitions (see §11.1)
└── src/
    ├── main.go              # Wails v3 app setup, window, asset handler, folder picker
    ├── go.mod
    ├── logo.svg             # Source for all app icons
    ├── build/               # Wails build metadata: config.yml, darwin/, windows/
    ├── styles/input.css     # Tailwind input (produces static/app.css)
    ├── static/              # htmx.min.js, htmx-sse.js (vendored); app.css (generated)
    ├── ui/                  # Templ components and HTTP handlers for HTMX
    │   ├── page.templ       # Single-screen layout and fragments
    │   ├── screen.go        # UI state, activity log, SSE updates, start/stop checks
    │   └── handler.go       # Routes: page, static assets, HTMX endpoints, SSE
    └── iqsync/              # Python ↔ IQPython sync tool
        ├── watcher.go       # fsnotify watcher, per-file debounce, content hashes
        ├── sync.go          # Pair sync in both directions (§5.3, §5.4)
        ├── iqpython.go      # IQPython struct, template, and textContent encoding (§6)
        └── atomic.go        # Atomic file writes (FR-3.5)
```
Each tool lives in its own directory under `src/` (today, only `iqsync/`) and has its own
section on the main screen.

### 7.2 Backend (Go)
- Uses `github.com/fsnotify/fsnotify` for file watching.
- The Wails v3 asset handler serves the page, `/static/` files, and the HTMX endpoints. The
  endpoints return HTML fragments, not JSON:
  - `GET /` — the page.
  - `POST /pick-directory` — opens the native folder picker and returns the panel fragment with
    the chosen directory. A cancel returns `204 No Content`, which leaves the panel unchanged.
  - `POST /start` / `POST /stop` — start or stop watching both directions. Return the panel
    fragment.
  - `GET /events` — SSE stream. Sends the full `panel` and `log` state on connect, then again on
    every change.
- Watcher runs in its own goroutine; state is guarded by a mutex.
- Settings (FR-1.5) are stored as `settings.json` under `os.UserConfigDir()` in a folder
  named `Tools for Vex IQ`, with atomic writes. The file holds only the directory and the
  watching flag.

### 7.3 Frontend
- Single page with no routing. Each UI section is a Templ component that HTMX can replace.
- Controls use `hx-post`; the server re-renders the panel fragment. The status and the log are
  pushed through SSE.
- Styling: Tailwind utilities only. The system font stack is set in `styles/input.css`. No
  background colors, except a subtle hover state on buttons.

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
| Directory removed while watching | Status shows `Error`; stop watching. |
| Saved directory missing or not writable on launch | Show error; stay stopped. |
| Settings missing, unreadable, or invalid | Use defaults (none, stopped); log. |

## 10. Testing

- **Implemented: Go unit tests**
  - `textContent` encoding: tabs, quotes, backslashes, `<`/`>`/`&`, unicode, control characters,
    and an empty file (`TestTextEncodingRoundTrip`). CRLF and lone CR are normalized to LF
    (`TestPythonToIQReplacesOnlyText`).
  - Round trip: Python → IQ → Python gives back the original source, with LF line endings
    (`TestPyToIQCreatesFromTemplate`).
  - Field preservation: non-`textContent` fields keep their values, and the output bytes match a
    re-encode of the same struct (`TestPythonToIQReplacesOnlyText`). The template matches
    `Sample.iqpython` (`TestNewIQPythonMatchesSampleFields`).
  - Invalid input: invalid UTF-8 and a missing or non-string `textContent` are rejected
    (`TestInvalidInputsRejected`).
  - Invalid JSON is not overwritten, in either direction
    (`TestPyToIQRefusesToOverwriteInvalidJSON`, `TestIQToPyLeavesPyUntouchedOnBadJSON`).
  - Unchanged content: no conversion for a save with the same content
    (`TestPyToIQUpdatesExistingAndSkipsWhenUnchanged`, `TestWatcherSkipsUnchangedSave`).
  - Loop prevention: a write by the app does not cause a sync in the other direction
    (`TestWatcherDoesNotEchoBetweenDirections`).
  - Startup: existing files are hashed but not synced when watching starts
    (`TestWatcherDoesNotSyncExistingFilesOnStart`).
  - Deletion: deleting a `.py` is logged and leaves the `.iqpython` untouched
    (`TestWatcherLogsDeletionAndIgnoresIt`).
  - Both files changed in one debounce window: the later change wins
    (`TestWatcherLaterChangeWinsWhenBothFilesChange`).
  - An atomic save is not logged as a deletion (`TestWatcherAtomicSaveIsNotADeletion`).
  - Atomic writes leave no temp file (`TestWriteFileAtomicReplacesAndLeavesNoTemp`).
  - Restore on launch, saving on stop, and the Git warning (`src/ui/screen_test.go`).
  - UI: page, static asset, directory picker fragment, start and stop, log cap, and the SSE
    stream (`src/ui/handler_test.go`, `TestLogIsCapped`).
- **Planned: Go unit tests**
  - Events for other files are ignored. Debounce: multiple rapid writes produce one sync.
  - Deleting a `.iqpython` is logged and leaves the `.py` untouched.
- **Manual tests**
  - Pick a directory, start, edit `.py` in an editor, and confirm `.iqpython` opens correctly in
    VEXcode IQ.
  - An atomic save from an editor (e.g., VS Code) triggers a sync.
  - Stop watching; further edits do nothing.
  - Quit and relaunch: the directory and watching state are restored.
  - Edit `.py` in an editor; confirm `.iqpython` updates and `.py` is not rewritten.
  - Edit `.iqpython` in VEXcode IQ; confirm the matching `.py` updates.
  - Save a file without changing it; confirm no log entry and no write.

## 11. Build and Run
- `task build` produces a native binary for the current platform. On macOS, `task app` also
  assembles a `.app` bundle. Windows produces an `.exe`. No installers are built.
- Behavior must be identical on all three platforms. Path handling uses `path/filepath`, and file
  watching uses fsnotify’s platform backends.

### 11.1 Taskfile
`Taskfile.yml` at the repo root is the only Taskfile in the project. It is written by hand, and
each task delegates to `src/` where needed. The helper tasks `css`, `templ`, and `icons` are run
by `generate`, and can also be run directly.

| Task | Purpose | Platforms |
|---|---|---|
| `setup` | Installs the development tools listed below. Safe to re-run. | macOS only |
| `generate` | Generates Templ code, the Tailwind stylesheet, and app icons. | All |
| `dev` | Runs the app from source. | All |
| `test` | Runs Go unit tests. | All |
| `build` | Builds the binary for the current platform into `bin/`. | All |
| `app` | Assembles the macOS `.app` bundle in `bin/`. | macOS only |

`setup` installs Xcode Command Line Tools (if missing), then the Homebrew packages `go`,
`go-task`, `tailwindcss`, `librsvg`, and `imagemagick`. Other platforms exit with a message.

Each task must have a `desc` so that `task --list` is self-documenting.
