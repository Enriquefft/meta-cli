package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

// --- JSON format tests ---

func TestPrint_JSON_Struct(t *testing.T) {
	type Campaign struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	var buf bytes.Buffer
	data := Campaign{ID: "123", Name: "Test Campaign", Status: "ACTIVE"}

	if err := Print(&buf, data, "json", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if got["id"] != "123" {
		t.Errorf("expected id=123, got %v", got["id"])
	}
	if got["name"] != "Test Campaign" {
		t.Errorf("expected name=Test Campaign, got %v", got["name"])
	}
	if got["status"] != "ACTIVE" {
		t.Errorf("expected status=ACTIVE, got %v", got["status"])
	}

	// Verify pretty-printed (indented)
	if !strings.Contains(buf.String(), "\n") {
		t.Error("expected pretty-printed JSON with newlines")
	}
}

func TestPrint_JSON_Map(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{
		"key1": "value1",
		"key2": 42,
	}

	if err := Print(&buf, data, "json", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if got["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", got["key1"])
	}
	if got["key2"] != float64(42) {
		t.Errorf("expected key2=42, got %v", got["key2"])
	}
}

func TestPrint_JSON_Slice(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"id": "1", "name": "First"},
		{"id": "2", "name": "Second"},
	}

	if err := Print(&buf, data, "json", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON array: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(got))
	}
	if got[0]["id"] != "1" {
		t.Errorf("expected first id=1, got %v", got[0]["id"])
	}
}

func TestPrint_JSON_WithFieldsFilter(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{
		"id":     "123",
		"name":   "Test",
		"status": "ACTIVE",
		"budget": 5000,
	}

	if err := Print(&buf, data, "json", "id,name"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if _, ok := got["id"]; !ok {
		t.Error("expected 'id' field to be present")
	}
	if _, ok := got["name"]; !ok {
		t.Error("expected 'name' field to be present")
	}
	if _, ok := got["status"]; ok {
		t.Error("expected 'status' field to be absent")
	}
	if _, ok := got["budget"]; ok {
		t.Error("expected 'budget' field to be absent")
	}
}

func TestPrint_JSON_WithFieldsFilter_Struct(t *testing.T) {
	type Campaign struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	var buf bytes.Buffer
	data := Campaign{ID: "123", Name: "Test", Status: "ACTIVE"}

	if err := Print(&buf, data, "json", "id,name"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("expected 2 fields, got %d: %v", len(got), got)
	}
	if _, ok := got["status"]; ok {
		t.Error("expected 'status' field to be filtered out")
	}
}

func TestPrint_JSON_WithFieldsFilter_Slice(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"id": "1", "name": "First", "status": "ACTIVE"},
		{"id": "2", "name": "Second", "status": "PAUSED"},
	}

	if err := Print(&buf, data, "json", "id,name"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON array: %v", err)
	}

	for i, item := range got {
		if _, ok := item["status"]; ok {
			t.Errorf("item %d: expected 'status' to be filtered out", i)
		}
		if _, ok := item["id"]; !ok {
			t.Errorf("item %d: expected 'id' to be present", i)
		}
	}
}

// --- Table format tests ---

func TestPrint_Table_SingleRow(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{
		"id":   "123",
		"name": "Test",
	}

	if err := Print(&buf, data, "table", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "123") {
		t.Error("expected output to contain '123'")
	}
	if !strings.Contains(output, "Test") {
		t.Error("expected output to contain 'Test'")
	}
	// Headers should be present
	if !strings.Contains(strings.ToUpper(output), "ID") {
		t.Error("expected output to contain header 'ID' (case-insensitive)")
	}
}

func TestPrint_Table_MultipleRows(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"id": "1", "name": "First"},
		{"id": "2", "name": "Second"},
	}

	if err := Print(&buf, data, "table", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "1") || !strings.Contains(output, "First") {
		t.Error("expected first row data")
	}
	if !strings.Contains(output, "2") || !strings.Contains(output, "Second") {
		t.Error("expected second row data")
	}
}

