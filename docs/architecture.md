# Architecture

    invoices.csv ─> load ─> open invoices ─> buckets per customer ─> priority ─> CLI and MCP tools

**Loading:** finds the header in the first 20 rows. Amounts become integer cents. A balance column wins over total minus paid. A row with no due date uses the invoice date plus payment terms, and a row with neither date is reported as skipped.

**Report:** each open invoice lands in one bucket. Totals, buckets and customer amounts come from the same cents, and a randomized test checks they agree. Credits stay separate.

**Priority:** the sum of amount times days overdue, so a small old debt can outrank a large new one.

**Packaging:** one npm package holds all five binaries, and `npm/bin.js` picks the right one.

See `docs/decisions/`.
