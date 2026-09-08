# Starport application fixture correction

The first resumed ownership gate failed O01 and O03. The focused reproduction identifies two tests that pass non-private temporary directories directly to Starmap.
The runtime correctly rejects those paths. The application fixture must supply a path that permits private runtime creation.

Starport commit `0acf476` selects a fresh child path under the test's temporary root.
Both direct runtime tests now close their runtime during cleanup. A comment correction satisfies the changed-file prose gate.
Production behavior and permission checks remain unchanged.

The [verification record](app-private-fixture/verification.json) retains the initial failures and subsequent checks.
Five focused race results pass, with no failures or skips. Package lint and final changed-file prose checks pass.
The full publication checks continue. These local workspace results do not qualify the published module or released pair.