func TestPrint_Table_EmptySlice(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{}

	if err := Print(&buf, data, "table", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty data should produce no output (or a minimal message)
	output := strings.TrimSpace(buf.String())
	if output != "" {
		t.Errorf("expected empty output for empty slice, got: %q", output)
	}
}

func TestPrint_Table_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var buf bytes.Buffer
	data := []map[string]any{
		{"id": "1", "status": "ACTIVE"},
	}

	if err := Print(&buf, data, "table", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Ensure no ANSI escape codes
	if strings.Contains(output, "\033[") {
		t.Error("expected no ANSI escape codes when NO_COLOR is set")
	}
}

func TestPrint_Table_Struct(t *testing.T) {
	type Account struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	var buf bytes.Buffer
	data := []Account{
		{ID: "act_1", Name: "Account 1"},
		{ID: "act_2", Name: "Account 2"},
	}

	if err := Print(&buf, data, "table", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "act_1") {
		t.Error("expected output to contain 'act_1'")
	}
	if !strings.Contains(output, "Account 2") {
		t.Error("expected output to contain 'Account 2'")
	}
}

// --- CSV format tests ---

func TestPrint_CSV_BasicOutput(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"id": "1", "name": "First"},
		{"id": "2", "name": "Second"},
	}

	if err := Print(&buf, data, "csv", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d: %v", len(lines), lines)
	}

	// Header line should contain id and name
	header := lines[0]
	if !strings.Contains(header, "id") || !strings.Contains(header, "name") {
		t.Errorf("expected header to contain 'id' and 'name', got: %s", header)
	}
}

func TestPrint_CSV_EmptySlice(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{}

	if err := Print(&buf, data, "csv", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "" {
		t.Errorf("expected empty output for empty slice, got: %q", output)
	}
}

func TestPrint_CSV_WithFieldsFilter(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"id": "1", "name": "First", "status": "ACTIVE"},
		{"id": "2", "name": "Second", "status": "PAUSED"},
	}

	if err := Print(&buf, data, "csv", "id,name"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	header := lines[0]
	if strings.Contains(header, "status") {
		t.Error("expected 'status' to be filtered out of CSV header")
	}
	if !strings.Contains(header, "id") || !strings.Contains(header, "name") {
		t.Error("expected 'id' and 'name' in CSV header")
	}
}

func TestPrint_CSV_SingleObject(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{
		"id":   "1",
		"name": "Test",
	}

	if err := Print(&buf, data, "csv", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (header + 1 row), got %d", len(lines))
	}
}

func TestPrint_CSV_NoColor(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"id": "1", "status": "ACTIVE"},
	}

	if err := Print(&buf, data, "csv", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "\033[") {
		t.Error("CSV should never contain ANSI escape codes")
	}
}

// --- PrintError tests ---

func TestPrintError_GraphError(t *testing.T) {
	var buf bytes.Buffer
	graphErr := &meta.GraphError{
		Message: "Invalid access token",
		Code:    190,
		Subcode: 467,
		TraceID: "AbC123xYz",
	}

	if err := PrintError(&buf, graphErr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'error' key with object value, got: %v", got)
	}

	if errObj["message"] != "Invalid access token" {
		t.Errorf("expected message='Invalid access token', got %v", errObj["message"])
	}
	if errObj["code"] != float64(190) {
		t.Errorf("expected code=190, got %v", errObj["code"])
	}
	if errObj["subcode"] != float64(467) {
		t.Errorf("expected subcode=467, got %v", errObj["subcode"])
	}
	if errObj["trace_id"] != "AbC123xYz" {
		t.Errorf("expected trace_id=AbC123xYz, got %v", errObj["trace_id"])
	}
}

