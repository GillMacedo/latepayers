# 0001: Rank customers by overdue money times age

**Decision:** "chase first" sorts by the sum of each overdue amount times its days overdue.

**Why:** old debts get harder to collect, so ranking by amount alone hides them, and ranking by age alone puts tiny balances first.

**Trade-off:** the score isn't meaningful on its own, so it's used only for ordering and isn't shown.

**Alternatives:** a fixed order of buckets, which treats 91 days and 900 days alike.
