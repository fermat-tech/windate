package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

var progName string
var localLoc *time.Location

func init() {
	progName = strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0]))
	// Go on Windows doesn't honour TZ; load it explicitly.
	if tz := os.Getenv("TZ"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			localLoc = loc
			time.Local = loc
		} else {
			fmt.Fprintf(os.Stderr, "%s: warning: unknown timezone %q\n", progName, tz)
		}
	}
	if localLoc == nil {
		localLoc = time.Local
	}
}

// extractISO pulls -I[fmt] and --iso-8601[=fmt] out of args before flag.Parse.
// Returns the ISO precision level ("" if not specified) and the remaining args.
func extractISO(args []string) (level string, rest []string) {
	for _, a := range args {
		switch {
		case a == "-I" || a == "--iso-8601":
			level = "date"
		case strings.HasPrefix(a, "-I") && len(a) > 2:
			level = expandISOLevel(a[2:])
		case strings.HasPrefix(a, "--iso-8601="):
			level = expandISOLevel(a[len("--iso-8601="):])
		default:
			rest = append(rest, a)
		}
	}
	return
}

// expandISOLevel expands single-letter abbreviations to their full level name.
func expandISOLevel(s string) string {
	switch s {
	case "h":
		return "hours"
	case "m":
		return "minutes"
	case "s":
		return "seconds"
	case "n":
		return "ns"
	default:
		return s
	}
}

func main() {
	// Extract -I / --iso-8601 manually before flag.Parse because Go's flag
	// package does not support optional-value flags or concatenated short flags
	// like -Is or -Iseconds.
	isoLevel, cleanArgs := extractISO(os.Args[1:])
	os.Args = append(os.Args[:1], cleanArgs...)

	var (
		dateStr = flag.String("d", "", "display time described by STRING, not 'now'")
		dateFile = flag.String("f", "", "like -d, use each line of DATEFILE")
		refFile  = flag.String("r", "", "display the last modification time of FILE")
		utc      = flag.Bool("u", false, "print or set Coordinated Universal Time (UTC)")
		rfc2822  = flag.Bool("R", false, "output date and time in RFC 5322 format")
		rfc3339  = flag.String("rfc-3339", "", "output date and time in RFC 3339 format (date|seconds|ns)")
		debug    = flag.Bool("debug", false, "annotate the parsed date, and warn about questionable usage")
	)

	// long-form aliases
	flag.StringVar(dateStr, "date", "", "display time described by STRING, not 'now'")
	flag.StringVar(dateFile, "file", "", "like -d, use each line of DATEFILE")
	flag.StringVar(refFile, "reference", "", "display the last modification time of FILE")
	flag.BoolVar(utc, "utc", false, "print or set Coordinated Universal Time (UTC)")
	flag.BoolVar(utc, "universal", false, "print or set Coordinated Universal Time (UTC)")
	flag.BoolVar(rfc2822, "rfc-email", false, "output date and time in RFC 5322 format")

	flag.Usage = usage
	flag.Parse()

	// Collect format string (first non-flag arg starting with '+')
	args := flag.Args()
	var formatArg string
	for _, a := range args {
		if strings.HasPrefix(a, "+") {
			formatArg = a[1:]
			break
		}
	}

	// Determine base times to display
	var times []time.Time

	switch {
	case *refFile != "":
		fi, err := os.Stat(*refFile)
		if err != nil {
			fatalf("cannot stat %q: %v", *refFile, err)
		}
		times = []time.Time{fi.ModTime()}

	case *dateFile != "":
		f, err := os.Open(*dateFile)
		if err != nil {
			fatalf("cannot open %q: %v", *dateFile, err)
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			t, err := parseDate(line)
			if err != nil {
				fatalf("invalid date %q: %v", line, err)
			}
			times = append(times, t)
		}

	case *dateStr != "":
		t, err := parseDate(*dateStr)
		if err != nil {
			fatalf("invalid date %q: %v", *dateStr, err)
		}
		if *debug {
			fmt.Fprintf(os.Stderr, "date: parsed %q as %s\n", *dateStr, t.Format(time.RFC3339))
		}
		times = []time.Time{t}

	default:
		times = []time.Time{time.Now()}
	}

	for _, t := range times {
		if *utc {
			t = t.UTC()
		}

		var out string
		switch {
		case isoLevel != "":
			out = formatISO(t, isoLevel)
		case *rfc2822:
			out = t.Format("Mon, 02 Jan 2006 15:04:05 -0700")
		case *rfc3339 != "":
			out = formatRFC3339(t, *rfc3339)
		case formatArg != "":
			out = applyFormat(t, formatArg)
		default:
			out = applyFormat(t, "%a %b %e %H:%M:%S %Z %Y")
		}
		fmt.Println(out)
	}
}

