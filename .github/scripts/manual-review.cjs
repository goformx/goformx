const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const MAX_FILE = 32 * 1024 * 1024;
// Keep aligned with the research action's --max-turns (guarded by regression).
const RESEARCH_MAX_TURNS = 16;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const TOOLS = new Set(['Read', 'Glob', 'Grep', 'Bash', 'Edit', 'Write', 'NotebookEdit', 'Agent', 'Task', 'WebFetch', 'WebSearch']);
const integer = value => Number.isSafeInteger(value) && value >= 0 ? value : 0;

function parseExecution(raw) {
  let messages;
  try { messages = JSON.parse(raw); } catch { return { status: 'invalid' }; }
  if (!Array.isArray(messages)) messages = [messages];
  const result = messages.findLast(m => m && m.type === 'result');
  if (!result) return { status: 'missing' };
  const status = result.subtype === 'success' && result.is_error === false ? 'success'
    : result.subtype === 'error_max_turns' ? 'turn_limit' : 'failed';
  const session = result.session_id || messages.find(m => m?.type === 'system' && m.subtype === 'init')?.session_id;
  const denials = Array.isArray(result.permission_denials) ? result.permission_denials : [];
  return {
    status,
    turns: integer(result.num_turns),
    duration_ms: integer(result.duration_ms),
    denied_count: denials.length,
    denied_tools: [...new Set(denials.map(d => TOOLS.has(d?.tool_name) ? d.tool_name : 'other'))],
    session: typeof session === 'string' && UUID.test(session) ? session : '',
    text: status === 'success' && typeof result.result === 'string' ? result.result : '',
  };
}

function readExecution(file) {
  try {
    if (fs.statSync(file).size > MAX_FILE) return { status: 'oversized' };
    return parseExecution(fs.readFileSync(file, 'utf8'));
  } catch { return { status: 'missing' }; }
}

// Allowlist projection only. Never serialize the transcript, tool inputs,
// model-generated error strings, final review text, session ID or environment.
function diagnostics(execution) {
  return {
    status: execution.status,
    turns: integer(execution.turns),
    duration_ms: integer(execution.duration_ms),
    denied_count: integer(execution.denied_count),
    denied_tools: execution.denied_tools || [],
    resumable: Boolean(execution.session),
  };
}

function directory(env) { return path.join(env.RUNNER_TEMP, 'manual-review-private'); }
function researchPath(env) { return path.join(directory(env), 'research.json'); }
function summaryPath(env) { return path.join(directory(env), 'summary.json'); }

function classifyResearch(execution) {
  // The SDK may report success beyond maxTurns; the pinned action rejects it
  // after retaining the transcript. Preserve its text, never its success verdict.
  return execution.status === 'success' && execution.turns > RESEARCH_MAX_TURNS
    ? { ...execution, status: 'over_budget' } : execution;
}

function readResearch(env) { return classifyResearch(readExecution(researchPath(env))); }

function saveDiagnostics(env, research = readResearch(env)) {
  fs.writeFileSync(path.join(env.RUNNER_TEMP, 'manual-review-diagnostics.json'), JSON.stringify({
    version: 1,
    research: diagnostics(research),
    summary: diagnostics(readExecution(summaryPath(env))),
  }, null, 2), { mode: 0o600 });
}

function prepare(env) {
  fs.mkdirSync(directory(env), { recursive: true, mode: 0o700 });
  const source = path.join(env.RUNNER_TEMP, 'claude-execution-output.json');
  const research = classifyResearch(readExecution(source));
  if (!['missing', 'invalid', 'oversized'].includes(research.status)) {
    fs.writeFileSync(researchPath(env), fs.readFileSync(source), { mode: 0o600 });
  }
  saveDiagnostics(env, research);
  fs.appendFileSync(env.GITHUB_OUTPUT, `summarize=${research.status === 'turn_limit' && Boolean(research.session)}\n`);
}

