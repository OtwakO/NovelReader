---
status: completed
---

# Browser worker lifecycle hardening

## Goal

Bound retained browser work and keep ownership until cleanup completes. Failed/uncertain teardown must stop the worker instead of accumulating replacement browsers. No claim of mathematically zero memory leaks or constant RSS.

## Scope

Worker admission, interactive operations/close ownership, browser teardown and runtime shutdown. No reader schema changes, source rewrites, Java bridge expansion or unrelated type cleanup.

## Accepted Approach

- Use the existing bounded work queue for interactive opens as well as one-shot execution.
- Bound interactive operation admission and duration while preserving per-session serialization; cleanup remains available when admission is saturated.
- Give each session one close task, retain its registry entry until cleanup/release finishes, and reject new operations once closing starts. Caller cancellation must not abandon cleanup.
- Bound browser close, retain the handle on failure, mark the worker unavailable and request owner shutdown. Never create a replacement after uncertain teardown.
- Let the root runtime own bounded shutdown of requests, worker, Playwright and display. If graceful shutdown stalls, exit the worker. Hosted deployment relies on container/process-group supervision to remove descendants, not Python respawning itself.

## Decisions

User approved automatic worker restart after fatal cleanup, rather than waiting for manual recovery. The existing Compose sidecar uses `restart: unless-stopped`, and its Docker CMD runs Python directly as container PID 1. Container exit terminates remaining processes in its PID namespace. Native hosted service supervision must terminate the entire process control group; bare unsupervised native runs are not the hosted recovery guarantee. No new supervisor framework is planned.

Compatibility: existing busy responses and normal request/session contracts remain; saturated opens/interactive operations can now reject instead of waiting without a bound. In-flight browser work can fail during fatal recovery; never automatically replay user actions. Rollback is the logical lifecycle change set; no data reset/migration is needed.

## Current State

Completed the ownership/admission correction. Interactive opens share the bounded queue and request deadline; frame/input admission is separately bounded and operations serialize under a deadline. Sessions retain one shielded close task through disposal/release. The worker retains context-close, browser-close and release-accounting tasks independently of caller cancellation. Unconfirmed cleanup, interrupted remote allocation and unexpected disconnection stop admission and request root shutdown. Teardown deadlines use `asyncio.wait`, not cancellation-dependent `wait_for`; failure keeps the root hard-exit watchdog armed even while remaining asyncio tasks drain. Idle consumers drop prior payload references. HTTP reads/writes are bounded, failed writers are not retried, and late-arriving connections are rejected during shutdown. HTTP timeout/busy failures are explicit; no automatic replay or in-process respawn.

One independent review identified cancellation-dependent close deadlines, interrupted release accounting, and consumers surviving cancellation during recycling. All were addressed with ownership corrections and focused regressions. Context-close uncertainty also escalates immediately instead of allowing failed contexts to accumulate while another interactive session keeps the browser busy.

## Next Action

Completed. Deploy the updated worker under container/process-control-group supervision. Resume the separate BookSource compatibility audit; no additional lifecycle framework or tuning is planned without new evidence.

## Verification

- Baseline synthetic probes reproduced the three original defects; additional regressions cover cancellation-resistant teardown, interrupted allocation, release-accounting cancellation, concurrent/cancelled close ownership, admission saturation and runtime shutdown.
- Complete default worker discovery: 47 tests run, 46 passed, one optional live test skipped. Existing fixture browser-close methods were corrected to real async mocks rather than relying on swallowed TypeErrors. New tests are picked up by the existing CI `test_*.py` discovery without workflow changes.
- Real headless browser: 20 probes and 10 queued interactive create/close cycles, six browser recycles; zero contexts/active slots after every cycle, zero pending cleanup tasks and an empty queue at the end.
- Final-code isolated container test mounted the four changed runtime modules into an existing worker image. Real Chrome and driver processes were observed. Injected Chrome close and driver stop refused cancellation; Docker recorded two fatal exits, one automatic restart under `on-failure:1`, and final exited state/PID 0. The test container was removed. Container PID-namespace termination is the descendant boundary; no host-PID mode or self-respawn was used.
- This is not a fresh release-image build, hosted CI run, long-duration RSS soak, arbitrary native-supervisor verification or guarantee against every browser/library leak. Final AFT checks report no diagnostics for `worker.py`; five diagnostics remain on untouched `browser.py` code, with incomplete coverage elsewhere and gopls unavailable. No clean whole-worker static typecheck is claimed; runtime tests are the verification gate here.
