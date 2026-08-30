const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const reporter = require('./manual-review.cjs');

const session = '47942c41-599a-411e-aa9b-f1d1a4f77677';
const success = text => ({ type: 'result', subtype: 'success', is_error: false, result: text, num_turns: 9 });
const exhausted = { type: 'result', subtype: 'error_max_turns', is_error: true, session_id: session,
  num_turns: 17, duration_ms: 82745, permission_denials: [
    { tool_name: 'Bash', tool_input: { command: 'SECRET_CANARY' } },
    { tool_name: 'Read', tool_input: { file_path: 'SECRET_CANARY' } },
    { tool_name: 'SECRET_CANARY', tool_input: 'SECRET_CANARY' },
  ], errors: ['SECRET_CANARY'], result: 'SECRET_CANARY' };

function fixture(t, research) {
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'manual-review-test-'));
  t.after(() => fs.rmSync(temp, { recursive: true, force: true }));
  const env = { RUNNER_TEMP: temp, GITHUB_OUTPUT: path.join(temp, 'output'), HOME: temp,
    GITHUB_TOKEN: 'github-secret-canary', GH_TOKEN: 'other-github-secret',
    CLAUDE_CODE_OAUTH_TOKEN: 'oauth-secret-canary', REVIEW_OAUTH_TOKEN: 'oauth-secret-canary',
    REVIEW_HEAD: 'head', REVIEW_BASE: 'base', REVIEW_COMMENT_ID: '42', RESEARCH_OUTCOME: 'success' };
  if (research) fs.writeFileSync(path.join(temp, 'claude-execution-output.json'), JSON.stringify(research));
  reporter.prepare(env);
  return env;
}

function apiMock({ stale = false, moved = false } = {}) {
  const calls = [], failures = [];
  let reads = 0;
  const github = { rest: {
    pulls: { get: async () => ({ data: { state: 'open', head: { sha: stale || (moved && reads++ > 0) ? 'changed' : 'head' }, base: { sha: 'base' } } }) },
    issues: {
      updateComment: async args => { calls.push(args); return { data: { id: 42 } }; },
      createComment: async args => { calls.push(args); return { data: { id: 42 } }; },
    },
  } };
  return { github, calls, failures, core: { setFailed: message => failures.push(message) },
    context: { repo: { owner: 'goformx', repo: 'goformx' }, issue: { number: 172 }, serverUrl: 'https://github.com', runId: 123 } };
}

test('SDK array and native CLI object formats; malformed/absent results fail closed', () => {
  assert.equal(reporter.parseExecution(JSON.stringify([success('ok')])).text, 'ok');
  assert.equal(reporter.parseExecution(JSON.stringify(success('ok'))).status, 'success');
  assert.equal(reporter.parseExecution('{bad').status, 'invalid');
  assert.equal(reporter.parseExecution('null').status, 'missing');
  assert.equal(reporter.parseExecution('[]').status, 'missing');
  assert.equal(reporter.parseExecution(JSON.stringify({ ...success('ok'), is_error: true })).status, 'failed');
});

test('actual exhaustion shape retains safe denial names/counts, not commands/errors/session/text', t => {
  const env = fixture(t, [{ type: 'assistant', message: 'SECRET_CANARY' }, exhausted]);
  const diagnostic = fs.readFileSync(path.join(env.RUNNER_TEMP, 'manual-review-diagnostics.json'), 'utf8');
  assert.doesNotMatch(diagnostic, /SECRET_CANARY|47942c41|tool_input|errors/);
  const parsed = JSON.parse(diagnostic).research;
  assert.equal(parsed.status, 'turn_limit');
  assert.equal(parsed.denied_count, 3);
  assert.deepEqual(parsed.denied_tools, ['Bash', 'Read', 'other']);
  assert.match(fs.readFileSync(env.GITHUB_OUTPUT, 'utf8'), /summarize=true/);
});

