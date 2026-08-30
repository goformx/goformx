# Manual cross-provider reviews

The implementation author and reviewer should be different providers:

| Code author | Reviewer | New PR conversation comment |
| --- | --- | --- |
| Codex / ChatGPT | Claude | `@claude review` |
| Claude | Codex | `@codex review` |

Reviews are requested by the maintainer when the change is ready. Agents must
not request each other's reviews automatically. Do not infer the authoring
provider from the GitHub author: both tools can commit as the same person.
For mixed-provider work, the maintainer chooses which review to request.

The Claude workflow accepts only a newly created PR conversation comment from
`jonesrussell` whose entire body is `@claude review`. Opening a PR, pushing a
commit, editing a comment, adding a label, or receiving a bot comment does not
start Claude. A changed revision needs a fresh manual request. Ordinary GitHub
CI checks are unaffected.

The workflow posts a status comment immediately, then updates it with a summary
labeled **Claude review** under `github-actions[bot]`. Claude does not post it.
The job token can read repository contents and write PR feedback, but cannot
push repository code. The reviewer does not run project code or tests. Findings
include the head and base SHAs; a changed revision invalidates the review. Agent
feedback is advisory and never substitutes for required checks or human approval.

## Account setup

Claude uses `CLAUDE_CODE_OAUTH_TOKEN`, a GitHub Actions repository secret generated
with `claude setup-token` while signed into the maintainer's Claude Max plan.
Store it through `gh secret set CLAUDE_CODE_OAUTH_TOKEN --repo OWNER/REPO` using
the hidden interactive prompt. Never paste credentials into issues, PRs, source
files, chat, or command-line arguments. Use a separate repository secret for
each authorized repository, not an organization-wide subscription secret.

This workflow does not use `ANTHROPIC_API_KEY` and does not use the separately
billed managed Claude Code Review or ultrareview services. There is no API-key
fallback. Missing or expired subscription credentials fail the run. Max usage
limits still apply. Keep Claude extra usage disabled to avoid paid overflow.

GitHub-hosted execution consumes Actions minutes. Private repositories must stay
within the included GitHub allowance; keep the applicable Actions spending
budget set to stop usage at the included allowance to prevent overage charges.
Each job has a 20-minute timeout. Research is limited to 16 turns and 8 minutes.
Only turn exhaustion with a valid retained session allows one tools-disabled
summary attempt (2 turns, a 120-second process timeout and a 3-minute step limit).
Publication has a separate 2-minute allowance. These are bounds, not a guarantee
of zero cost if paid overflow has been enabled elsewhere.

Whichever research limit is reached first stops the step. The 8-minute limit
reserves time for failure reporting; it does not guarantee transcript recovery.
The pinned action writes its transcript after receiving a result or catching an
execution error, not continuously. A hard step timeout can therefore leave no
transcript: reporting then posts INCOMPLETE without evidence or a resume attempt.
Increasing the clock limit would not make transcript recovery unconditional.

For Codex, connect this repository through the ChatGPT GitHub app. Keep personal
**Auto review**, repository automatic review options, and **Enable credits use**
off. Then request `@codex review` manually. Reviews use the ChatGPT Pro review
allowance; wait for the allowance to reset rather than buying extra credits.

The workflow must be merged to the default branch before GitHub processes its
`issue_comment` trigger. The presence of this file alone does not establish that
account connections and secrets have been configured or a live review has passed.

## Failure reporting and recovery

The initial status comment links the workflow and records the requested head and
base. Workflow code, not a model tool call, updates that same comment on success,
turn exhaustion, missing/invalid output or setup failure. Head/base changes are
checked immediately before and after publication; stale output is withheld.
The fallback publisher is inline so it can run even when checkout failed.
If the post-publication revision lookup fails transiently, the same comment keeps
its sanitized evidence but becomes INCOMPLETE with an explicit unverified-revision
warning, and the job fails. A confirmed changed revision still suppresses evidence.

If research exhausts its budget, the action-installed CLI resumes its local
session once with **no tools**, hooks disabled and an empty MCP configuration.
This is synthesis of evidence already gathered, not another research pass. A
partial summary stays labeled **INCOMPLETE**, and the workflow stays failed.
It is never reported as a clean or completed review. Other provider/setup failures
are reported without automatically retrying subscription requests.

The workflow retains `manual-review-diagnostics-RUN-ATTEMPT` for seven days when
capture succeeds. Its JSON contains only an allowlisted execution status, turn
and duration counts, permission-denial counts/names and a resumability boolean.
It excludes prompts, tool arguments/results, arbitrary error messages, final
review text, session IDs and credentials. Unknown tool names become `other`.
Raw transcripts remain in runner-temporary storage and are **not uploaded**.
Keep full-output/debug logging disabled; the upstream action enables verbose
output when Actions step debugging is explicitly enabled.

Model-authored review text is bounded, credential-redacted and rendered as inert
text, not executable workflow input or active Markdown. This is defensive
filtering, not permission to expose secrets to the reviewer.

Hard job cancellation, runner loss or GitHub API outages can prevent final
delivery: the initial status/run link is the fallback, not a promise that a dead
runner can post a terminal comment. Inspect the linked run before retrying. No
comment means preflight could not reach GitHub either. Request a new review only
after fixing the cause and confirming the intended revision; do not weaken tool
permissions merely because calls were denied.

## Regression verification

Run `node --test .github/scripts/manual-review.test.cjs` and pinned actionlint:

```sh
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -shellcheck= -pyflakes= .github/workflows/manual-claude-review.yml .github/workflows/manual-review-tests.yml
```

The separate **Manual review reporting tests** workflow runs on changes to this
workflow, its helper/tests or this guide. It has no subscription secret or PR-write
permission. Tests use synthetic SDK/CLI records and mocked GitHub/provider calls;
they do not invoke Claude or post real comments. Live subscription/resume behavior
must still be observed after this workflow is merged to the default branch.

The pinned action exposes a local `claude-execution-output.json` transcript even
on turn exhaustion. Its SDK argument parser drops an empty `--tools` argument,
so tools-disabled synthesis invokes the **same action-installed CLI version**
directly rather than routing that argument through a second action invocation.
The native 2.1.251 CLI's `--help` explicitly documents `--tools ""` as disabling all
tools. That local parser contract is distinct from live hosted resume verification.
Recheck these interfaces when updating the action pin.

## References

- [Claude Code GitHub Actions](https://code.claude.com/docs/en/github-actions)
- [Codex GitHub reviews](https://learn.chatgpt.com/docs/third-party/github)
- [Codex pricing and limits](https://learn.chatgpt.com/docs/pricing)
