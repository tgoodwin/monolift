# PocketBase Summary

## Scope

- Traces analyzed: 11 (`pocketbase/M-1` through `M-11`)
- Codebase size: ~122k LOC
- Dominant path shapes: PocketBase command startup, router binding, request event handlers, hook dispatch, filesystem/S3 operations, mailers, and auth helpers.

## Hook and Event Boundaries

PocketBase hooks provide real contracts, so their edges are useful boundary evidence. They are not automatically clean cuts: request events, app receivers, transaction callbacks, and hook continuation callbacks often need access back into the app. The best cuts are usually below the hook shell, after the event has been converted into a file operation, mail operation, OAuth provider call, password hash/check, or relation expansion operation.

## Callback Pressure

Hook chains create callback pressure in `M-7` and `M-9`, where the framework can invoke user hooks before continuing to the concrete send or transaction work. Cutting at the concrete helper (`SMTPClient.send`, `safeFileFromURL`) avoids turning the hook continuation into a reverse network callback.

## Synthesis Notes

- Password hashing/checking traces behave as pure leaves despite being reached through auth request handlers.
- Filesystem and S3 traces are client-reconstructible but may need stream/proxy treatment when the cut is above the concrete file operation.
- PocketBase confirms that hook interfaces are strong evidence for reachability, but boundary placement should usually occur below the hook event object when the target is a concrete operation.