function summarize(env, run = spawnSync) {
  const research = readResearch(env);
  if (research.status !== 'turn_limit' || !UUID.test(research.session)) return;
  // Use the action-installed pinned CLI directly: the action's SDK argument
  // parser does not preserve an empty --tools value. No tools means no further
  // research, shell access, MCP calls or comment publication in this stage.
  const args = ['--print', '--resume', research.session, '--tools', '',
    '--max-turns', '2', '--output-format', 'json', '--permission-mode', 'default',
    '--setting-sources', 'user', '--strict-mcp-config', '--mcp-config', '{"mcpServers":{}}',
    '--settings', '{"disableAllHooks":true}',
    'Summarize only the review evidence already gathered. The research turn budget was exhausted. '+
    'State incomplete coverage and unresolved questions; do not claim a complete or clean review. '+
    'Give concrete findings with severity, file/line and impact, or say none were established. '+
    'Do not use tools, request another agent, post comments, or include secrets. Return final text only.'];
  const childEnv = { ...env };
  delete childEnv.GH_TOKEN;
  delete childEnv.GITHUB_TOKEN;
  const result = run(path.join(env.HOME, '.local/bin/claude'), args, {
    env: childEnv, encoding: 'utf8', timeout: 120000, maxBuffer: 4 * 1024 * 1024,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  // A timeout/nonzero exit is not a successful summary, even if stdout happens
  // to contain a success-looking record. Do not log raw stdout/stderr/errors.
  const output = result.status === 0 && !result.error ? result.stdout : '{"type":"result","subtype":"error","is_error":true}';
  fs.writeFileSync(summaryPath(env), output || '{}', { mode: 0o600 });
  saveDiagnostics(env);
}

function safeText(text, secrets = []) {
  for (const secret of secrets.filter(s => typeof s === 'string' && s.length > 0)) text = text.split(secret).join('[REDACTED]');
  return text
    .replace(/(?:gh[pousr]_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-ant-[A-Za-z0-9_-]+)/g, '[REDACTED]')
    .replace(/\bBearer\s+[^\s]+/gi, 'Bearer [REDACTED]')
    .replace(/[\u0000-\u0008\u000b-\u001f\u007f]/g, '')
    .replace(/`/g, 'ˋ').replace(/@/g, '@\u200b')
    .slice(0, 18000);
}

function render({ research, summary, outcome, stale, revisionUnverified = false, head, base, runURL, secrets }) {
  research = classifyResearch(research);
  const researchComplete = outcome === 'success' && research.status === 'success' && Boolean(research.text?.trim());
  const complete = !stale && !revisionUnverified && researchComplete;
  const heading = stale ? 'STALE — a new manual review is required.'
    : revisionUnverified ? 'INCOMPLETE — revision could not be reverified after publication. No tests were run.'
    : complete ? 'Completed static review. No tests were run.'
    : 'INCOMPLETE — no completed review verdict. No tests were run.';
  const reason = research.status === 'over_budget'
    ? `Research reported ${research.turns} turns, exceeding the ${RESEARCH_MAX_TURNS}-turn limit. Over-budget output is not a completed verdict.`
    : research.status === 'turn_limit' ? `Research reached its ${RESEARCH_MAX_TURNS}-turn limit.`
    : !complete ? 'Review setup, execution, or result validation did not complete.' : '';
  const text = stale ? '' : researchComplete || research.status === 'over_budget' ? research.text : summary.status === 'success' ? summary.text : '';
  const body = [
    'Claude review (manual, requested by jonesrussell)', '', heading, reason,
    `Head: \`${head || 'unavailable'}\``, `Base: \`${base || 'unavailable'}\``,
    `Workflow: ${runURL}`, '',
    `Denied tool calls: ${integer(research.denied_count)} (${(research.denied_tools || []).join(', ') || 'none recorded'}).`,
    'Sanitized status/count/tool-name diagnostics are attached to the workflow when available.',
    text?.trim() ? '\n' + (complete ? 'Review summary:' : 'Partial evidence only:') + '\n```text\n' + safeText(text, secrets) + '\n```' : '',
    '', 'Advisory feedback only; this does not approve or merge the PR.',
  ].filter(line => line !== undefined).join('\n');
  return { body, complete };
}

async function publish({ github, context, core, env = process.env }) {
  const research = readResearch(env);
  const summary = readExecution(summaryPath(env));
  const runURL = `${context.serverUrl}/${context.repo.owner}/${context.repo.repo}/actions/runs/${context.runId}`;
  const getPR = () => github.rest.pulls.get({ ...context.repo, pull_number: context.issue.number });
  const current = (await getPR()).data;
  const changed = pr => pr.state !== 'open' || pr.head.sha !== env.REVIEW_HEAD || pr.base.sha !== env.REVIEW_BASE;
  const input = { research, summary, outcome: env.RESEARCH_OUTCOME, stale: changed(current),
    head: env.REVIEW_HEAD, base: env.REVIEW_BASE, runURL,
    secrets: [env.REVIEW_OAUTH_TOKEN, env.GITHUB_TOKEN] };
  let report = render(input);
  const commentID = Number(env.REVIEW_COMMENT_ID);
  let posted;
  if (Number.isSafeInteger(commentID) && commentID > 0) {
    posted = await github.rest.issues.updateComment({ ...context.repo, comment_id: commentID, body: report.body });
  } else {
    posted = await github.rest.issues.createComment({ ...context.repo, issue_number: context.issue.number, body: report.body });
  }
  // Check once again after publishing to catch movement during the API call.
  if (!input.stale) {
    let after;
    try { after = (await getPR()).data; } catch {
      // A failed recheck is not evidence of a changed revision. Keep the
      // already-published evidence, but invalidate its completed verdict.
      report = render({ ...input, revisionUnverified: true });
    }
    if (after && changed(after)) report = render({ ...input, stale: true });
    if (!after || changed(after)) {
      await github.rest.issues.updateComment({ ...context.repo, comment_id: posted.data.id, body: report.body });
    }
  }
  if (!report.complete) core.setFailed('Manual review incomplete or stale; see the PR status comment.');
}

module.exports = { RESEARCH_MAX_TURNS, parseExecution, readExecution, diagnostics, prepare, summarize, render, publish, safeText };
if (require.main === module) {
  try {
    if (process.argv[2] === 'prepare') prepare(process.env);
    else if (process.argv[2] === 'summarize') summarize(process.env);
    else throw new Error('Unknown operation');
  } catch {
    console.error('Review reporting helper failed; the final publisher will report incomplete execution.');
    process.exitCode = 1;
  }
}
