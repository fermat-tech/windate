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

func main() {
	var (
		dateStr  = flag.String("d", "", "display time described by STRING, not 'now'")
		dateFile = flag.String("f", "", "like -d, use each line of DATEFILE")
		refFile  = flag.String("r", "", "display the last modification time of FILE")
		utc      = flag.Bool("u", false, "print or set Coordinated Universal Time (UTC)")
		iso      = flag.String("I", "\x00", "output date/time in ISO 8601 format")
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

	// Resolve ISO flag: -I with no value means "date"
	isoLevel := ""
	if *iso != "\x00" {
		isoLevel = *iso
		if isoLevel == "" {
			isoLevel = "date"
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
		return t.Format("2006-01-02T15-07:00")
	case "minutes":
		return t.Format("2006-01-02T15:04-07:00")
	case "seconds":
		return t.Format("2006-01-02T15:04:05-07:00")
	case "ns":
		return t.Format("2006-01-02T15:04:05.000000000-07:00")
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

var monthNames = map[string]time.Month{
	"jan": time.January, "january": time.January,
	"feb": time.February, "february": time.February,
	"mar": time.March, "march": time.March,
	"apr": time.April, "april": time.April,
	"may": time.May,
	"jun": time.June, "june": time.June,
	"jul": time.July, "july": time.July,
	"aug": time.August, "august": time.August,
	"sep": time.September, "september": time.September,
	"oct": time.October, "october": time.October,
	"nov": time.November, "november": time.November,
	"dec": time.December, "december": time.December,
}

var dayNames = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// parseDate parses a human-readable date string similar to GNU date's -d option.
func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	now := time.Now()

	// Unix @timestamp
	if strings.HasPrefix(s, "@") {
		epoch, err := strconv.ParseInt(s[1:], 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid epoch %q", s)
		}
		return time.Unix(epoch, 0), nil
	}

	lower := strings.ToLower(s)

	// Quick keywords
	switch lower {
	case "now":
		return now, nil
	case "today":
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location()), nil
	case "yesterday":
		return midnight(now.AddDate(0, 0, -1)), nil
	case "tomorrow":
		return midnight(now.AddDate(0, 0, 1)), nil
	}

	// Normalize slash-separated dates: pad single-digit month/day with leading zero
	s = normalizeSlashDate(s)

	// Try standard Go layouts first (most specific to least)
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"01/02/2006",
		"01/02/06",
		"January 2, 2006",
		"Jan 2, 2006",
		"Jan 2 2006",
		"2 January 2006",
		"2 Jan 2006",
		"January 2006",
		"Jan 2006",
		"02 Jan 2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon Jan 2 15:04:05 MST 2006",
		"Mon Jan 2 15:04:05 2006",
		"15:04:05",
		"15:04",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			// If only time was parsed, attach today's date
			if layout == "15:04:05" || layout == "15:04" {
				y, m, d := now.Date()
				t = time.Date(y, m, d, t.Hour(), t.Minute(), t.Second(), 0, now.Location())
			}
			return t, nil
		}
	}

	// Relative expressions: "N unit [ago]", "next/last weekday", etc.
	if t, ok := parseRelative(s, now); ok {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date %q", s)
}

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// normalizeSlashDate pads single-digit month and day in M/D/YYYY or M/D/YY.
func normalizeSlashDate(s string) string {
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return s
	}
	// only touch parts that are purely numeric
	for _, p := range parts {
		for _, c := range p {
			if c < '0' || c > '9' {
				return s
			}
		}
	}
	if len(parts[0]) == 1 {
		parts[0] = "0" + parts[0]
	}
	if len(parts[1]) == 1 {
		parts[1] = "0" + parts[1]
	}
	return strings.Join(parts, "/")
}

// parseRelative handles strings like:
//   "1 day ago", "2 hours", "3 weeks ago", "next monday", "last friday",
//   "+1 hour", "-2 days", "1 year 2 months ago"
func parseRelative(s string, now time.Time) (time.Time, bool) {
	lower := strings.ToLower(strings.TrimSpace(s))

	// "next <weekday>"
	if strings.HasPrefix(lower, "next ") {
		wd, ok := dayNames[strings.TrimSpace(lower[5:])]
		if ok {
			return nextWeekday(now, wd), true
		}
	}
	// "last <weekday>"
	if strings.HasPrefix(lower, "last ") {
		wd, ok := dayNames[strings.TrimSpace(lower[5:])]
		if ok {
			return lastWeekday(now, wd), true
		}
		// "last month", "last year", "last week"
		switch strings.TrimSpace(lower[5:]) {
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
		wd, ok := dayNames[strings.TrimSpace(lower[5:])]
		if ok {
			return thisWeekday(now, wd), true
		}
	}

	// Tokenise into number-unit pairs, optionally followed by "ago"
	ago := false
	if strings.HasSuffix(lower, " ago") {
		ago = true
		lower = lower[:len(lower)-4]
	}

	// leading sign
	sign := 1
	if len(lower) > 0 && (lower[0] == '+' || lower[0] == '-') {
		if lower[0] == '-' {
			sign = -1
		}
		lower = lower[1:]
	}
	if ago {
		sign = -sign
	}

	// parse one or more "N unit" pairs
	tokens := strings.Fields(lower)
	if len(tokens) == 0 {
		return time.Time{}, false
	}

	t := now
	i := 0
	matched := false
	for i < len(tokens) {
		numStr := tokens[i]
		n, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			break
		}
		i++
		if i >= len(tokens) {
			break
		}
		unit := strings.TrimRight(tokens[i], "s") // plurals
		i++
		matched = true

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
			// check if it's a weekday used as a unit (e.g., "2 mondays ago")
			if _, ok := dayNames[unit]; ok {
				return time.Time{}, false
			}
			matched = false
		}
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
  -I[FMT], --iso-8601[=FMT] output date/time in ISO 8601 format.
                               FMT='date' for date only (the default),
                               'hours', 'minutes', 'seconds', or 'ns'
                               for date and time to the indicated precision.
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

