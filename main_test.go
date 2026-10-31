package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/GillMacedo/latepayers/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const sample = "examples/invoices.csv"

var fixed = []string{"--as-of", "2026-10-31"}

func TestCheckPrints(t *testing.T) {
	var out, errs bytes.Buffer
	if code := run(append([]string{"check", sample}, fixed...), &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	for _, want := range []string{"Chase first:", "Fictional Cafe", "usually pays 25.0 days late", "usually pays 3.0 days early", "not late", "CREDIT  INV-2010", "SKIPPED row"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
}

func TestOptionsCanFollowThePath(t *testing.T) {
	var out, errs bytes.Buffer
	if code := run(append([]string{"check", sample, "--json"}, fixed...), &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	var body map[string]any
	if err := json.Unmarshal(out.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["as_of"] != "2026-10-31" || len(body["customers_by_priority"].([]any)) != 4 {
		t.Errorf("got %v", body)
	}
}

func TestErrorsExitTwo(t *testing.T) {
	var out, errs bytes.Buffer
	for _, args := range [][]string{{"check"}, {"check", "/no/such.csv"}, {"nope"}, {}, {"check", sample, "--as-of", "soon"}, {"check", sample, "--columns", "oops"}, {"check", sample, "--date-format", "x"}} {
		if code := run(args, &out, &errs); code != 2 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
}

func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	clientSide, serverSide := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.New("test", func() time.Time { return time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC) }).Connect(ctx, serverSide, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, clientSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func call(t *testing.T, name string, args map[string]any) (map[string]any, *mcp.CallToolResult) {
	t.Helper()
	result, err := connect(t).CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{}
	if !result.IsError {
		data, _ := json.Marshal(result.StructuredContent)
		json.Unmarshal(data, &body)
	}
	return body, result
}

func TestServerListsReadOnlyTools(t *testing.T) {
	list, err := connect(t).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != 2 {
		t.Fatalf("got %d tools", len(list.Tools))
	}
	for _, tool := range list.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s isn't read-only", tool.Name)
		}
	}
}

func TestServerReports(t *testing.T) {
	body, _ := call(t, "receivables_aging_report", map[string]any{"path": sample})
	if body["customer_count"] != float64(4) || body["as_of"] != "2026-10-31" {
		t.Errorf("got %v", body)
	}
	body, _ = call(t, "receivables_aging_report", map[string]any{"path": sample, "limit": 0})
	if body["truncated"] != true || len(body["customers_by_priority"].([]any)) != 0 {
		t.Errorf("got %v", body)
	}
	_, result := call(t, "receivables_aging_report", map[string]any{"path": sample, "as_of": "soon"})
	if !result.IsError {
		t.Error("a bad date should be a tool error")
	}
}

func TestServerInspectHasNoRowContents(t *testing.T) {
	body, result := call(t, "receivables_inspect_csv", map[string]any{"path": sample})
	if body["invoices"] != float64(10) {
		t.Errorf("got %v", body)
	}
	data, _ := json.Marshal(result)
	if strings.Contains(string(data), "Fictional") {
		t.Error("inspect returned row contents")
	}
}

func TestServerBadPathIsAToolError(t *testing.T) {
	_, result := call(t, "receivables_aging_report", map[string]any{"path": "/no/such.csv"})
	if !result.IsError {
		t.Error("want a tool error")
	}
}

func TestSourceHasNoNetworkCode(t *testing.T) {
	for _, file := range []string{"main.go", "internal/aging/load.go", "internal/aging/report.go", "internal/aging/parse.go", "internal/server/server.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{`"net/http"`, `"net"`} {
			if strings.Contains(string(data), banned) {
				t.Errorf("%s imports %s", file, banned)
			}
		}
	}
}
