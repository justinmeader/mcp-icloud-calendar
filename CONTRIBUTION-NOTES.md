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

Contribution-branch verification on October 12, 2026: race tests, vet, build and
golangci-lint 2.11.3 pass. Total statement coverage is 84.4%, above the 80% gate.
Go 1.26.9 is selected in the toolchain, CI, release and Docker builder settings;
jsonparser 1.1.2 and x/sys 0.44.0 address scanner findings. Govulncheck 1.9.0
reports no vulnerabilities, including verbose package/module checks. Regression
tests cover the actual REPORT projection and read decorators. Temporary diagnostic
logging of REPORT bodies was removed. Gitleaks working-tree and full-history scans
found no credentials; the added changes contain no private deployment coordinates.
Two narrow gosec annotations explain existing operator-configured file reads;
client-supplied resource paths continue to be validated.

These results supersede the initial 77.6% staging result. The contribution still
awaits upstream scope discussion and has not been submitted or accepted as a PR.

The upstream README asks for an issue discussing larger changes before a PR.
Use that discussion to agree on scope and split this staging branch into focused
PRs. Check existing iCloud fixes for overlap. Complete upstream CI requirements
(race tests, vet, 80% total coverage, lint, vulnerability scan and build) before
presenting a PR as ready. Passing deployment checks alone is insufficient.
