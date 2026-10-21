package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/engine"
)

// envDot renders the environment as its name with a coloured dot on a TTY.
func envDot(w io.Writer, env connection.Environment) string {
	dot := "●"
	if isTTY(w) {
		code := map[connection.Environment]string{
			connection.Production:  "31",
			connection.Staging:     "33",
			connection.Development: "34",
			connection.Local:       "90",
		}[env]
		if code != "" {
			dot = "\x1b[" + code + "m●\x1b[0m"
		}
	}
	return dot + " " + env.Label()
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// table prints rows with aligned columns.
func table(w io.Writer, header []string, rows [][]string, numeric []bool) {
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if i < len(widths) && utf8.RuneCountInString(c) > widths[i] {
				widths[i] = utf8.RuneCountInString(c)
			}
		}
	}
	line := func(cells []string) {
		parts := make([]string, len(cells))
		for i, c := range cells {
			pad := widths[i] - utf8.RuneCountInString(c)
			if i < len(numeric) && numeric[i] {
				parts[i] = strings.Repeat(" ", pad) + c
			} else {
				parts[i] = c + strings.Repeat(" ", pad)
			}
		}
		fmt.Fprintln(w, "  "+strings.TrimRight(strings.Join(parts, "  "), " "))
	}
	line(header)
	sep := make([]string, len(header))
	for i := range sep {
		sep[i] = strings.Repeat("─", widths[i])
	}
	line(sep)
	for _, r := range rows {
		line(r)
	}
}

// result prints a query result as a table.
func result(w io.Writer, res *engine.Result) {
	header := make([]string, len(res.Columns))
	numeric := make([]bool, len(res.Columns))
	for i, c := range res.Columns {
		header[i] = c.Name
		numeric[i] = c.Kind == "int" || c.Kind == "float" || c.Kind == "decimal"
	}
	rows := make([][]string, 0, len(res.Rows))
	for _, r := range res.Rows {
		cells := make([]string, len(r))
		for i, v := range r {
			if v.IsNull() {
				cells[i] = "NULL"
			} else {
				cells[i] = v.Text()
			}
		}
		rows = append(rows, cells)
	}
	if len(res.Columns) > 0 {
		table(w, header, rows, numeric)
	}
	note := fmt.Sprintf("%d rows · %d ms", len(res.Rows), res.Duration.Milliseconds())
	if len(res.Columns) == 0 {
		note = fmt.Sprintf("%s %d · %d ms", res.Command, res.RowsAffected, res.Duration.Milliseconds())
	}
	if res.Truncated {
		note += " · truncated"
	}
	fmt.Fprintln(w, "  "+note)
}

// plan prints a plan the way the review dialog shows it.
func plan(w io.Writer, p *changeset.Plan) {
	for i, s := range p.Statements {
		for j, line := range strings.Split(s.Display(), "\n") {
			if j == 0 {
				fmt.Fprintf(w, "  %3d  %s\n", i+1, line)
			} else {
				fmt.Fprintf(w, "       %s\n", line)
			}
		}
	}
	if len(p.Risks) > 0 {
		fmt.Fprintln(w)
		for _, r := range p.Risks {
			sym := symbolOK
			switch r.Level {
			case changeset.Warning:
				sym = symbolWarn
			case changeset.Danger:
				sym = symbolErr
			}
			fmt.Fprintf(w, "  %s %s\n", sym, r.Message)
		}
	}
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func compact(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1fB", float64(n)/1e9), ".0B") + strings.Repeat("B", 0)
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1fM", float64(n)/1e6), ".0M")
	case n >= 1_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1fK", float64(n)/1e3), ".0K")
	}
	return fmt.Sprintf("%d", n)
}
