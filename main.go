// json-log-parser: read JSON logs (stdin or file), print in a chosen debug-friendly format.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

var timeKeys = []string{"time", "ts", "timestamp", "@timestamp", "Time"}
var levelKeys = []string{"level", "lvl", "severity", "Level"}
var msgKeys = []string{"msg", "message", "text", "Message"}

const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorGray    = "\033[90m"
	colorGreen   = "\033[32m"
	colorBold    = "\033[1m"
	colorCyan    = "\033[36m"
	colorMagenta = "\033[35m"
	colorWhite   = "\033[97m"
)

type entry struct {
	raw     string
	fields  map[string]any
	time    string
	level   string
	message string
	extra   []string // sorted "key=val" for remaining fields
	valid   bool
}

func firstString(m map[string]any, keys []string) (string, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			delete(m, k)
			return fmt.Sprint(v), true
		}
	}
	return "", false
}

func parseLine(line string) entry {
	e := entry{raw: line}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return e
	}
	e.valid = true
	e.fields = m
	e.time, _ = firstString(m, timeKeys)
	e.level, _ = firstString(m, levelKeys)
	e.message, _ = firstString(m, msgKeys)

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b, err := json.Marshal(m[k])
		if err != nil {
			continue
		}
		e.extra = append(e.extra, fmt.Sprintf("%s=%s", k, b))
	}
	return e
}

func levelColor(level string, noColor bool) string {
	if noColor {
		return ""
	}
	switch strings.ToUpper(level) {
	case "ERROR", "ERR", "FATAL", "PANIC":
		return colorRed
	case "WARN", "WARNING":
		return colorYellow
	case "INFO":
		return colorBlue
	case "DEBUG", "TRACE":
		return colorGray
	default:
		return ""
	}
}

func main() {
	table := flag.Bool("table", false, "print as aligned table")
	oneline := flag.Bool("oneline", false, "print as one line per entry (time level msg key=val ...)")
	pprint := flag.Bool("pprint", false, "pretty-print each entry as indented JSON")
	logfmt := flag.Bool("logfmt", false, "print as logfmt (key=val key=val ...)")
	levelFilter := flag.String("level", "", "only show entries matching this level (case-insensitive)")
	grep := flag.String("grep", "", "only show entries whose raw line contains this substring")
	noColor := flag.Bool("no-color", false, "disable ANSI colors")
	flag.Parse()

	modes := 0
	for _, b := range []bool{*table, *oneline, *pprint, *logfmt} {
		if b {
			modes++
		}
	}
	if modes > 1 {
		fmt.Fprintln(os.Stderr, "error: choose only one of -table, -oneline, -pprint, -logfmt")
		os.Exit(1)
	}
	if modes == 0 {
		*oneline = true
	}

	var r io.Reader = os.Stdin
	if args := flag.Args(); len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		r = f
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	if *table {
		printTableHeader(w)
	}

	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		e := parseLine(line)
		if *grep != "" && !strings.Contains(e.raw, *grep) {
			continue
		}
		if *levelFilter != "" && !strings.EqualFold(e.level, *levelFilter) {
			continue
		}

		switch {
		case *table:
			printTableRow(w, e, *noColor)
		case *pprint:
			printPprintEntry(w, e, *noColor, first)
		case *logfmt:
			printLogfmtEntry(w, e)
		default:
			printOnelineEntry(w, e, *noColor)
		}
		first = false
		w.Flush() // stream each entry immediately (e.g. when piped from a running process)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
		os.Exit(1)
	}
}

func printOnelineEntry(w io.Writer, e entry, noColor bool) {
	if !e.valid {
		fmt.Fprintln(w, e.raw)
		return
	}
	c := levelColor(e.level, noColor)
	reset := ""
	if c != "" {
		reset = colorReset
	}
	parts := []string{}
	if e.time != "" {
		t := e.time
		if !noColor {
			t = colorGray + t + colorReset
		}
		parts = append(parts, t)
	}
	if e.level != "" {
		parts = append(parts, fmt.Sprintf("%s%-5s%s", c, strings.ToUpper(e.level), reset))
	}
	if e.message != "" {
		m := e.message
		if !noColor {
			m = colorWhite + m + colorReset
		}
		parts = append(parts, m)
	}
	if len(e.extra) > 0 {
		parts = append(parts, colorizeFields(e.extra, noColor))
	}
	fmt.Fprintln(w, strings.Join(parts, " "))
}

func printLogfmtEntry(w io.Writer, e entry) {
	if !e.valid {
		fmt.Fprintln(w, e.raw)
		return
	}
	parts := []string{}
	if e.time != "" {
		parts = append(parts, "time="+e.time)
	}
	if e.level != "" {
		parts = append(parts, "level="+e.level)
	}
	if e.message != "" {
		parts = append(parts, fmt.Sprintf("msg=%q", e.message))
	}
	parts = append(parts, e.extra...)
	fmt.Fprintln(w, strings.Join(parts, " "))
}

