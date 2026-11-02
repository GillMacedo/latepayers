# latepayers: accounts receivable aging report from an invoice CSV

[![npm](https://img.shields.io/npm/v/latepayers)](https://www.npmjs.com/package/latepayers) [![CI](https://github.com/GillMacedo/latepayers/actions/workflows/ci.yml/badge.svg)](https://github.com/GillMacedo/latepayers/actions/workflows/ci.yml) [![MIT license](https://img.shields.io/badge/license-MIT-blue)](LICENSE) [![MCP Registry](https://img.shields.io/badge/MCP_Registry-listed-green)](https://registry.modelcontextprotocol.io/v0/servers?search=io.github.GillMacedo/latepayers)

**See who owes you money, how late each invoice is, and who to chase first. For freelancers, small businesses and bookkeepers tracking overdue invoices in spreadsheets.**

Late payments quietly drain cash flow. `latepayers` reads an invoice export as CSV and builds an AR aging report with the standard buckets, per customer, ranked by overdue money and age. It also shows how late each customer usually pays. It's an MCP server and a single Go binary, with no account, no API key and no network access.

## What you can ask your agent

- "Who owes me money, and how late are they?"
- "Which overdue invoices should I chase first?"
- "Build an accounts receivable aging report."
- "Which customers always pay late?"

## Install

As a Claude Code plugin, which adds the MCP server and a skill that tells Claude when to use it. You need Node, which runs the server through `npx`:

    claude plugin marketplace add GillMacedo/latepayers
    claude plugin install latepayers@latepayers-marketplace

For other MCP clients:

```json
{
  "mcpServers": {
    "latepayers": {
      "command": "npx",
      "args": ["-y", "latepayers", "serve"]
    }
  }
}
```

Or build the binary with Go 1.27 or newer:

    go install github.com/GillMacedo/latepayers@latest

## Usage

Without installing anything, run `npx latepayers check invoices.csv`. It uses today's date unless you pass `--as-of`. This example uses the made-up file in `examples/`:

    $ go run . check examples/invoices.csv --as-of 2026-10-31
    As of 2026-10-31: 5745.50 open, 3745.50 overdue (65.2%)
    not due: 2000.00 | 1-30: 500.00 | 31-60: 975.00 | 61-90: 0.00 | 90+: 2270.50
    Chase first:
      Example Supplies Ltd  1700.00 overdue of 1700.00 open, oldest 92 days
        INV-2001  1200.00  due 2026-07-31  92 days late
        INV-2002  500.00  due 2026-10-01  30 days late
      Demo Bakery  620.00 overdue of 620.00 open, oldest 122 days
        INV-2009  620.00  due 2026-07-01  122 days late
      Fictional Cafe  450.50 overdue of 450.50 open, oldest 139 days, usually pays 25.0 days late
        INV-2003  450.50  due 2026-06-14  139 days late
      Sample Widgets  975.00 overdue of 2975.00 open, oldest 42 days, usually pays 3.0 days early
        INV-2007  975.00  due 2026-09-19  42 days late
        INV-2006  2000.00  due 2026-11-09  not late
    CREDIT  INV-2010  -75.00
    SKIPPED row 13: no customer, amount or date

Add `--terms 45` to set payment terms for invoices with no due date, and `--json` for machine-readable output.

## How it calculates

| Value | Rule |
|---|---|
| **Open amount** | The balance column if there is one, otherwise total minus paid |
| **Due date** | The due date column, otherwise invoice date plus payment terms (default 30 days) |
| **Buckets** | Not due, 1-30, 31-60, 61-90 and 90+ days past due |
| **Chase first** | Ranked by the sum of each overdue amount times its days overdue |
| **Usual lateness** | The average of paid date minus due date, if the file has paid dates |

Negative balances are listed as credits and left out of the totals. Repeated invoice numbers and mixed currencies produce warnings.

## MCP tools

| Tool | What it does |
|---|---|
| `receivables_aging_report` | Returns totals, buckets and customers by priority, with a limit for large files |
| `receivables_inspect_csv` | Shows detected columns and row counts, without row contents |

Both are read-only, and neither can contact anyone. `receivables_aging_report` returns customer names and amounts to the MCP client, so they reach whichever AI model you use. The CLI keeps everything on your machine.

## FAQ

**What is an AR aging report?** It groups unpaid invoices by how many days past due they are, so you can see how much money is at risk and who to contact.

**Does it work with QuickBooks, Xero or FreshBooks exports?** It reads CSV and recognizes common column names such as `Balance Due`, and you can name others with `--columns customer=Who,amount=Sum`. It doesn't connect to those products, and it hasn't been tested on each one's real exports yet.

**Can it send payment reminders?** No. It tells you who to contact, and you or your agent can draft messages for you to approve.

## Limits

- This helps you manage collections and isn't accounting or legal advice.
- It's only as accurate as the export, so check balances before contacting a customer.
- It doesn't apply credits to invoices, and doesn't convert currencies.
- It treats each distinct customer name as one customer.
- It reads CSV only, up to 50 MB.
- So far it has been tested on made-up data, not real exports. If yours fails, please open an issue with the column headers, not your data.

## Contributing

Contributions are welcome, from people and from people working with AI agents. See [CONTRIBUTING.md](CONTRIBUTING.md). Good first issues:

- Add column names from more accounting tools and languages
- Add `--out` to write the report to a CSV
- Add Days Sales Outstanding (DSO) to the summary
- Add a by-invoice view sorted by days late
- Add a plugin icon

### How pull requests work

1. **Open a PR.** Fork the repo, work on a branch, and keep it to one change. Write a title that says what changes for users, because it becomes the changelog line: `feat:` goes under Added and `fix:` under Fixed.
2. **Automatic check.** Code changes need tests, and `make test` must pass. Changes over 30 lines need a linked open issue (`Closes #<number>`), and a PR can have at most 800 changed lines. If something is missing, a comment tells you what to fix.
3. **Review.** Claude reviews each PR automatically. Code PRs also get a security review and are tested in a sandbox. The review then merges the PR or asks for changes. PRs that change CI, agent settings, npm or Python dependencies, vendored code or build files can't be merged this way, and a comment says so. The same goes for anything the security review doesn't clear.
4. **Release.** A merge that changes what users install becomes a new version within about two hours, with a GitHub Release and, if the project has one, its npm package. Docs and CI changes wait for the next release.

## AI disclosure

This project was largely generated with Claude Code and reviewed by its maintainer.

## About me

I'm Giulliano Macedo, and I build developer tools at GoTech Solutions. Follow along or get in touch:

- GitHub: [GillMacedo](https://github.com/GillMacedo)
- LinkedIn: [giulliano](https://www.linkedin.com/in/giulliano/)
- X: [@giumacedo](https://x.com/giumacedo)

If `latepayers` helped you get paid, a star helps others find it.

---

MIT licensed.

Built by [Giulliano Macedo](https://github.com/GillMacedo), GoTech Solutions · [LinkedIn](https://www.linkedin.com/in/giulliano/) · [X](https://x.com/giumacedo)
