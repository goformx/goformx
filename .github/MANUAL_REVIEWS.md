# Manual cross-provider reviews

| Code author | Reviewer | Maintainer's new PR comment | Integration |
| --- | --- | --- | --- |
| Codex / ChatGPT | Claude | @claude review | Pinned Claude Code Action, native progress tracking |
| Claude | Codex | @codex review | Hosted Codex GitHub integration; no repository Action |

The maintainer chooses the reviewer, including for mixed-provider changes.
Agents may request a review only when explicitly authorized; never start reciprocal
review loops. GitHub authorship does not identify the authoring provider.
Feedback is advisory, not approval or a substitute for required checks.

## Claude: supported tracking, constrained to review

The workflow adapts the official
[progress-tracked review example](https://github.com/anthropics/claude-code-action/blob/a874e9ecd7bb36efdad65429c6b35815f5a08f10/examples/pr-review-comprehensive.yml)
at the same immutable action revision. It retains our manual-only trigger and
subscription authentication, not the example's automatic PR trigger.

Only a new PR conversation comment whose entire body is @claude review from
the human maintainer jonesrussell starts the job. Edits, pushes, labels, other
users and bot comments do not. Preflight requires an open PR and the subscription
secret and records its head/base. The workflow must be on the default branch
before GitHub uses it for issue_comment events.
The outer Actions condition is case-insensitive; trusted JavaScript preflight
enforces the exact case and rejects extra text or newlines before any model call.

With a custom prompt, track_progress: true selects the action's native tracking
path. The action creates a progress comment and gives Claude a tool to update
that comment with findings. Its finalizer marks caught execution failures as
errors and links the run. We no longer parse SDK transcripts, resume the CLI,
render a second report or upload custom diagnostics artifacts.

### Read-only adaptations

Tracking mode is implementation-capable by default. At the pinned revision:

- Tag mode checks out the PR head and adds implementation permissions.
- User arguments follow the mode defaults, but allowed-tool lists accumulate.
- We override the permission mode, restrict built-in tools to Read/Glob/Grep,
  explicitly deny shell execution, edits, delegation, CI-log tools and file-op
  MCP tools, and retain only the supplied comment-writing capability.
  Some denials are defense in depth for capabilities not enabled at this pin.
- Hooks are disabled; only user settings and the action's MCP configuration load.
  The action restores PR-controlled Claude configuration from the base branch.
- GITHUB_TOKEN has contents: read and pull-requests: write; no code-write grant.
  The reviewer must not execute repository code or claim tests ran.
  Tag mode puts its job token in the checkout's Git remote configuration; Read
  denies cover .git and its contents, including symlink targets. Denying Git
  metadata also prevents Grep/Glob from searching that directory.
- Native context lists changed files but omits their patches. A trusted setup
  step downloads the actual diff to runner-temporary storage for Read, with a
  1 MiB bound and no truncation. Missing/invalid/oversized diffs fail before Claude.
- Post-review validation is inline trusted workflow code. It never imports a
  helper from the PR checkout. A changed head/base, closed PR, wrong checkout,
  or failed revision lookup fails the job and posts a STALE or UNVERIFIED warning.

Review this composition again when changing the action pin, especially
[src/modes/tag/index.ts](https://github.com/anthropics/claude-code-action/blob/a874e9ecd7bb36efdad65429c6b35815f5a08f10/src/modes/tag/index.ts),
[the argument parser](https://github.com/anthropics/claude-code-action/blob/a874e9ecd7bb36efdad65429c6b35815f5a08f10/base-action/src/parse-sdk-options.ts),
and [the execution finalizer](https://github.com/anthropics/claude-code-action/blob/a874e9ecd7bb36efdad65429c6b35815f5a08f10/src/entrypoints/run.ts).
These restrictions are our adaptation, not guarantees made by a stock recipe.

### Failure and revision limits

A successful process is not proof that a complete review was posted. Confirm the
native comment contains a final review, the run succeeded, and the head/base
still match. A failed or incomplete run is never a clean verdict.

Native comments use upstream sanitization and secret redaction, but remain
provider-authored Markdown, not our previous inert-text
publisher. An error finalizer or stale warning does not retract existing
findings. Disregard feedback from failed, incomplete, stale or unverified runs.
The post-check is a point-in-time guard, not protection against later pushes.

Setup failures before comment creation, hard cancellation/timeouts, runner loss
or GitHub outages can prevent terminal reporting. Inspect the Actions run when a
comment is absent or still running. No raw transcript artifact or automatic
recovery call is retained. Native reporting cannot recover findings from an
earlier runner or repair the SDK's turn-limit behavior.

## Subscription and spending policy

Claude uses the repository secret CLAUDE_CODE_OAUTH_TOKEN from the maintainer's
Max subscription. Generate it with claude setup-token and store it using the
hidden interactive prompt of gh secret set; never put credentials in source,
issues, chat, command arguments or review text. Use a separate repository secret
for each authorized repository. See
[official authentication options](https://code.claude.com/docs/en/github-actions).

There is no API-key fallback, managed Code Review service, paid overflow or
automatic subscription retry. Keep Claude extra usage disabled. Requested
research is limited to 16 turns and eight minutes; the job has a 20-minute limit.
The pinned action rejects even SDK success reported beyond the turn limit.
This workflow does not attempt a second synthesis call.

Actions minutes also count toward GitHub's allowance. Keep paid Actions overage
disabled. These workflow bounds do not enforce external account spending
settings. Keep full-output and Actions step-debug logging disabled. Do not
upload transcripts or enable debugging to recover a failed review.

## Codex recipe audit

Use the [official hosted GitHub review setup](https://learn.chatgpt.com/docs/third-party/github):
configure Codex cloud for this repository and enable repository Code review.
Leave automatic reviews and personal auto-review disabled. Request @codex review
manually; expect an acknowledgement followed by GitHub review feedback.

Current documentation says GitHub reviews flag P0/P1 issues. No findings is not
a comprehensive tech-debt audit. Root AGENTS.md contains three scoped Code Review
Rules. Other @codex requests can start implementation tasks; do not request fixes
as part of this review-only workflow.

The separate [Codex Action recipe](https://learn.chatgpt.com/docs/github-action)
uses openai/codex-action with an OpenAI API key and a separate feedback-posting
step. We do not add it: that documented authentication path does not fit our
subscription-only policy.

Use the included plan allowance and keep paid credit use disabled; wait for
reset rather than buying overflow. See [usage documentation](https://learn.chatgpt.com/docs/pricing).
Account connections, review toggles, allowances and spending controls were not
verified or changed by this repository audit. Verify them before a live request.

## Verification and rollout

Run from the repository root:

    node --test .github/scripts/manual-review.test.cjs
    go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -shellcheck= -pyflakes= .github/workflows/manual-claude-review.yml .github/workflows/manual-review-tests.yml

The Manual review workflow tests job uses Node 22, pinned actionlint, mocked
GitHub responses and no model calls or subscription secrets. It checks request
eligibility, workflow guardrails and executable preflight/revision-check behavior;
it does not prove the provider's runtime tool enforcement or native delivery.

After independent review and an authorized merge, a separately authorized live
request must verify native final-comment delivery and failure handling on the
exact reviewed revision. Do not merge or retry an unrelated PR to test this
workflow. Do not treat the existence of this file as successful account setup.
