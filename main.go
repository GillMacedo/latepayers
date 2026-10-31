// Command latepayers builds an accounts receivable aging report from an invoice CSV.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/GillMacedo/latepayers/internal/aging"
	"github.com/GillMacedo/latepayers/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.1.0"

const usage = `Usage:
  latepayers check <invoices.csv>   Report who owes what, and how late
  latepayers serve                  Run as an MCP server over stdio
  latepayers version

Options for check:
  --as-of 2026-12-31                Report date (default: today)
  --terms <days>                    Payment terms for rows with no due date (default 30)
  --date-format dmy|mdy             Needed only when dates are ambiguous
  --columns customer=Who,amount=Sum Name columns it doesn't recognize
  --json                            Print JSON

Reads your file and never changes it. Never uses the network.
`

func today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return 2
	}
	switch args[0] {
	case "serve":
		if err := server.New(version, today).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			fmt.Fprintf(stderr, "latepayers: %v\n", err)
			return 2
		}
		return 0
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	case "check":
		if err := check(args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "latepayers: %v\n", err)
			return 2
		}
		return 0
	}
	fmt.Fprint(stdout, usage)
	return 2
}

func check(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	asOfFlag := flags.String("as-of", "", "")
	terms := flags.Int("terms", 30, "")
	dateFormat := flags.String("date-format", "", "")
	columnsFlag := flags.String("columns", "", "")
	asJSON := flags.Bool("json", false, "")

	// The standard flag package stops at the first path, so options can't
	// follow it without this loop.
	var paths []string
	for rest := args; ; {
		if err := flags.Parse(rest); err != nil {
			return err
		}
		if flags.NArg() == 0 {
			break
		}
		paths = append(paths, flags.Arg(0))
		rest = flags.Args()[1:]
	}
	if len(paths) != 1 {
		return fmt.Errorf("check takes one CSV file\n%s", usage)
	}
	if *dateFormat != "" && *dateFormat != "dmy" && *dateFormat != "mdy" {
		return fmt.Errorf("--date-format must be dmy or mdy")
	}
	overrides := map[string]string{}
	if *columnsFlag != "" {
		for _, pair := range strings.Split(*columnsFlag, ",") {
			field, name, ok := strings.Cut(pair, "=")
			if !ok {
				return fmt.Errorf("--columns needs field=Column pairs, not %q", pair)
			}
			overrides[strings.TrimSpace(field)] = strings.TrimSpace(name)
		}
	}
	asOf := today()
	if *asOfFlag != "" {
		parsed, err := time.Parse("2006-01-02", *asOfFlag)
		if err != nil {
			return fmt.Errorf("--as-of must look like 2026-12-31, not %q", *asOfFlag)
		}
		asOf = parsed
	}
	file, err := aging.Load(paths[0], overrides, *dateFormat, *terms)
	if err != nil {
		return err
	}
	report := aging.Build(file.Invoices, asOf)
	if *asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(struct {
			aging.Report
			SkippedRows []int `json:"skipped_rows"`
		}{report, file.Skipped})
	}

	fmt.Fprintf(stdout, "As of %s: %s open, %s overdue (%.1f%%)\n", report.AsOf, report.TotalOpen, report.TotalOverdue, report.OverduePct)
	var parts []string
	for _, name := range aging.Buckets {
		parts = append(parts, name+": "+report.Buckets[name])
	}
	fmt.Fprintln(stdout, strings.Join(parts, " | "))
	fmt.Fprintln(stdout, "Chase first:")
	for _, line := range report.Customers {
		late := ""
		if line.AvgDaysLate != nil {
			late = fmt.Sprintf(", usually pays %.1f days late", *line.AvgDaysLate)
			if *line.AvgDaysLate < 0 {
				late = fmt.Sprintf(", usually pays %.1f days early", -*line.AvgDaysLate)
			}
		}
		fmt.Fprintf(stdout, "  %s  %s overdue of %s open, oldest %d days%s\n", line.Customer, line.Overdue, line.Open, line.OldestDays, late)
		for _, invoice := range line.Invoices {
			status := fmt.Sprintf("%d days late", invoice.DaysOverdue)
			if invoice.DaysOverdue == 0 {
				status = "not late"
			}
			fmt.Fprintf(stdout, "    %s  %s  due %s  %s\n", invoice.Number, invoice.Open, invoice.Due, status)
		}
	}
	for _, credit := range report.Credits {
		fmt.Fprintf(stdout, "CREDIT  %s  %s\n", credit.Number, credit.Open)
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(stdout, "WARNING: %s\n", warning)
	}
	for _, row := range file.Skipped {
		fmt.Fprintf(stdout, "SKIPPED row %d: no customer, amount or date\n", row)
	}
	return nil
}
