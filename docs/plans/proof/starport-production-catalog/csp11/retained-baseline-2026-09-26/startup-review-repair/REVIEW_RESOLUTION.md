# Startup review repair

The Sol/Opus review at `e7710f16f` returned one P2 finding from Sol.
Opus returned no findings. The startup path read a fleet snapshot before lease acquisition.
The former owner could publish and release ownership between those operations.
The exact-predecessor fence rejected the stale startup publication.

The regression reproduces that failure without timing sleeps.
A test store returns the old snapshot after the real runtime publishes a new observation and releases its lease.
The replacement must start with the latest content and publish under its own grant.
The test fails before repair and passes afterward with race detection.

The repair reloads shared state after capturing the newly acquired grant.
It checks acquisition access against the updated retained inputs before publication.
Unpinned startup reselects effective state, including an explicit pin release.
The final storage transaction still checks the original grant and exact predecessor.

The repair changes startup only. It adds no storage call to the inference path.
The test now belongs to the A13 stale-epoch acceptance group.

All 122 fleet, acquisition, and pin race results pass with no failures or skips.
Full repository verification passes. Commit `b46c077a7` contains the repair.
The revised Sol/Opus review remains active. Both merges remain open.