// ─── output formatters ────────────────────────────────────────────────────────

func formatISO(t time.Time, level string) string {
	switch level {
	case "date":
		return t.Format("2006-01-02")
	case "hours":
		return t.Format("2006-01-02T15Z07:00")
	case "minutes":
		return t.Format("2006-01-02T15:04Z07:00")
	case "seconds":
		return t.Format(time.RFC3339) // "2006-01-02T15:04:05Z07:00"
	case "ns":
		return t.Format(time.RFC3339Nano) // "2006-01-02T15:04:05.999999999Z07:00"
	default:
		fatalf("invalid argument %q for --iso-8601", level)
		return ""
	}
}

func formatRFC3339(t time.Time, level string) string {
	switch level {
	case "date":
		return t.Format("2006-01-02")
	case "seconds":
		return t.Format("2006-01-02 15:04:05-07:00")
	case "ns":
		return t.Format("2006-01-02 15:04:05.000000000-07:00")
	default:
		fatalf("invalid argument %q for --rfc-3339", level)
		return ""
	}
}

// applyFormat converts a GNU date format string to output.
func applyFormat(t time.Time, fmt string) string {
	var sb strings.Builder
	i := 0
	for i < len(fmt) {
		if fmt[i] != '%' {
			sb.WriteByte(fmt[i])
			i++
			continue
		}
		i++
		if i >= len(fmt) {
			sb.WriteByte('%')
			break
		}

		// flags: -, _, 0, ^, #
		flag := byte(0)
		if fmt[i] == '-' || fmt[i] == '_' || fmt[i] == '0' || fmt[i] == '^' || fmt[i] == '#' {
			flag = fmt[i]
			i++
		}
		// optional width
		width := 0
		for i < len(fmt) && fmt[i] >= '0' && fmt[i] <= '9' {
			width = width*10 + int(fmt[i]-'0')
			i++
		}
		if i >= len(fmt) {
			break
		}

		spec := fmt[i]
		i++

		// handle %:z and %::z
		if spec == ':' && i < len(fmt) {
			if fmt[i] == 'z' {
				sb.WriteString(colonTZ(t, 1))
				i++
				continue
			}
			if fmt[i] == ':' && i+1 < len(fmt) && fmt[i+1] == 'z' {
				sb.WriteString(colonTZ(t, 2))
				i += 2
				continue
			}
		}

		s := specToString(t, spec, flag, width)
		sb.WriteString(s)
	}
	return sb.String()
}