func TestPrintError_PlainError(t *testing.T) {
	var buf bytes.Buffer
	plainErr := strings.NewReader("") // just need any error
	_ = plainErr

	if err := PrintError(&buf, &simpleError{msg: "something went wrong"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'error' key with object value, got: %v", got)
	}

	if errObj["message"] != "something went wrong" {
		t.Errorf("expected message='something went wrong', got %v", errObj["message"])
	}

	// Plain error should not have code/subcode/trace_id
	if _, ok := errObj["code"]; ok {
		t.Error("expected no 'code' field for plain error")
	}
	if _, ok := errObj["subcode"]; ok {
		t.Error("expected no 'subcode' field for plain error")
	}
	if _, ok := errObj["trace_id"]; ok {
		t.Error("expected no 'trace_id' field for plain error")
	}
}

func TestPrintError_GraphError_WithUserFields(t *testing.T) {
	var buf bytes.Buffer
	graphErr := &meta.GraphError{
		Message:     "Invalid parameter",
		Code:        100,
		Subcode:     1443226,
		TraceID:     "A_4_hqeC6Ihn23dP1sxHqfg",
		UserTitle:   "Your ad needs a video thumbnail",
		UserMessage: "Please specify one of image_hash or image_url in the video_data field of object_story_spec.",
	}

	if err := PrintError(&buf, graphErr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'error' key with object value, got: %v", got)
	}

	if errObj["user_title"] != "Your ad needs a video thumbnail" {
		t.Errorf("expected user_title to be surfaced, got %v", errObj["user_title"])
	}
	const wantUserMsg = "Please specify one of image_hash or image_url in the video_data field of object_story_spec."
	if errObj["user_message"] != wantUserMsg {
		t.Errorf("expected user_message=%q, got %v", wantUserMsg, errObj["user_message"])
	}
	// Existing fields must still be present (backward compatibility).
	if errObj["code"] != float64(100) {
		t.Errorf("expected code=100, got %v", errObj["code"])
	}
	if errObj["subcode"] != float64(1443226) {
		t.Errorf("expected subcode=1443226, got %v", errObj["subcode"])
	}
	if errObj["trace_id"] != "A_4_hqeC6Ihn23dP1sxHqfg" {
		t.Errorf("expected trace_id preserved, got %v", errObj["trace_id"])
	}
}

func TestPrintError_GraphError_OmitsEmptyUserFields(t *testing.T) {
	var buf bytes.Buffer
	graphErr := &meta.GraphError{
		Message: "Invalid token",
		Code:    190,
		Subcode: 467,
		TraceID: "trace",
	}

	if err := PrintError(&buf, graphErr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	errObj := got["error"].(map[string]any)
	if _, ok := errObj["user_title"]; ok {
		t.Error("expected user_title to be absent when empty")
	}
	if _, ok := errObj["user_message"]; ok {
		t.Error("expected user_message to be absent when empty")
	}
}

func TestPrintError_WrappedGraphError(t *testing.T) {
	var buf bytes.Buffer
	graphErr := &meta.GraphError{
		Message: "Rate limit hit",
		Code:    17,
		Subcode: 0,
		TraceID: "trace123",
	}
	// Wrap the error
	wrapped := &wrappedError{msg: "api call failed", inner: graphErr}

	if err := PrintError(&buf, wrapped); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	errObj := got["error"].(map[string]any)
	if errObj["code"] != float64(17) {
		t.Errorf("expected code=17 from unwrapped GraphError, got %v", errObj["code"])
	}
	if errObj["trace_id"] != "trace123" {
		t.Errorf("expected trace_id=trace123, got %v", errObj["trace_id"])
	}
}

// --- Invalid format test ---

func TestPrint_InvalidFormat(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{"id": "1"}

	err := Print(&buf, data, "xml", "")
	if err == nil {
		t.Error("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("expected 'unsupported format' in error, got: %v", err)
	}
}

// --- Nil / empty data tests ---

func TestPrint_JSON_NilData(t *testing.T) {
	var buf bytes.Buffer

	if err := Print(&buf, nil, "json", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "null" {
		t.Errorf("expected 'null' for nil data, got: %q", output)
	}
}

func TestPrint_Table_NilData(t *testing.T) {
	var buf bytes.Buffer

	if err := Print(&buf, nil, "table", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "" {
		t.Errorf("expected empty output for nil table data, got: %q", output)
	}
}

func TestPrint_CSV_NilData(t *testing.T) {
	var buf bytes.Buffer

	if err := Print(&buf, nil, "csv", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "" {
		t.Errorf("expected empty output for nil csv data, got: %q", output)
	}
}

func TestPrint_Table_WithFieldsFilter(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"id": "1", "name": "First", "status": "ACTIVE"},
		{"id": "2", "name": "Second", "status": "PAUSED"},
	}

	if err := Print(&buf, data, "table", "id,name"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "1") || !strings.Contains(output, "First") {
		t.Error("expected filtered table to contain 'id' and 'name' values")
	}
	// status values should not appear as standalone fields
	// (they may appear as substrings, but the header should not contain "status")
	lines := strings.Split(output, "\n")
	if len(lines) > 0 && strings.Contains(strings.ToLower(lines[0]), "status") {
		t.Error("expected 'status' column to be filtered out of table header")
	}
}

// --- helper types for tests ---

type simpleError struct {
	msg string
}

func (e *simpleError) Error() string { return e.msg }

type wrappedError struct {
	msg   string
	inner error
}

func (e *wrappedError) Error() string { return e.msg }
func (e *wrappedError) Unwrap() error { return e.inner }
