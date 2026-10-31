---
name: checking-overdue-invoices
description: Builds an accounts receivable aging report from an invoice CSV, showing who owes what, how late, and who to chase first. Use when the user asks about overdue or unpaid invoices, collections, who owes them money, late payers or AR aging.
---

# Checking overdue invoices

Call `receivables_aging_report` instead of adding invoices up yourself, because the buckets and totals need exact arithmetic. If it can't find columns, the error says how to name them, and `receivables_inspect_csv` shows what it detected. If dates are ambiguous, ask whether they're day-first or month-first. The report date defaults to today.

## Reporting results
- Lead with total open, total overdue and the buckets.
- Then list who to chase first, with their oldest invoice.
- Report `credits`, `warnings` and `skipped_rows`.

Ask the user to check balances before contacting anyone, since the export may be out of date. You may draft reminders if asked, but don't send anything without approval. This isn't accounting or legal advice.
