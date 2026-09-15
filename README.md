# json-log-parser

A small CLI that reads JSON log lines (from stdin or a file) and prints them in a debug-friendly format: aligned table, one-line, logfmt, or pretty-printed JSON — with ANSI colors by level.

## Features

- Auto-detects common time/level/message keys (`time`, `ts`, `timestamp`, `@timestamp`, `Time`; `level`, `lvl`, `severity`, `Level`; `msg`, `message`, `text`, `Message`)
- Four output modes: `-table`, `-oneline` (default), `-logfmt`, `-pprint`
- Colorized output by log level (ERROR/FATAL red, WARN yellow, INFO blue, DEBUG/TRACE gray)
- Filter by level (`-level`) or substring (`-grep`)
- Streams output line by line, so it works with `tail -f` / piped live processes
- `-no-color` to disable ANSI colors

## Requirements

- Go 1.21+

## Install

Clone the repo and build/install with the included Makefile:

```bash
git clone <repo-url>
cd json-log-parser
make build      # builds ./bin/json-log-parser
# or
make install    # go install . — puts the binary in $GOBIN (or $GOPATH/bin)
```

Alternatively, without the Makefile:

```bash
go build -o bin/json-log-parser .
# or
go install .
```

Make sure `$GOBIN` (or `$GOPATH/bin`) is on your `PATH` to run `json-log-parser` from anywhere after `go install`.

## Usage

```bash
# pipe from stdin
tail -f app.log | json-log-parser -table

# read from a file
json-log-parser -table app.log

# one-line format (default)
json-log-parser app.log

# logfmt
json-log-parser -logfmt app.log

# pretty-printed JSON per entry
json-log-parser -pprint app.log

# filter by level
json-log-parser -level error app.log

# filter by substring in the raw line
json-log-parser -grep "mirror-detection" app.log

# disable colors (e.g. when redirecting to a file)
json-log-parser -no-color -table app.log > out.txt
```

## Flags

| Flag         | Description                                              |
|--------------|-----------------------------------------------------------|
| `-table`     | Print as an aligned table                                  |
| `-oneline`   | Print one line per entry: `time level msg key=val ...` (default) |
| `-logfmt`    | Print as `key=val key=val ...`                              |
| `-pprint`    | Pretty-print each entry as indented, colorized JSON         |
| `-level`     | Only show entries matching this level (case-insensitive)   |
| `-grep`      | Only show entries whose raw line contains this substring    |
| `-no-color`  | Disable ANSI colors                                         |

Only one of `-table`, `-oneline`, `-pprint`, `-logfmt` may be used at a time.

Lines that aren't valid JSON are printed as-is (raw), so the parser degrades gracefully on mixed log output.
