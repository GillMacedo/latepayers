// Package server exposes the aging report as MCP tools.
package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GillMacedo/latepayers/internal/aging"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ReportInput is the input of receivables_aging_report.
type ReportInput struct {
	Path       string            `json:"path" jsonschema:"Path to the invoice CSV"`
	AsOf       string            `json:"as_of,omitempty" jsonschema:"Report date, YYYY-MM-DD. Default is today"`
	TermsDays  *int              `json:"terms_days,omitempty" jsonschema:"Payment terms for invoices with no due date, default 30"`
	DateFormat string            `json:"date_format,omitempty" jsonschema:"dmy or mdy, needed only if the tool reports that dates are ambiguous"`
	Columns    map[string]string `json:"columns,omitempty" jsonschema:"Column names by field (number, customer, issued, due, amount, paid, balance, paiddate, currency), when detection fails"`
	Limit      *int              `json:"limit,omitempty" jsonschema:"Most customers to return, highest priority first, default 100. 0 returns only the totals"`
}

// ReportOutput is the output of receivables_aging_report.
type ReportOutput struct {
	aging.Report
	CustomerCount int   `json:"customer_count"`
	Truncated     bool  `json:"truncated"`
	SkippedRows   []int `json:"skipped_rows"`
}

// InspectInput is the input of receivables_inspect_csv.
type InspectInput struct {
	Path string `json:"path" jsonschema:"Path to the invoice CSV"`
}

// InspectOutput holds no row contents.
type InspectOutput struct {
	Columns  map[string]string `json:"columns"`
	Invoices int               `json:"invoices"`
	Skipped  int               `json:"skipped"`
}

func no() *bool { value := false; return &value }

// New returns the MCP server. The clock is a parameter so tests can fix the date.
func New(version string, now func() time.Time) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "latepayers", Version: version}, nil)
	annotations := &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: no(), IdempotentHint: true, OpenWorldHint: no()}

	mcp.AddTool(server, &mcp.Tool{
		Name:  "receivables_aging_report",
		Title: "Accounts receivable aging report",
		Description: "Builds an accounts receivable aging report from an invoice CSV: what's owed in each overdue bucket, by customer, with who to chase first and how late each customer usually pays. " +
			"Use for overdue invoices, collections and cash flow. Read-only, and never uses the network. Results include customer names and amounts from the file, so they reach the model. It can't contact anyone.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in ReportInput) (*mcp.CallToolResult, ReportOutput, error) {
		limit := 100
		if in.Limit != nil {
			limit = *in.Limit
		}
		if limit < 0 || limit > 500 {
			return nil, ReportOutput{}, errors.New("limit must be between 0 and 500")
		}
		asOf := now()
		if in.AsOf != "" {
			parsed, err := time.Parse("2006-01-02", in.AsOf)
			if err != nil {
				return nil, ReportOutput{}, fmt.Errorf("as_of must look like 2026-12-31, not %q", in.AsOf)
			}
			asOf = parsed
		}
		terms := 30
		if in.TermsDays != nil {
			terms = *in.TermsDays
		}
		file, err := aging.Load(in.Path, in.Columns, in.DateFormat, terms)
		if err != nil {
			return nil, ReportOutput{}, err
		}
		report := aging.Build(file.Invoices, asOf)
		out := ReportOutput{Report: report, CustomerCount: len(report.Customers), SkippedRows: file.Skipped}
		if len(out.Customers) > limit {
			out.Customers, out.Truncated = out.Customers[:limit], true
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "receivables_inspect_csv",
		Title:       "Inspect an invoice CSV",
		Description: "Shows which columns latepayers detects in an invoice CSV and how many rows it can read, without returning row contents. Use if the file is unfamiliar or a report failed.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in InspectInput) (*mcp.CallToolResult, InspectOutput, error) {
		file, err := aging.Load(in.Path, nil, "dmy", 30)
		if err != nil {
			return nil, InspectOutput{}, err
		}
		return nil, InspectOutput{Columns: file.Columns, Invoices: len(file.Invoices), Skipped: len(file.Skipped)}, nil
	})
	return server
}
