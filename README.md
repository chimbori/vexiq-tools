# Tools for Vex IQ

A small desktop utility for VEX IQ developers. It keeps your plain Python (`.py`) files and
the VEXcode IQ project files (`.iqpython`) in sync, so you can edit code in any editor.

Runs on macOS, Windows, and Linux.

## Tools

### IQ Sync

VEXcode IQ stores your Python program inside a `.iqpython` project file. That makes the code
awkward to edit outside VEXcode. IQ Sync watches a folder and keeps each `.py` file and its
matching `.iqpython` file in step. Two files match when they have the same name apart from the
extension, for example `Robot.py` and `Robot.iqpython`.

Both directions are watched at the same time:

- **Python → IQ:** when you save a `.py` file, its `.iqpython` file is updated with the new code.
- **IQ → Python:** when you save a `.iqpython` file, its `.py` file is updated with the code from
  the project.

Only the code is changed. All other settings in a `.iqpython` file are preserved. A save that
doesn’t change the content does nothing.

## Using the app

1. Start Tools for Vex IQ.
2. Click **Choose Directory…** and select the folder that contains your `.py` and `.iqpython`
   files.
3. Click **Start Watching**.
4. Edit and save files as usual. Each sync appears in the activity log within about a second.
5. Click **Stop Watching** when you are done.

Notes:

- The app only looks at the top level of the folder. Subfolders are ignored.
- If the folder is not inside a Git repository, the app shows a warning. Watching still works.
  We recommend keeping your project in Git so you can review and undo the changes the sync makes.
- Watching does not start automatically. You must click **Start Watching** each time.
- Starting to watch does not change existing files. Only later saves are synced.
- You can’t change the folder while watching. Stop first.
- If a `.py` file has no matching `.iqpython` file, one is created from a built-in template.
- If a `.iqpython` file has no matching `.py` file, the `.py` file is created from the project’s code.
- Deleting a file does not delete its match.
- If a `.iqpython` file isn’t valid JSON, the app leaves it alone and logs an error.

### Troubleshooting

| Problem | What to check |
|---|---|
| Start Watching is disabled | Choose a directory first. |
| "Folder is not writable." | Check the folder permissions. |
| Log says `Error: … invalid character …` | Open the `.iqpython` file and check that it is valid JSON, or restore it from a backup. |
| A save doesn’t sync | Make sure you are watching the right folder and that the file is at the top level of the folder. |
| Changes don’t appear in VEXcode IQ | Close and reopen the project in VEXcode IQ. |

## Development

### Setup

On macOS, run this once on a new development machine:

```sh
task setup
```

This installs everything needed to build the app:

- Xcode Command Line Tools, if they are missing (macOS may show a dialog to confirm).
- `go`, `go-task`, `tailwindcss`, `librsvg`, and `imagemagick` through Homebrew. You need
  [Homebrew](https://brew.sh) first.

It is safe to run again; anything already installed is skipped.

`task setup` supports macOS only. On Windows or Linux, install the same tools manually: Go,
[Task](https://taskfile.dev/), and the [Tailwind CSS standalone CLI](https://tailwindcss.com/blog/standalone-cli).

### Layout

- `Taskfile.yml`: all build and run tasks. It is the only Taskfile in the project.
- `src/`: Go backend, `ui/` (Templ templates and HTTP handlers), `iqsync/` sync tool, `static/`
  (HTMX and generated CSS), and `styles/` (Tailwind input).

### Build and run

Run the tasks defined in the root `Taskfile.yml`:

```sh
task --list   # show available tasks
task dev      # run from source
task test     # run unit tests
task build    # build the binary into bin/
task app      # macOS only: build bin/Tools for Vex IQ.app
```
