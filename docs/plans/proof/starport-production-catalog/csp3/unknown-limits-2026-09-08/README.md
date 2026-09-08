# Unknown model limits

Reconciliation now preserves an explicit unknown limit when no source supplies a known value. A known fallback still takes precedence over unknown data. Every selected limit retains its original source receipt.

The runtime test covers all six limits in six scenarios. The scenarios cover positive values, zero, unknown and missing values with a fallback, and unknown and missing values without one. Each scenario verifies publication, filesystem persistence, restart, source refresh, and retained observation history.

The old code records three failure events and five passing events. The unknown-without-fallback scenario loses all six values and their receipts. The focused correction passes eleven events on each Go toolchain. Counts include parent tests and package events.

The [field audit](../field-presence-audit-2026-09-08.md) records other CSP3 gaps. Its initial metadata probe records two failure events. The expanded composite probe records four failure events.

The optional-record probe records sixteen failure events and one passing event. The description selector probe records two failure events. These probes do not qualify transport or Starport admission.

Commit `69c2b3a5` passes 876 runtime, acquisition, and reconciler race events. The [verification record](verification.json) binds the source and captures. This repair adds no primary acceptance credit. Publication review and the released-pair gate remain required.
