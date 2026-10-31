# latepayers

Builds accounts receivable aging reports from invoice CSVs. It's a Go MCP server and CLI, with a Claude Code plugin wrapper.

## Architecture
- `internal/aging/parse.go`: amounts and dates. `load.go`: files and columns.
- `internal/aging/report.go`: buckets, customers and priority.
- `internal/server`: MCP tools. `main.go`: CLI.
- `npm/bin.js` and `scripts/build-npm.sh`: one npm package holding every platform's binary.
- `vendor/`: dependencies, committed so tests run with no network.

## Commands

    make test
    make lint    # plugin validation and MCP Inspector
    make demo    # the README example

## Rules
- Money is integer cents. Totals must always add up: a randomized test checks this.
- Bucket changes need a test at the exact boundary.
- Never guess day-first or month-first when a column could be either.
- Never drop a row silently. Skipped rows are reported.
- Never add network access, writes to input files, or a way to contact customers.
- Sample data stays obviously made up.
- Never write to stdout in server code, because it corrupts JSON-RPC.

## Definition of done
Tests pass, the README example matches, and `CHANGELOG.md` and `ROADMAP.md` are updated if relevant.
