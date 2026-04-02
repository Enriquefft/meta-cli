package output

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/enriquefft/meta-cli/internal/meta"
)

// Print formats data to the given writer in the specified format, optionally filtering fields.
// data: any JSON-serializable value (struct, map, slice).
// format: "json" | "table" | "csv".
// fields: comma-separated field names to include (empty string means all fields).
func Print(w io.Writer, data any, format string, fields string) error {
	switch format {
	case "json":
		return printJSON(w, data, fields)
	case "table":
		return printTable(w, data, fields)
	case "csv":
		return printCSV(w, data, fields)
	default:
		return fmt.Errorf("unsupported format: %q (valid: json, table, csv)", format)
	}
}

// PrintError writes a structured JSON error to the given writer.
// For *meta.GraphError: includes message, code, subcode, and trace_id.
// For other errors: includes message only.
func PrintError(w io.Writer, err error) error {
	var ge *meta.GraphError
	if errors.As(err, &ge) {
		return writeJSON(w, map[string]any{
			"error": map[string]any{
				"message":  ge.Message,
				"code":     ge.Code,
				"subcode":  ge.Subcode,
				"trace_id": ge.TraceID,
			},
		})
	}

	return writeJSON(w, map[string]any{
		"error": map[string]any{
			"message": err.Error(),
		},
	})
}

// --- JSON ---

func printJSON(w io.Writer, data any, fields string) error {
	if fields == "" {
		return writeJSON(w, data)
	}

	filtered, err := filterFields(data, parseFields(fields))
	if err != nil {
		return err
	}
	return writeJSON(w, filtered)
}

func writeJSON(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// --- Table ---

func printTable(w io.Writer, data any, fields string) error {
	rows, err := toRows(data, fields)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	headers := rows[0]
	dataRows := rows[1:]

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// Write header (color support to be added when charmbracelet/lipgloss is integrated).
	if _, err = fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return err
	}

	// Write data rows
	for _, row := range dataRows {
		if _, writeErr := fmt.Fprintln(tw, strings.Join(row, "\t")); writeErr != nil {
			return writeErr
		}
	}

	return tw.Flush()
}

// --- CSV ---

func printCSV(w io.Writer, data any, fields string) error {
	rows, err := toRows(data, fields)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	cw := csv.NewWriter(w)
	for _, row := range rows {
		if writeErr := cw.Write(row); writeErr != nil {
			return writeErr
		}
	}
	cw.Flush()
	return cw.Error()
}

// --- Field filtering and data normalization ---

// parseFields splits a comma-separated fields string into a slice of trimmed field names.
func parseFields(fields string) []string {
	parts := strings.Split(fields, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// filterFields applies field filtering to data. Works with maps, slices, and structs.
func filterFields(data any, fields []string) (any, error) {
	if len(fields) == 0 {
		return data, nil
	}

	normalized, err := normalize(data)
	if err != nil {
		return nil, err
	}

	switch v := normalized.(type) {
	case map[string]any:
		return filterMap(v, fields), nil
	case []any:
		result := make([]any, 0, len(v))
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("slice element is not an object")
			}
			result = append(result, filterMap(m, fields))
		}
		return result, nil
	default:
		return normalized, nil
	}
}

// filterMap returns a new map containing only the specified keys.
func filterMap(m map[string]any, fields []string) map[string]any {
	filtered := make(map[string]any, len(fields))
	for _, f := range fields {
		if v, ok := m[f]; ok {
			filtered[f] = v
		}
	}
	return filtered
}

// normalize converts any JSON-serializable value to its generic representation
// (map[string]any or []any) via JSON round-trip.
func normalize(data any) (any, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshaling data: %w", err)
	}
	var result any
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling data: %w", err)
	}
	return result, nil
}

// toRows converts data into a string matrix: first row is headers, subsequent rows are data.
// Returns nil for nil/empty data.
func toRows(data any, fields string) ([][]string, error) {
	if data == nil {
		return nil, nil
	}

	// Check if it's an empty slice via reflection
	rv := reflect.ValueOf(data)
	if rv.Kind() == reflect.Slice && rv.Len() == 0 {
		return nil, nil
	}

	normalized, err := normalize(data)
	if err != nil {
		return nil, err
	}

	var maps []map[string]any

	switch v := normalized.(type) {
	case map[string]any:
		maps = []map[string]any{v}
	case []any:
		if len(v) == 0 {
			return nil, nil
		}
		maps = make([]map[string]any, 0, len(v))
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("slice element is not an object")
			}
			maps = append(maps, m)
		}
	default:
		return nil, fmt.Errorf("unsupported data type for tabular output: %T", data)
	}

	if len(maps) == 0 {
		return nil, nil
	}

	// Determine headers: use field filter if specified, otherwise collect all keys.
	var headers []string
	if fields != "" {
		headers = parseFields(fields)
	} else {
		headers = collectKeys(maps)
	}

	if len(headers) == 0 {
		return nil, nil
	}

	rows := make([][]string, 0, len(maps)+1)
	rows = append(rows, headers)

	for _, m := range maps {
		row := make([]string, len(headers))
		for i, h := range headers {
			row[i] = formatValue(m[h])
		}
		rows = append(rows, row)
	}

	return rows, nil
}

// collectKeys returns sorted, deduplicated keys from a slice of maps.
func collectKeys(maps []map[string]any) []string {
	seen := make(map[string]struct{})
	var keys []string
	for _, m := range maps {
		for k := range m {
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

// formatValue converts a value to its string representation for table/CSV output.
func formatValue(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// IsNoColor returns true when the NO_COLOR environment variable is set.
func IsNoColor() bool {
	_, set := os.LookupEnv("NO_COLOR")
	return set
}
