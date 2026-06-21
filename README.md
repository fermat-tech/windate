# windate

A Windows port of the Unix `date` command, compatible with GNU date. Displays or formats the current date and time, with support for custom format strings, human-readable date expressions, and timezone control.

---

## Installation

**Option 1 — Download binary**

Download `windate.exe` from the [latest release](https://github.com/fermat-tech/windate/releases/latest) and place it anywhere on your `PATH`.

**Option 2 — Go install**

If you have [Go 1.22+](https://go.dev/dl/) installed:

```powershell
go install github.com/fermat-tech/windate@latest
```

This builds and places `windate.exe` in your `GOBIN` directory (typically `%USERPROFILE%\go\bin`), which should already be on your `PATH`.

---

## Usage

```
windate [OPTION]... [+FORMAT]
```

With no options, prints the current local date and time:

```
Fri Jan  2 15:04:05 CST 2026
```

Supply a `+FORMAT` argument to control the output:

```
windate "+%Y-%m-%d"
2026-01-02
```

---

## Options

| Option | Long form | Description |
|---|---|---|
| `-d STRING` | `--date=STRING` | Display time described by STRING instead of now |
| `-f FILE` | `--file=FILE` | Like `-d`, but read one date string per line from FILE |
| `-r FILE` | `--reference=FILE` | Display the last modification time of FILE |
| `-u` | `--utc`, `--universal` | Print time in Coordinated Universal Time (UTC) |
| `-I[FMT]` | `--iso-8601[=FMT]` | Output in ISO 8601 format (see below) |
| `-R` | `--rfc-email` | Output in RFC 5322 format |
| | `--rfc-3339=FMT` | Output in RFC 3339 format (see below) |
| | `--debug` | Print the parsed date to stderr (useful with `-d`) |

---

## Format Strings

When a `+FORMAT` argument is given, `windate` replaces `%` sequences with date/time values.

### Date

| Sequence | Description | Example |
|---|---|---|
| `%Y` | Full year | `2026` |
| `%y` | Last two digits of year | `26` |
| `%C` | Century | `20` |
| `%m` | Month (01–12) | `01` |
| `%B` | Full month name | `January` |
| `%b`, `%h` | Abbreviated month name | `Jan` |
| `%d` | Day of month, zero-padded (01–31) | `02` |
| `%e` | Day of month, space-padded ( 1–31) | ` 2` |
| `%j` | Day of year (001–366) | `002` |
| `%q` | Quarter of year (1–4) | `1` |
| `%F` | Full date; same as `%Y-%m-%d` | `2026-01-02` |
| `%D` | Date; same as `%m/%d/%y` | `01/02/26` |
| `%x` | Locale date; same as `%m/%d/%Y` | `01/02/2026` |

### Time

| Sequence | Description | Example |
|---|---|---|
| `%H` | Hour (00–23) | `15` |
| `%I` | Hour (01–12) | `03` |
| `%k` | Hour, space-padded ( 0–23) | `15` |
| `%l` | Hour, space-padded ( 1–12) | ` 3` |
| `%M` | Minute (00–59) | `04` |
| `%S` | Second (00–60) | `05` |
| `%N` | Nanoseconds (000000000–999999999) | `000000000` |
| `%p` | AM or PM | `PM` |
| `%P` | am or pm | `pm` |
| `%r` | 12-hour time; same as `%I:%M:%S %p` | `03:04:05 PM` |
| `%R` | 24-hour hour and minute; same as `%H:%M` | `15:04` |
| `%T` | Time; same as `%H:%M:%S` | `15:04:05` |
| `%X` | Locale time; same as `%H:%M:%S` | `15:04:05` |
| `%s` | Seconds since 1970-01-01 00:00:00 UTC | `1735830245` |

### Weekday

| Sequence | Description | Example |
|---|---|---|
| `%A` | Full weekday name | `Friday` |
| `%a` | Abbreviated weekday name | `Fri` |
| `%u` | Day of week (1–7); Monday = 1 | `5` |
| `%w` | Day of week (0–6); Sunday = 0 | `5` |

### Week Numbers

| Sequence | Description |
|---|---|
| `%U` | Week number of year, Sunday as first day (00–53) |
| `%W` | Week number of year, Monday as first day (00–53) |
| `%V` | ISO 8601 week number, Monday as first day (01–53) |
| `%G` | ISO 8601 week-based year |
| `%g` | ISO 8601 week-based year, last two digits |

### Timezone

| Sequence | Description | Example |
|---|---|---|
| `%Z` | Timezone abbreviation | `CST` |
| `%z` | Numeric offset (`+hhmm`) | `-0600` |
| `%:z` | Numeric offset (`+hh:mm`) | `-06:00` |
| `%::z` | Numeric offset (`+hh:mm:ss`) | `-06:00:00` |

### Special

| Sequence | Description |
|---|---|
| `%%` | A literal `%` |
| `%n` | A newline |
| `%t` | A tab |
| `%c` | Full date and time (`Mon Jan  2 15:04:05 2026`) |

### Padding Flags

By default, numeric fields are zero-padded. Place a flag immediately after `%` to change this:

| Flag | Effect |
|---|---|
| `-` | No padding |
| `_` | Space padding |
| `0` | Zero padding (default) |
| `^` | Uppercase |
| `#` | Opposite case |

Example: `%-d` gives `2` instead of `02`; `%_d` gives ` 2`.

---

## ISO 8601 Output (`-I`)

The `-I` flag accepts an optional precision level:

| Flag | Output |
|---|---|
| `-Idate` or `-I` | `2026-01-02` |
| `-Ihours` | `2026-01-02T15-06:00` |
| `-Iminutes` | `2026-01-02T15:04-06:00` |
| `-Iseconds` | `2026-01-02T15:04:05-06:00` |
| `-Ins` | `2026-01-02T15:04:05.000000000-06:00` |

---

## RFC 3339 Output (`--rfc-3339`)

| Value | Output |
|---|---|
| `date` | `2026-01-02` |
| `seconds` | `2026-01-02 15:04:05-06:00` |
| `ns` | `2026-01-02 15:04:05.000000000-06:00` |

---

## Date Strings (`-d`)

The `-d` option accepts a wide variety of human-readable date expressions.

### Keywords

```
windate -d "now"
windate -d "today"
windate -d "yesterday"
windate -d "tomorrow"
```

### Relative Offsets

```
windate -d "1 day ago"
windate -d "2 hours ago"
windate -d "3 weeks ago"
windate -d "+1 hour"
windate -d "-2 days"
windate -d "1 year 6 months ago"
```

### Weekday Navigation

```
windate -d "next monday"
windate -d "last friday"
windate -d "this wednesday"
```

### Absolute Dates

```
windate -d "2026-01-15"
windate -d "1/15/2026"
windate -d "1/1/2026"
windate -d "Jan 15 2026"
windate -d "January 15, 2026"
windate -d "15 Jan 2026"
```

### Unix Timestamps

```
windate -d "@0"           # 1970-01-01 00:00:00 UTC
windate -d "@1735830245"
```

### RFC and ISO Timestamps

```
windate -d "2026-01-02T15:04:05"
windate -d "Fri, 02 Jan 2026 15:04:05 -0600"
```

---

## Timezone

Set the `TZ` environment variable to display dates in a specific timezone. `windate` includes the full IANA timezone database and does not rely on Windows system timezone data.

```powershell
$env:TZ = "America/New_York"
windate

$env:TZ = "Europe/London"
windate "+%H:%M %Z"

$env:TZ = "GMT"
windate -d "1/1/2026"
```

Valid timezone names follow the IANA format, e.g. `America/Chicago`, `Asia/Tokyo`, `UTC`.

---

## Examples

```powershell
# Current date and time
windate

# ISO 8601 date
windate -I

# ISO 8601 with seconds precision
windate -Iseconds

# RFC 5322 (email) format
windate -R

# Custom format
windate "+Today is %A, %B %e %Y"

# UTC time
windate -u "+%Y-%m-%d %H:%M:%S %Z"

# Seconds since epoch
windate "+%s"

# File modification time
windate -r C:\Windows\System32\cmd.exe "+%F %T"

# Process multiple dates from a file
windate -f dates.txt "+%A, %B %e %Y"

# What day of the week was July 4, 1776?
windate -d "July 4, 1776" "+%A"
```

---

## Building from Source

Requires [Go 1.22+](https://go.dev/dl/).

```powershell
git clone https://github.com/fermat-tech/windate.git
cd windate
go build -o windate.exe .
```

---

## License

MIT