func specToString(t time.Time, spec byte, flag byte, width int) string {
	pad := func(s string, def int) string {
		w := def
		if width > 0 {
			w = width
		}
		if flag == '-' {
			return s
		}
		if flag == '_' {
			for len(s) < w {
				s = " " + s
			}
			return s
		}
		for len(s) < w {
			s = "0" + s
		}
		return s
	}
	upper := func(s string) string {
		if flag == '^' || flag == '#' {
			return strings.ToUpper(s)
		}
		return s
	}

	switch spec {
	case '%':
		return "%"
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case 'a':
		return upper(t.Format("Mon"))
	case 'A':
		return upper(t.Format("Monday"))
	case 'b', 'h':
		return upper(t.Format("Jan"))
	case 'B':
		return upper(t.Format("January"))
	case 'c':
		return t.Format("Mon Jan  2 15:04:05 2006")
	case 'C':
		return pad(strconv.Itoa(t.Year()/100), 2)
	case 'd':
		return pad(strconv.Itoa(t.Day()), 2)
	case 'D':
		return applyFormat(t, "%m/%d/%y")
	case 'e':
		s := strconv.Itoa(t.Day())
		if len(s) < 2 {
			s = " " + s
		}
		return s
	case 'F':
		return applyFormat(t, "%Y-%m-%d")
	case 'g':
		isoY, _ := t.ISOWeek()
		return pad(strconv.Itoa(isoY%100), 2)
	case 'G':
		isoY, _ := t.ISOWeek()
		return pad(strconv.Itoa(isoY), 4)
	case 'H':
		return pad(strconv.Itoa(t.Hour()), 2)
	case 'I':
		h := t.Hour() % 12
		if h == 0 {
			h = 12
		}
		return pad(strconv.Itoa(h), 2)
	case 'j':
		return pad(strconv.Itoa(t.YearDay()), 3)
	case 'k':
		s := strconv.Itoa(t.Hour())
		if len(s) < 2 {
			s = " " + s
		}
		return s
	case 'l':
		h := t.Hour() % 12
		if h == 0 {
			h = 12
		}
		s := strconv.Itoa(h)
		if len(s) < 2 {
			s = " " + s
		}
		return s
	case 'm':
		return pad(strconv.Itoa(int(t.Month())), 2)
	case 'M':
		return pad(strconv.Itoa(t.Minute()), 2)
	case 'N':
		return pad(strconv.Itoa(t.Nanosecond()), 9)
	case 'p':
		if t.Hour() < 12 {
			return "AM"
		}
		return "PM"
	case 'P':
		if t.Hour() < 12 {
			return "am"
		}
		return "pm"
	case 'q':
		return strconv.Itoa((int(t.Month())-1)/3 + 1)
	case 'r':
		return applyFormat(t, "%I:%M:%S %p")
	case 'R':
		return applyFormat(t, "%H:%M")
	case 's':
		return strconv.FormatInt(t.Unix(), 10)
	case 'S':
		return pad(strconv.Itoa(t.Second()), 2)
	case 'T':
		return applyFormat(t, "%H:%M:%S")
	case 'u':
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		return strconv.Itoa(wd)
	case 'U':
		// Week number, Sunday first day
		_, week := sundayWeek(t)
		return pad(strconv.Itoa(week), 2)
	case 'V':
		_, week := t.ISOWeek()
		return pad(strconv.Itoa(week), 2)
	case 'w':
		return strconv.Itoa(int(t.Weekday()))
	case 'W':
		// Week number, Monday first day
		_, week := mondayWeek(t)
		return pad(strconv.Itoa(week), 2)
	case 'x':
		return applyFormat(t, "%m/%d/%Y")
	case 'X':
		return applyFormat(t, "%H:%M:%S")
	case 'y':
		return pad(strconv.Itoa(t.Year()%100), 2)
	case 'Y':
		return pad(strconv.Itoa(t.Year()), 4)
	case 'z':
		_, off := t.Zone()
		sign := "+"
		if off < 0 {
			sign = "-"
			off = -off
		}
		h, m := off/3600, (off%3600)/60
		return sign + pad(strconv.Itoa(h), 2) + pad(strconv.Itoa(m), 2)
	case 'Z':
		name, _ := t.Zone()
		return name
	default:
		return "%" + string(spec)
	}
}

func colonTZ(t time.Time, colons int) string {
	_, off := t.Zone()
	sign := "+"
	if off < 0 {
		sign = "-"
		off = -off
	}
	h := off / 3600
	m := (off % 3600) / 60
	s := (off % 60)
	hs := fmt.Sprintf("%02d", h)
	ms := fmt.Sprintf("%02d", m)
	ss := fmt.Sprintf("%02d", s)
	if colons == 1 {
		return sign + hs + ":" + ms
	}
	return sign + hs + ":" + ms + ":" + ss
}

func sundayWeek(t time.Time) (int, int) {
	jan1 := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	dayOfYear := t.YearDay() - 1
	jan1Weekday := int(jan1.Weekday()) // 0=Sun
	week := (dayOfYear + jan1Weekday) / 7
	return t.Year(), week
}

func mondayWeek(t time.Time) (int, int) {
	jan1 := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	dayOfYear := t.YearDay() - 1
	jan1Weekday := int(jan1.Weekday())
	if jan1Weekday == 0 {
		jan1Weekday = 7
	}
	jan1Weekday-- // Mon=0
	week := (dayOfYear + jan1Weekday) / 7
	return t.Year(), week
}

