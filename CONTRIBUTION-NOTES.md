# Calendar enhancement contribution candidate

This branch is a contribution staging branch for roygabriel/mcp-icloud-calendar,
not a separate product or an upstream-approved release. Original MIT attribution
and upstream history are retained.

## AI disclosure

The added implementation and tests were generated with OpenAI Codex under Justin
Meader's direction, with automated testing and additional AI-assisted code review.
This disclosure does not imply independent human line-by-line review. The
contributor remains responsible for the submission and maintainer feedback.

## Proposed changes

- Read stored event reminders and richer iCalendar fields.
- Add get_event and exact resource-path read/update support when UID differs from filename.
- Create, replace and clear relative DISPLAY reminders; omitted reminders preserve existing alarms.
- Create DAILY/WEEKLY/MONTHLY/YEARLY recurring events and replace or remove whole-series schedules.
- Validate rules before mutation, preserve local start clock across DST, and reject unsupported exception edits.
- Bound recurrence expansion and preserve event properties not explicitly changed.
- Retain an existing iCloud REPORT compatibility fix; compare it with related open upstream PRs before submission.

The deployed implementation passed race tests, vet and disposable past-dated
iCloud event/series round-trips, including deletion with subsequent 404 checks.
Those live tests exercised the private deployment version. This contribution
branch preserves newer upstream tests and build configuration; it requires its
own checks and final maintainer review before acceptance. No credentials, private
tunnel implementation or personal deployment configuration are included.

## Scope boundaries

No individual-occurrence editing, exception-series rule changes, device
notification history/settings, or advanced all-day/floating/named-zone time edits.
New recurring creation accepts an IANA zone; its default is TZ, or UTC if unset.

## Upstream process

The October 12, 2026 staging checks are historical evidence, not current
clearance. The October 22 review reproduced a raw REPORT href regression and an
unsafe series-start change with recurrence exceptions; both now have regression
coverage and fixes. REPORT paths are normalized to safe calendar-scoped paths,
including same-origin absolute hrefs and escaped resource filenames. Moving a
series start or changing its rule is rejected before mutation if exceptions or
detached occurrences exist.

Recurrence expansion now caps yielded-candidate work including historical
occurrences, bounds timeset allocation and selector sizes, and rejects unsafe
stored intervals and nonstandard BYEASTER offsets. These limits do not bound the
recurrence dependency's internal scan through empty periods in sparse or
impossible schedules; they must not be described as a complete CPU-time bound.

The feature candidate preserves upstream dependency, toolchain, CI, release,
Docker and configuration files exactly. Runtime/security maintenance is isolated
on `maintenance/calendar-runtime-2026-10-22`: Go 1.26.10, the previously staged
jsonparser/x/sys fixes, and explicit explanations for operator-controlled file
reads flagged by the current linter. The combined, tested fork version is on
`dogfood/calendar-enhancements`; using that branch name does not establish what
commit is deployed. Read-only live connector checks confirm the custom tools
are available, but the exact container/source provenance and rollout require
infrastructure access.

Verification is reported separately for the feature candidate and the combined
fork version. The base-only feature candidate encounters the existing config
file-read lint findings and dependency scan findings; the maintenance branch
addresses those separately. The upstream discussion remains pending, and no
feature PR has been opened. Larger features should be split by direct reads,
reminders and recurrence after the maintainer agrees on scope.

The upstream README asks for an issue discussing larger changes before a PR.
Use that discussion to agree on scope and split this staging branch into focused
PRs. Check existing iCloud fixes for overlap. Complete upstream CI requirements
(race tests, vet, 80% total coverage, lint, vulnerability scan and build) before
presenting a PR as ready. Passing deployment checks alone is insufficient.
