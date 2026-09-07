# CSP0.3 current-release demonstration

CSP0.3 passes locally. Starport commit `02efa34` adds the first-use animation, static poster, transcript, uncut capture, and reproduction sources.
E03 passes for the captured v1.2.0 release. The final campaign recording and released-pair acceptance remain UNVERIFIED.

The earlier console tour had 11 frames over 17.1 seconds. It did not show installation, provider setup, or real inference.
The [baseline review](csp0.3/tour-baseline.json) records those missing scenes.

The replacement uses the actual macOS ARM64 release archive. Its SHA-256 and binary digest match the earlier native release evidence.
The catalog read precedes the provider credential. The recorded OpenAI response completes with HTTP 200 and `[DONE]`.

The request takes 1.700 seconds, including provider time. The temporary process exits successfully and leaves no files in its temporary home.
The [capture record](csp0.3/capture.json) retains command results, output timestamps, stream events, and cleanup observations without credential values.

The edited GIF is 1280 × 800, 38.160 seconds, and 516,182 bytes. The uncut version is 64.450 seconds and 512,086 bytes.
At a 900-pixel display width, the body text has an effective size of 18.28 pixels.

The credential-entry gap shrinks to eight seconds. The installed-version and final frames gain reading time.
No edit intersects inference. GIF timestamps round to centiseconds.

The [media review](csp0.3/media-review.json) binds the current source and output artifacts.
It covers scene order, reading time, credential redaction, light and dark previews, keyboard focus, missing-image fallback, and a 320-pixel viewport.
The browser check uses a local HTML preview of the README media block. It does not claim a hosted GitHub rendering.
Narrow readers can open the full-size image or use the transcript. The README uses a static poster, so it does not autoplay.

The [E03 report](csp0.3/product.json) passes. The [renewed E02 check](csp0.3/e02-renewed.json) passes after the README header change.
All 43 verifier tests pass. They reject incomplete streams, stale artifacts, false dimensions, and edits within inference.
The README checks, their regression tests, documentation links, and changed prose pass.

The first renderer could not load its native terminal dependency. The final renderer uses Pillow and the recorded timestamps.
It does not repeat installation or provider calls. Capture and rendering sources ship with the media.
Pre-PR review remains pending before these local commits can publish.