// ─── date string parser ───────────────────────────────────────────────────────

var dayNames = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// absoluteLayouts are tried in order against the full input string.
var absoluteLayouts = []string{
	// ISO 8601 with timezone
	time.RFC3339Nano,
	time.RFC3339,
	// ISO 8601 without timezone
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	// Date + time, space separator
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	// Date only (ISO)
	"2006-01-02",
	// Slash date + time
	"01/02/2006 15:04:05",
	"01/02/2006 15:04",
	"01/02/06 15:04:05",
	"01/02/06 15:04",
	// Slash date only
	"01/02/2006",
	"01/02/06",
	// Full month name, day, year (comma optional)
	"January 2, 2006 15:04:05 MST",
	"January 2, 2006 15:04:05 -0700",
	"January 2, 2006 15:04:05",
	"January 2, 2006 15:04 MST",
	"January 2, 2006 15:04 -0700",
	"January 2, 2006 15:04",
	"January 2, 2006",
	"January 2 2006 15:04:05 MST",
	"January 2 2006 15:04:05 -0700",
	"January 2 2006 15:04:05",
	"January 2 2006 15:04 MST",
	"January 2 2006 15:04 -0700",
	"January 2 2006 15:04",
	"January 2 2006",
	// Abbreviated month name, day, year (comma optional)
	"Jan 2, 2006 15:04:05 MST",
	"Jan 2, 2006 15:04:05 -0700",
	"Jan 2, 2006 15:04:05",
	"Jan 2, 2006 15:04 MST",
	"Jan 2, 2006 15:04 -0700",
	"Jan 2, 2006 15:04",
	"Jan 2, 2006",
	"Jan 2 2006 15:04:05 MST",
	"Jan 2 2006 15:04:05 -0700",
	"Jan 2 2006 15:04:05",
	"Jan 2 2006 15:04 MST",
	"Jan 2 2006 15:04 -0700",
	"Jan 2 2006 15:04",
	"Jan 2 2006",
	// Day first, full month name
	"2 January 2006 15:04:05",
	"2 January 2006 15:04",
	"2 January 2006",
	// Day first, abbreviated month name
	"2 Jan 2006 15:04:05",
	"2 Jan 2006 15:04",
	"2 Jan 2006",
	// DD-Mon-YYYY
	"02-Jan-2006 15:04:05",
	"02-Jan-2006 15:04",
	"02-Jan-2006",
	// Month name + year only
	"January 2006",
	"Jan 2006",
	// RFC / ctime variants
	"02 Jan 2006 15:04:05 MST",
	"02 Jan 2006 15:04:05 -0700",
	time.RFC1123Z,
	time.RFC1123,
	time.RFC822Z,
	time.RFC822,
	"Mon, 02 Jan 2006 15:04:05 -0700",
	"Mon Jan 2 15:04:05 MST 2006",
	"Mon Jan 2 15:04:05 2006",
}

// timeOnlyLayouts are tried when the input looks like a bare time; today's date is attached.
var timeOnlyLayouts = []string{
	// 24-hour with timezone
	"15:04:05 MST",
	"15:04:05 -0700",
	"15:04 MST",
	"15:04 -0700",
	// 24-hour bare
	"15:04:05",
	"15:04",
	// 12-hour with timezone
	"3:04:05 PM MST",
	"3:04:05 PM -0700",
	"3:04 PM MST",
	"3:04 PM -0700",
	// 12-hour bare (Go is case-insensitive for AM/PM values)
	"3:04:05 PM",
	"3:04:05PM",
	"3:04 PM",
	"3:04PM",
	"3 PM",
	"3PM",
}