test('one bounded resume has no tools/GitHub tokens and never logs its private output', t => {
  const env = fixture(t, [exhausted]);
  let count = 0;
  reporter.summarize(env, (exe, args, options) => {
    count++;
    assert.equal(exe, path.join(env.HOME, '.local/bin/claude'));
    assert.equal(args[args.indexOf('--tools') + 1], '');
    assert.equal(args[args.indexOf('--resume') + 1], session);
    assert.equal(args[args.indexOf('--max-turns') + 1], '2');
    assert.equal(options.timeout, 120000);
    assert.equal(options.env.GITHUB_TOKEN, undefined);
    assert.equal(options.env.GH_TOKEN, undefined);
    return { status: 0, stdout: JSON.stringify(success('Partial evidence.')), stderr: 'SECRET_CANARY' };
  });
  assert.equal(count, 1);
  assert.equal(reporter.readExecution(path.join(env.RUNNER_TEMP, 'manual-review-private/summary.json')).text, 'Partial evidence.');
  assert.doesNotMatch(fs.readFileSync(path.join(env.RUNNER_TEMP, 'manual-review-diagnostics.json'), 'utf8'), /Partial evidence|SECRET_CANARY/);
});

test('timeout cannot masquerade as success and malformed session cannot invoke resume', t => {
  const env = fixture(t, [exhausted]);
  reporter.summarize(env, () => ({ status: null, error: new Error('SECRET_CANARY'), stdout: JSON.stringify(success('false verdict')) }));
  assert.equal(reporter.readExecution(path.join(env.RUNNER_TEMP, 'manual-review-private/summary.json')).status, 'failed');
  const other = fixture(t, [{ ...exhausted, session_id: '--malicious' }]);
  reporter.summarize(other, () => assert.fail('must not run'));
  assert.match(fs.readFileSync(other.GITHUB_OUTPUT, 'utf8'), /summarize=false/);
});

test('successful research does not request a second provider call', t => {
  const env = fixture(t, [success('No findings established.')]);
  reporter.summarize(env, () => assert.fail('must not run'));
  assert.match(fs.readFileSync(env.GITHUB_OUTPUT, 'utf8'), /summarize=false/);
});

test('successful report updates the existing status comment once', async t => {
  const env = fixture(t, [success('No findings established.')]);
  const api = apiMock();
  await reporter.publish({ ...api, env });
  assert.equal(api.calls.length, 1);
  assert.equal(api.calls[0].comment_id, 42);
  assert.match(api.calls[0].body, /Completed static review/);
  assert.match(api.calls[0].body, /No tests were run/);
  assert.equal(api.failures.length, 0);
});

test('exhaustion publishes partial evidence but remains failed/incomplete', async t => {
  const env = fixture(t, [exhausted]);
  env.RESEARCH_OUTCOME = 'failure';
  reporter.summarize(env, () => ({ status: 0, stdout: JSON.stringify(success('One possible concern. Coverage incomplete.')) }));
  const api = apiMock();
  await reporter.publish({ ...api, env });
  assert.match(api.calls[0].body, /INCOMPLETE/);
  assert.match(api.calls[0].body, /16-turn limit/);
  assert.match(api.calls[0].body, /Partial evidence only/);
  assert.equal(api.failures.length, 1);
});

for (const [name, record, outcome] of [
  ['missing output', null, 'failure'],
  ['provider failure', { type: 'result', subtype: 'error', is_error: true }, 'failure'],
  ['empty success', success(''), 'success'],
  ['action failed after model success', success('not a completed action'), 'failure'],
]) {
  test(`${name} still produces a failed status comment`, async t => {
    const env = fixture(t, record);
    env.RESEARCH_OUTCOME = outcome;
    env.REVIEW_COMMENT_ID = '';
    const api = apiMock();
    await reporter.publish({ ...api, env });
    assert.equal(api.calls[0].issue_number, 172);
    assert.match(api.calls[0].body, /INCOMPLETE/);
    assert.equal(api.failures.length, 1);
  });
}

test('stale head never publishes the model verdict', async t => {
  const env = fixture(t, [success('stale-verdict-canary')]);
  const api = apiMock({ stale: true });
  await reporter.publish({ ...api, env });
  assert.match(api.calls[0].body, /STALE/);
  assert.doesNotMatch(api.calls[0].body, /stale-verdict-canary/);
  assert.equal(api.failures.length, 1);
});

