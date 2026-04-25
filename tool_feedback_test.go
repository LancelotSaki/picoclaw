package utils

import (
	"encoding/json"
	"testing"
)

func TestFormatArgsJSON_Defaults(t *testing.T) {
	args := map[string]any{"path": "README.md", "line": 42}
	got := FormatArgsJSON(args, false, false)
	var gotVal, wantVal any
	if err := json.Unmarshal([]byte(got), &gotVal); err != nil {
		t.Fatalf("FormatArgsJSON() returned invalid JSON: %v", err)
	}
	want := `{"path":"README.md","line":42}`
	if err := json.Unmarshal([]byte(want), &wantVal); err != nil {
		t.Fatalf("invalid test want JSON: %v", err)
	}
	if !jsonValEq(gotVal, wantVal) {
		t.Fatalf("FormatArgsJSON() = %q, want %q", got, want)
	}
}

func TestFormatArgsJSON_PrettyPrint(t *testing.T) {
	args := map[string]any{"path": "README.md", "line": 42}
	got := FormatArgsJSON(args, true, false)
	var gotVal any
	if err := json.Unmarshal([]byte(got), &gotVal); err != nil {
		t.Fatalf("FormatArgsJSON() returned invalid JSON: %v", err)
	}
	want := `{"path":"README.md","line":42}`
	var wantVal any
	if err := json.Unmarshal([]byte(want), &wantVal); err != nil {
		t.Fatalf("invalid test want JSON: %v", err)
	}
	if !jsonValEq(gotVal, wantVal) {
		t.Fatalf("FormatArgsJSON() prettyPrint = %q, want structure %q", got, want)
	}
}

func TestFormatArgsJSON_DisableEscapeHTML(t *testing.T) {
	args := map[string]any{"msg": "a < b && c > d"}
	got := FormatArgsJSON(args, false, true)
	var gotVal, wantVal any
	want := `{"msg":"a < b && c > d"}`
	if err := json.Unmarshal([]byte(got), &gotVal); err != nil {
		t.Fatalf("FormatArgsJSON() returned invalid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantVal); err != nil {
		t.Fatalf("invalid test want JSON: %v", err)
	}
	if !jsonValEq(gotVal, wantVal) {
		t.Fatalf("FormatArgsJSON() disableEscapeHTML = %q, want %q", got, want)
	}
}

func TestFormatArgsJSON_PrettyPrintAndDisableEscapeHTML(t *testing.T) {
	args := map[string]any{"msg": "a < b && c > d"}
	got := FormatArgsJSON(args, true, true)
	var gotVal, wantVal any
	want := `{"msg":"a < b && c > d"}`
	if err := json.Unmarshal([]byte(got), &gotVal); err != nil {
		t.Fatalf("FormatArgsJSON() returned invalid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantVal); err != nil {
		t.Fatalf("invalid test want JSON: %v", err)
	}
	if !jsonValEq(gotVal, wantVal) {
		t.Fatalf("FormatArgsJSON() combined = %q, want %q", got, want)
	}
}

func TestFormatArgsJSON_EscapeHTMLByDefault(t *testing.T) {
	args := map[string]any{"msg": "a < b && c > d"}
	got := FormatArgsJSON(args, false, false)
	var gotVal, wantVal any
	want := `{"msg":"a \u003c b \u0026\u0026 c \u003e d"}`
	if err := json.Unmarshal([]byte(got), &gotVal); err != nil {
		t.Fatalf("FormatArgsJSON() returned invalid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantVal); err != nil {
		t.Fatalf("invalid test want JSON: %v", err)
	}
	if !jsonValEq(gotVal, wantVal) {
		t.Fatalf("FormatArgsJSON() default escape = %q, want %q", got, want)
	}
}

func TestFormatArgsJSON_NilArgs(t *testing.T) {
	got := FormatArgsJSON(nil, false, false)
	want := `null`
	if got != want {
		t.Fatalf("FormatArgsJSON() nil = %q, want %q", got, want)
	}
}

func jsonValEq(a, b any) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}
