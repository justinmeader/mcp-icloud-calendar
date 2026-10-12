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

Contribution-branch verification on October 12, 2026: go test -race with coverage,
go vet and go build passed using Go 1.25.7. Total statement coverage is 77.6%,
below upstream's 80% gate. Lint and vulnerability scanning have not yet been run
for this candidate. This branch is not PR-ready.

The upstream README asks for an issue discussing larger changes before a PR.
Use that discussion to agree on scope and split this staging branch into focused
PRs. Check existing iCloud fixes for overlap. Complete upstream CI requirements
(race tests, vet, 80% total coverage, lint, vulnerability scan and build) before
presenting a PR as ready. Passing deployment checks alone is insufficient.
