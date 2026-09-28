# Deployment inventory and capture commands

Configuration owns the selected-file inventory through its canonical path manifest.
Every active role needs an explicit capture rule.
Native adapters capture databases and blob objects.
Bounded file inspection preserves runtime evidence, baseline files, credential policy, workspace recovery files, local tokens, selected configuration, and transport trust.
Portable artifact names retain original file names through the private inventory mapping.

Loaded configuration must match its original digest during copy.
An absent optional default configuration file remains valid.

`backup close` changes the SQL recovery record without starting the gateway.
A missing record becomes closed. A repeated close preserves its epoch.
The operator must separately stop and fence every writer.

`backup create` requires existing stores, a closed epoch, selected-key access, and explicit references for fencing and key recovery.

`backup verify` checks the retained digest and key challenge without opening live stores.
These commands cannot approve admission.

The source interface exposes enumeration and close, with no import methods.
Badger opens read-only on Linux and macOS.
Windows requires a native exclusive open and can recover engine state.
Application maintenance stays disabled during that open.
Capture refuses an empty source directory without a manifest.
Native Windows execution remains UNVERIFIED.

Valkey binds its observed incarnation and refuses a pending import barrier.

The focused cohort passes 50 race results and 50 pure-Go results without failures or skips.
The broader configuration and CLI suite passes 499 results with one optional container-recipe skip.
The Badger cohort passes 68 results. Its final empty-directory guard also passes in the focused cohorts.

Linux and Windows cross-compilation passes. It does not qualify native behavior.
Full lint, affected-package vet, and six dependency checks pass.

Reference validation, historical credential decryption, independent later history, and full restore remain required.
Native qualification, pre-PR review, and merge remain open.
No additional CSP13 acceptance subcase is complete.