// parseDate parses a human-readable date string (GNU date -d compatible).
func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	now := time.Now()

	// @epoch  (e.g. @1750464000)
	if strings.HasPrefix(s, "@") {
		epoch, err := strconv.ParseInt(strings.TrimSpace(s[1:]), 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid epoch %q", s)
		}
		return time.Unix(epoch, 0), nil
	}

	// Normalize slash dates before layout matching (fixes "1/1/2026 06:00" etc.)
	norm := normalizeSlashDate(s)

	// Try absolute layouts
	for _, layout := range absoluteLayouts {
		if t, err := time.ParseInLocation(layout, norm, time.Local); err == nil {
			return t, nil
		}
	}

	// Try time-only layouts; attach today's date
	for _, layout := range timeOnlyLayouts {
		if t, err := time.ParseInLocation(layout, norm, time.Local); err == nil {
			y, m, d := now.Date()
			return time.Date(y, m, d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), now.Location()), nil
		}
	}

	// Try relative expressions, with an optional time-of-day suffix
	// e.g. "today 08:00", "next monday 9am", "2 days ago 06:00"
	if t, ok := parseRelativeExpr(s, now); ok {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date %q", s)
}

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// normalizeSlashDate pads single-digit month/day in M/D/YYYY (or M/D/YY [HH:MM...]).
// Only the month and day tokens are required to be purely numeric.
func normalizeSlashDate(s string) string {
	first := strings.Index(s, "/")
	if first < 0 {
		return s
	}
	second := strings.Index(s[first+1:], "/")
	if second < 0 {
		return s
	}
	second += first + 1

	month := s[:first]
	day := s[first+1 : second]
	rest := s[second+1:] // year, possibly followed by a time

	for _, c := range month {
		if c < '0' || c > '9' {
			return s
		}
	}
	for _, c := range day {
		if c < '0' || c > '9' {
			return s
		}
	}
	if len(month) == 1 {
		month = "0" + month
	}
	if len(day) == 1 {
		day = "0" + day
	}
	return month + "/" + day + "/" + rest
}

// parseRelativeExpr parses a relative date expression with an optional time suffix.
// Examples: "today", "yesterday 08:00", "next monday 9am", "2 days ago"
func parseRelativeExpr(s string, now time.Time) (time.Time, bool) {
	datePart, timePart := splitOffTimeSuffix(s)

	base, ok := parseRelativeBase(strings.TrimSpace(datePart), now)
	if !ok {
		return time.Time{}, false
	}

	if timePart == "" {
		return base, true
	}

	if t, ok2 := applyTimeSuffix(strings.TrimSpace(timePart), base); ok2 {
		return t, true
	}
	return base, true
}

// splitOffTimeSuffix splits "yesterday 08:00" into ("yesterday", "08:00").
// It checks the last 1 or 2 tokens for a time-of-day pattern.
func splitOffTimeSuffix(s string) (datePart, timePart string) {
	tokens := strings.Fields(s)
	n := len(tokens)
	if n < 2 {
		return s, ""
	}

	// Last two tokens: time + AM/PM  (e.g. "9:30 AM")
	if n >= 3 {
		last := strings.ToLower(tokens[n-1])
		if last == "am" || last == "pm" {
			if isTimeToken(strings.ToLower(tokens[n-2])) {
				return strings.Join(tokens[:n-2], " "), strings.Join(tokens[n-2:], " ")
			}
		}
	}

	// Last token alone is a time (e.g. "08:00", "9am")
	if isTimeToken(strings.ToLower(tokens[n-1])) {
		return strings.Join(tokens[:n-1], " "), tokens[n-1]
	}

	return s, ""
}

// isTimeToken returns true if s looks like a standalone time-of-day token.
func isTimeToken(s string) bool {
	s = strings.ToLower(s)
	for _, sfx := range []string{"am", "pm"} {
		if strings.HasSuffix(s, sfx) {
			return isClockFace(s[:len(s)-2])
		}
	}
	return isClockFace(s)
}

// isClockFace returns true for bare digit strings like "15", "15:04", "15:04:05".
func isClockFace(s string) bool {
	if s == "" {
		return false
	}
	parts := strings.Split(s, ":")
	if len(parts) == 0 || len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// applyTimeSuffix parses a time-only string and applies it to the date portion of base.
func applyTimeSuffix(s string, base time.Time) (time.Time, bool) {
	// Go's time parser requires uppercase AM/PM; normalize case.
	norm := normalizeAMPM(s)
	for _, layout := range timeOnlyLayouts {
		if t, err := time.ParseInLocation(layout, norm, base.Location()); err == nil {
			y, m, d := base.Date()
			return time.Date(y, m, d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), base.Location()), true
		}
	}
	return time.Time{}, false
}

