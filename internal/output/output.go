package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatYAML  = "yaml"
)

// NormalizeFormat normalizes user-provided output formats.
func NormalizeFormat(in string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(in))
	switch value {
	case "", "table", "text":
		return FormatTable, nil
	case FormatJSON:
		return FormatJSON, nil
	case "yml", FormatYAML:
		return FormatYAML, nil
	default:
		return "", fmt.Errorf("unsupported output format %q (supported: table, json, yaml)", in)
	}
}

// Encode writes JSON or YAML payloads.
func Encode(w io.Writer, format string, payload any) error {
	normalized, err := NormalizeFormat(format)
	if err != nil {
		return err
	}

	switch normalized {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	case FormatYAML:
		enc := yaml.NewEncoder(w)
		defer func() {
			_ = enc.Close()
		}()
		return enc.Encode(payload)
	default:
		return fmt.Errorf("table output requires a table renderer")
	}
}

// WriteTable renders a simple tab-separated table.
func WriteTable(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if len(headers) > 0 {
		_, _ = fmt.Fprintln(tw, strings.Join(headers, "\t"))
	}
	for _, row := range rows {
		_, _ = fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	_ = tw.Flush()
}

// ActionableError creates user-first error text with cause, hint, and next command.
func ActionableError(title, probableCause, fixHint, nextCommand string) error {
	lines := []string{title}
	if strings.TrimSpace(probableCause) != "" {
		lines = append(lines, "Cause: "+strings.TrimSpace(probableCause))
	}
	if strings.TrimSpace(fixHint) != "" {
		lines = append(lines, "Hint: "+strings.TrimSpace(fixHint))
	}
	if strings.TrimSpace(nextCommand) != "" {
		lines = append(lines, "Next: "+strings.TrimSpace(nextCommand))
	}
	return errors.New(strings.Join(lines, "\n"))
}
