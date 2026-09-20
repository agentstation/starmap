# Badger expiry and concurrent writes

Commit `c18b22f686f56e9eb70df3461bbcbe00deddc47a` updates expiry in one Badger transaction.
The transaction reads the current value and applies its deadline together.
Conflicts cause bounded retries. Cancellation stops retries before another transaction.

The previous read-then-write sequence lost successful increments in all three regression runs.
Each run attempted 1,000 increments. The final values were 427, 424, and 435.
After the repair, all ten race-enabled runs retain 1,000 increments.
A separate cancellation test preserves both the value and its original deadline.

[The evidence record](badger-expiry.json) identifies commands, counts, and source.
Storage and usage package checks pass with 266 test events.
The run skipped 26 optional Valkey cases without a configured service.
This result qualifies the Badger repair only.

The budget probe still admits two requests against a one-token limit.
All three runs now record two consumed tokens.
CSP12.2 must add atomic admission before strict concurrent budgets can pass.
CSP15 still needs its complete backend qualification, review, and merge.