// normalizeAMPM uppercases a trailing "am" or "pm" suffix so Go's parser accepts it.
func normalizeAMPM(s string) string {
	lower := strings.ToLower(s)
	for _, sfx := range []string{"am", "pm"} {
		if strings.HasSuffix(lower, sfx) {
			return s[:len(s)-2] + strings.ToUpper(sfx)
		}
	}
	return s
}

// parseRelativeBase parses a pure relative expression (no time suffix).
func parseRelativeBase(s string, now time.Time) (time.Time, bool) {
	lower := strings.ToLower(s)

	switch lower {
	case "now":
		return now, true
	case "today":
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location()), true
	case "yesterday":
		return midnight(now.AddDate(0, 0, -1)), true
	case "tomorrow":
		return midnight(now.AddDate(0, 0, 1)), true
	}

	// "next <weekday>"
	if strings.HasPrefix(lower, "next ") {
		if wd, ok := dayNames[strings.TrimSpace(lower[5:])]; ok {
			return nextWeekday(now, wd), true
		}
	}
	// "last <weekday|week|month|year>"
	if strings.HasPrefix(lower, "last ") {
		rest := strings.TrimSpace(lower[5:])
		if wd, ok := dayNames[rest]; ok {
			return lastWeekday(now, wd), true
		}
		switch rest {
		case "week":
			return now.AddDate(0, 0, -7), true
		case "month":
			return now.AddDate(0, -1, 0), true
		case "year":
			return now.AddDate(-1, 0, 0), true
		}
	}
	// "this <weekday>"
	if strings.HasPrefix(lower, "this ") {
		if wd, ok := dayNames[strings.TrimSpace(lower[5:])]; ok {
			return thisWeekday(now, wd), true
		}
	}

	// "+N unit" / "-N unit" / "N unit ago" / "N unit N unit ago"
	return parseOffset(lower, now)
}

// parseOffset handles "N unit [ago]", "+N unit", "-N unit", and combinations.
func parseOffset(s string, now time.Time) (time.Time, bool) {
	ago := false
	if strings.HasSuffix(s, " ago") {
		ago = true
		s = s[:len(s)-4]
	}

	sign := 1
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			sign = -1
		}
		s = s[1:]
	}
	if ago {
		sign = -sign
	}

	tokens := strings.Fields(s)
	if len(tokens) == 0 {
		return time.Time{}, false
	}

	t := now
	matched := false
	for i := 0; i+1 < len(tokens); i += 2 {
		n, err := strconv.ParseFloat(tokens[i], 64)
		if err != nil {
			return time.Time{}, false
		}
		unit := strings.TrimRight(strings.ToLower(tokens[i+1]), "s")
		n *= float64(sign)

		switch unit {
		case "second", "sec":
			t = t.Add(time.Duration(n * float64(time.Second)))
		case "minute", "min":
			t = t.Add(time.Duration(n * float64(time.Minute)))
		case "hour":
			t = t.Add(time.Duration(n * float64(time.Hour)))
		case "day":
			t = t.AddDate(0, 0, int(math.Round(n)))
		case "week":
			t = t.AddDate(0, 0, int(math.Round(n))*7)
		case "month":
			t = t.AddDate(0, int(math.Round(n)), 0)
		case "year":
			t = t.AddDate(int(math.Round(n)), 0, 0)
		default:
			return time.Time{}, false
		}
		matched = true
	}

	if matched {
		return t, true
	}
	return time.Time{}, false
}

func nextWeekday(from time.Time, wd time.Weekday) time.Time {
	d := int(wd) - int(from.Weekday())
	if d <= 0 {
		d += 7
	}
	return from.AddDate(0, 0, d)
}

func lastWeekday(from time.Time, wd time.Weekday) time.Time {
	d := int(from.Weekday()) - int(wd)
	if d <= 0 {
		d += 7
	}
	return from.AddDate(0, 0, -d)
}