func printPprintEntry(w io.Writer, e entry, noColor bool, first bool) {
	if !first {
		fmt.Fprintln(w, strings.Repeat("-", 40))
	}
	if !e.valid {
		fmt.Fprintln(w, e.raw)
		return
	}
	c := levelColor(e.level, noColor)
	reset := ""
	if c != "" {
		reset = colorReset
	}
	if e.level != "" {
		fmt.Fprintf(w, "%s[%s]%s\n", c, strings.ToUpper(e.level), reset)
	}
	if noColor {
		b, _ := json.MarshalIndent(e.fields, "", "  ")
		fmt.Fprintln(w, string(b))
	} else {
		var sb strings.Builder
		writeColoredJSON(&sb, e.fields, 0)
		fmt.Fprintln(w, sb.String())
	}
}

// writeColoredJSON renders v as indented JSON with ANSI colors:
// keys blue, strings green, numbers yellow, bool/null magenta.
func writeColoredJSON(sb *strings.Builder, v any, indent int) {
	pad := strings.Repeat("  ", indent)
	childPad := strings.Repeat("  ", indent+1)
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			sb.WriteString("{}")
			return
		}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sb.WriteString("{\n")
		for i, k := range keys {
			sb.WriteString(childPad)
			fmt.Fprintf(sb, "%s%q%s: ", colorBlue+colorBold, k, colorReset)
			writeColoredJSON(sb, val[k], indent+1)
			if i < len(keys)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		sb.WriteString(pad)
		sb.WriteString("}")
	case []any:
		if len(val) == 0 {
			sb.WriteString("[]")
			return
		}
		sb.WriteString("[\n")
		for i, item := range val {
			sb.WriteString(childPad)
			writeColoredJSON(sb, item, indent+1)
			if i < len(val)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		sb.WriteString(pad)
		sb.WriteString("]")
	case string:
		sb.WriteString(colorGreen)
		b, _ := json.Marshal(val)
		sb.Write(b)
		sb.WriteString(colorReset)
	case float64:
		sb.WriteString(colorYellow)
		b, _ := json.Marshal(val)
		sb.Write(b)
		sb.WriteString(colorReset)
	case bool, nil:
		sb.WriteString(colorRed)
		b, _ := json.Marshal(val)
		sb.Write(b)
		sb.WriteString(colorReset)
	default:
		b, _ := json.Marshal(val)
		sb.Write(b)
	}
}

// Fixed column widths so the table can be printed one row at a time as
// entries arrive, instead of buffering everything to compute alignment.
const (
	tableTimeW  = 24
	tableLevelW = 5
	tableMsgW   = 40
)

func truncate(s string, w int) string {
	if len(s) <= w {
		return s
	}
	if w <= 1 {
		return s[:w]
	}
	return s[:w-1] + "…"
}

const tableSep = " │ "

func printTableHeader(w io.Writer) {
	hc := colorBold + colorCyan
	fmt.Fprintf(w, "%s%-*s%s%s%s%-*s%s%s%s%-*s%s%s%sFIELDS%s\n",
		hc, tableTimeW, "TIME", colorReset, tableSep,
		hc, tableLevelW, "LEVEL", colorReset, tableSep,
		hc, tableMsgW, "MESSAGE", colorReset, tableSep,
		hc, colorReset)
	fmt.Fprintln(w, colorGray+strings.Repeat("─", tableTimeW+tableLevelW+tableMsgW+30)+colorReset)
}

func printTableRow(w io.Writer, e entry, noColor bool) {
	if !e.valid {
		fmt.Fprintln(w, e.raw)
		return
	}
	c := levelColor(e.level, noColor)
	reset := ""
	if c != "" {
		reset = colorReset
	}
	sep := tableSep
	if !noColor {
		sep = colorGray + tableSep + colorReset
	}
	timeCol := truncate(e.time, tableTimeW)
	msgCol := truncate(e.message, tableMsgW)
	if !noColor {
		timeCol = colorGray + fmt.Sprintf("%-*s", tableTimeW, timeCol) + colorReset
	} else {
		timeCol = fmt.Sprintf("%-*s", tableTimeW, timeCol)
	}
	if !noColor {
		msgCol = colorWhite + fmt.Sprintf("%-*s", tableMsgW, msgCol) + colorReset
	} else {
		msgCol = fmt.Sprintf("%-*s", tableMsgW, msgCol)
	}
	levelCol := c + fmt.Sprintf("%-*s", tableLevelW, strings.ToUpper(e.level)) + reset
	var sb strings.Builder
	sb.WriteString(timeCol)
	sb.WriteString(sep)
	sb.WriteString(levelCol)
	sb.WriteString(sep)
	sb.WriteString(msgCol)
	sb.WriteString(sep)
	sb.WriteString(colorizeFields(e.extra, noColor))
	fmt.Fprintln(w, sb.String())
}

// colorizeFields renders "key=val" pairs with key in cyan and val in green.
func colorizeFields(extra []string, noColor bool) string {
	if noColor {
		return strings.Join(extra, " ")
	}
	parts := make([]string, len(extra))
	for i, kv := range extra {
		if key, val, ok := strings.Cut(kv, "="); ok {
			parts[i] = colorCyan + key + colorReset + colorGray + "=" + colorReset + colorGreen + val + colorReset
		} else {
			parts[i] = kv
		}
	}
	return strings.Join(parts, " ")
}