test('revision movement during publication invalidates the same comment', async t => {
  const env = fixture(t, [success('review text')]);
  const api = apiMock({ moved: true });
  await reporter.publish({ ...api, env });
  assert.equal(api.calls.length, 2);
  assert.equal(api.calls[0].comment_id, api.calls[1].comment_id);
  assert.match(api.calls[1].body, /STALE/);
  assert.equal(api.failures.length, 1);
});

test('post-publication API failure preserves evidence as unverified, never as a completed verdict', async t => {
  const env = fixture(t, [success('Evidence that must not be discarded.')]);
  const api = apiMock();
  let reads = 0;
  api.github.rest.pulls.get = async () => {
    if (reads++ > 0) throw new Error('SECRET_API_ERROR');
    return { data: { state: 'open', head: { sha: 'head' }, base: { sha: 'base' } } };
  };
  const run = new AsyncFunction('require', 'github', 'context', 'core', 'process', inlineScript('Publish final review or failure status'));
  await run(() => ({ publish: args => reporter.publish({ ...args, env }) }), api.github, api.context, api.core,
    { env: { ...env, GITHUB_WORKSPACE: '/trusted' } });
  assert.equal(api.calls.length, 2);
  assert.equal(api.calls[0].comment_id, api.calls[1].comment_id);
  assert.match(api.calls[1].body, /INCOMPLETE.*revision could not be reverified/);
  assert.match(api.calls[1].body, /Evidence that must not be discarded/);
  assert.doesNotMatch(api.calls[1].body, /Completed static review|SECRET_API_ERROR/);
  assert.equal(api.failures.length, 1);
});

test('review body redacts credentials and cannot escape its inert code fence', () => {
  const result = reporter.safeText('literal-secret ghp_abcdef sk-ant-oat01-secret Bearer abc ```\n![x](url) @claude review', ['literal-secret']);
  assert.doesNotMatch(result, /literal-secret|ghp_abcdef|sk-ant-oat01-secret|Bearer abc|```|@claude/);
  assert.equal(reporter.safeText('x'.repeat(20000)).length, 18000);
});

const workflow = fs.readFileSync(path.join(__dirname, '../workflows/manual-claude-review.yml'), 'utf8');
function inlineScript(stepName) {
  const step = workflow.split(`      - name: ${stepName}\n`)[1].split('\n      - name:')[0];
  return step.split('          script: |\n')[1].split('\n').map(line => line.replace(/^            /, '')).join('\n');
}
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;

test('checkout/helper failure uses the inline deterministic publisher', async () => {
  const api = apiMock();
  const run = new AsyncFunction('require', 'github', 'context', 'core', 'process', inlineScript('Publish final review or failure status'));
  await run(() => { throw new Error('missing checkout'); }, api.github, api.context, api.core,
    { env: { GITHUB_WORKSPACE: '/absent', REVIEW_COMMENT_ID: '42' } });
  assert.equal(api.calls.length, 1);
  assert.match(api.calls[0].body, /INCOMPLETE/);
  assert.equal(api.failures.length, 1);
});

test('missing subscription posts status before failing preflight', async () => {
  const api = apiMock();
  api.core.setOutput = () => {};
  const run = new AsyncFunction('github', 'context', 'core', 'process', inlineScript('Validate request and subscription setup'));
  await run(api.github, api.context, api.core, { env: { HAS_SUBSCRIPTION_TOKEN: 'false' } });
  assert.equal(api.calls.length, 1);
  assert.match(api.calls[0].body, /no verdict yet/);
  assert.equal(api.failures.length, 1);
});

test('workflow retains manual trust gate and publishes outside the model', () => {
  assert.match(workflow, /github\.event\.comment\.body == '@claude review'/);
  assert.match(workflow, /github\.actor == 'jonesrussell'/);
  assert.match(workflow, /ref: \$\{\{ github.sha \}\}/);
  assert.doesNotMatch(workflow, /Bash\(gh pr comment/);
  assert.match(workflow, /name: Publish final review or failure status\s+if: always\(\)/);
  assert.match(workflow, /path: \$\{\{ runner.temp \}\}\/manual-review-diagnostics.json/);
  assert.doesNotMatch(workflow, /show_full_output: true|path:.*execution-output|pull_request_target/);
});