func thisWeekday(from time.Time, wd time.Weekday) time.Time {
	d := int(wd) - int(from.Weekday())
	return from.AddDate(0, 0, d)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func fatalf(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s: "+f+"\n", append([]any{progName}, a...)...)
	os.Exit(1)
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: %s [OPTION]... [+FORMAT]
  or:  %s [-u|--utc|--universal] [MMDDhhmm[[CC]YY][.ss]]

Display or format the current date and time.

Mandatory arguments to long options are mandatory for short options too.
  -d, --date=STRING          display time described by STRING, not 'now'
  -f, --file=DATEFILE        like --date; once for each line of DATEFILE
  -I, --iso-8601 [<FMT>]    output date/time in ISO 8601 format.
                             FMT='date' for date only (the default),
                             'hours', 'minutes', 'seconds', or 'ns'
                             for date and time to the indicated precision.
                             Example: 2006-08-14T02:34:56-06:00
                             [possible values: date, hours, minutes, seconds, ns]
  -R, --rfc-email            output date and time in RFC 5322 format.
                               Example: Mon, 14 Aug 2006 02:34:56 -0600
      --rfc-3339=FMT         output date/time in RFC 3339 format.
                               FMT='date', 'seconds', or 'ns'
  -r, --reference=FILE       display the last modification time of FILE
  -u, --utc, --universal     print or set Coordinated Universal Time (UTC)
      --debug                annotate the parsed date
      --help                 display this help and exit

FORMAT controls the output; see below for the recognized sequences.

FORMAT sequences:
  %%%%   a literal %%
  %%a   locale's abbreviated weekday name (e.g., Sun)
  %%A   locale's full weekday name (e.g., Sunday)
  %%b   locale's abbreviated month name (e.g., Jan)
  %%B   locale's full month name (e.g., January)
  %%c   locale's date and time (e.g., Thu Mar  3 23:05:25 2005)
  %%C   century; like %%Y, except omit last two digits (e.g., 20)
  %%d   day of month (e.g., 01)
  %%D   date; same as %%m/%%d/%%y
  %%e   day of month, space padded; same as %%_d
  %%F   full date; like %%+4Y-%%m-%%d
  %%g   last two digits of ISO week year
  %%G   ISO week year
  %%H   hour (00..23)
  %%I   hour (01..12)
  %%j   day of year (001..366)
  %%k   hour, space padded ( 0..23)
  %%l   hour, space padded ( 1..12)
  %%m   month (01..12)
  %%M   minute (00..59)
  %%n   a newline
  %%N   nanoseconds (000000000..999999999)
  %%p   locale's equivalent of either AM or PM; blank if not known
  %%P   like %%p, but lower case
  %%q   quarter of year (1..4)
  %%r   locale's 12-hour clock time (e.g., 11:11:04 PM)
  %%R   24-hour hour and minute; same as %%H:%%M
  %%s   seconds since 1970-01-01 00:00:00 UTC
  %%S   second (00..60)
  %%t   a tab
  %%T   time; same as %%H:%%M:%%S
  %%u   day of week (1..7); 1 is Monday
  %%U   week number of year, with Sunday as first day of week (00..53)
  %%V   ISO week number, with Monday as first day of week (01..53)
  %%w   day of week (0..6); 0 is Sunday
  %%W   week number of year, with Monday as first day of week (00..53)
  %%x   locale's date representation (e.g., 12/31/99)
  %%X   locale's time representation (e.g., 23:13:48)
  %%y   last two digits of year (00..99)
  %%Y   year
  %%z   +hhmm numeric time zone (e.g., -0400)
  %%:z  +hh:mm numeric time zone (e.g., -04:00)
  %%::z +hh:mm:ss numeric time zone (e.g., -04:00:00)
  %%Z   alphabetic time zone abbreviation (e.g., EDT)

By default, date pads numeric fields with zeroes.
The following optional flags may follow '%%':
  -  (hyphen) do not pad the field
  _  (underscore) pad with spaces
  0  (zero) pad with zeros
  ^  use upper case if possible
  #  use opposite case if possible

DATE STRING examples:
  "now", "today", "yesterday", "tomorrow"
  "next monday", "last friday"
  "1 day ago", "2 hours ago", "3 weeks ago"
  "+1 hour", "-2 days"
  "2024-01-15", "Jan 15 2024", "15 Jan 2024"
  "@1234567890"  (Unix timestamp)
`, progName, progName)
}

