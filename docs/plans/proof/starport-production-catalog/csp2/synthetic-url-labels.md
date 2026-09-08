# Synthetic URL labels in retained test logs

The evidence scan reports one synthetic credentialed URL in 19 retained test logs as an unknown credential.
The source test constructs that URL with eight placeholder characters. The reports contain no provider credential.
The [earlier scan record](../csp3/publication-evidence-scan.json) identifies the same synthetic value in seven earlier captures.

The publishable logs now use a synthetic-label marker in place of that exact URL.
The [substitution manifest](synthetic-url-labels.json) records each original hash, current hash, and substitution count.
Reversing the substitution restores every original byte. Test statuses, counts, assertions, and timing values remain unchanged.

Historical verification hashes continue to identify the original captures. Retrieve those bytes from the manifest's original Git commit when checking historical hashes.
The current files are publication derivatives. The manifest binds them to their original captures without changing the historical verification records.
No scanner rule or test assertion changes. The pre-PR secret scan must pass before publication.
