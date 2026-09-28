# Batch shutdown review

Status: local implementation review. CSP12.2 remains in progress.
The required pre-PR autoreview remains open.

The baseline probe showed that application Close returned success during active checkpoint retirement.
The application now drains batch workers before it closes dependencies.
A deadline returns an error and preserves dependencies for a later Close attempt.
This failure does not consume the application's final cleanup guard.

Worker registration precedes submission writes. A rejected submission releases its registration.
Shutdown rejects new submissions and cancels later dispatch. Admitted calls retain their execution context and can finish.
Worker completion follows result storage, aggregate recovery, checkpoint cleanup, and final slot recovery.
An incomplete run retains its durable claims and results for restart recovery.

The tests hold a metadata write, a provider call, and checkpoint retirement at their respective boundaries.
They verify timeout, retry, refused submission, retained capacity, and zero dispatch for untouched lines.
The HTTP controller maps a closed batch service to 503.
Server tests now drain their workers before temporary storage cleanup.

The existing runtime loop stops background sweeps before application Close.
This change owns batch workers. It does not redefine native-video cancellation or force-stop providers that ignore cancellation.
A caller must honor a Close error and retain dependencies until a successful retry.
Forced process termination remains a crash-recovery case.

Shared backends, native CI, capacity, full A47, and complete task qualification remain open.
The owner decision about untouched lines remains pending. Shutdown never authorizes their automatic execution.
