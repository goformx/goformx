const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const workflow = fs.readFileSync(path.join(__dirname, '../workflows/manual-claude-review.yml'), 'utf8');
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;

function script(name) {
  const step = workflow.split('      - name: ' + name + '\n')[1]?.split('\n      - name: ')[0];
  assert.ok(step, 'missing step: ' + name);
  return step.split('          script: |\n')[1].split('\n').map(line => line.replace(/^            /, '')).join('\n');
}

function harness({head = 'head', base = 'base', state = 'open', lookupFails = false, checkout = 'head'} = {}) {
  const failures = [], comments = [], outputs = {};
  return {
    failures, comments, outputs,
    github: {rest: {
      pulls: {get: async () => {
        if (lookupFails) throw new Error('PRIVATE_ERROR');
        return {data: {head: {sha: head}, base: {sha: base}, state}};
      }},
      issues: {createComment: async input => { comments.push(input); }},
    }},
    context: {repo: {owner: 'goformx', repo: 'goformx'}, issue: {number: 172}, runId: 123, serverUrl: 'https://github.com',
      actor: 'jonesrussell', payload: {issue: {pull_request: {}},
        comment: {body: '@claude review', user: {login: 'jonesrussell', type: 'User'}}}},
    core: {setFailed: message => failures.push(message), setOutput: (key, value) => { outputs[key] = value; }},
    require: name => {
      assert.equal(name, 'node:child_process');
      return {execFileSync: (command, args, options) => {
        assert.equal(command, 'git');
        assert.deepEqual(args, ['rev-parse', 'HEAD']);
        assert.deepEqual(options, {encoding: 'utf8'});
        return checkout + '\n';
      }};
    },
  };
}

async function run(name, fixture, env) {
  return new AsyncFunction('github', 'context', 'core', 'process', 'require', script(name))(
    fixture.github, fixture.context, fixture.core, {env}, fixture.require
  );
}

test('trusted setup provides the actual diff without model shell access or truncated evidence', async () => {
  for (const diff of ['diff --git a/file b/file\n+change', '', ' ', {}, 'x'.repeat(1048577), '\u00e9'.repeat(524289)]) {
    const h = harness();
    const writes = [];
    h.github.rest.pulls.get = async input => {
      assert.deepEqual(input, {...h.context.repo, pull_number: 172, mediaType: {format: 'diff'}});
      return {data: diff};
    };
    h.require = name => {
      if (name === 'node:path') return path;
      assert.equal(name, 'node:fs');
      return {writeFileSync: (...args) => writes.push(args)};
    };
    const result = run('Prepare review diff', h, {RUNNER_TEMP: '/runner-temp'});
    if (typeof diff === 'string' && diff.trim() && Buffer.byteLength(diff, 'utf8') <= 1048576) {
      await result;
      assert.deepEqual(writes, [[path.join('/runner-temp', 'manual-claude-review.diff'), diff, {mode: 0o600, flag: 'wx'}]]);
      assert.equal(h.outputs.path, writes[0][0]);
    } else {
      await assert.rejects(result, /diff is missing, invalid or exceeds/);
      assert.equal(writes.length, 0);
      assert.deepEqual(h.outputs, {});
    }
  }
  assert.ok(workflow.includes('First Read the actual diff at $' + '{{ steps.diff.outputs.path }}'));
});

test('preflight enforces exact human maintainer PR requests beyond the case-insensitive Actions gate', async () => {
  // Check the outer guard structurally, not using JS equality to simulate Actions.
  for (const clause of ['github.event.issue.pull_request', "github.event.comment.body == '@claude review'",
    "github.event.comment.user.login == 'jonesrussell'", "github.event.comment.user.type == 'User'",
    "github.actor == 'jonesrussell'"]) assert.ok(workflow.includes(clause));
  for (const mutate of [
    g => { g.actor = 'someone-else'; }, g => { g.payload.comment.user.type = 'Bot'; },
    g => { g.payload.comment.user.login = 'someone-else'; }, g => { delete g.payload.issue.pull_request; },
    g => { g.payload.comment.body += ' please'; }, g => { g.payload.comment.body = '@Claude Review'; },
    g => { g.payload.comment.body += '\n'; }, g => { delete g.payload.comment; },
  ]) {
    const h = harness(); mutate(h.context);
    h.github.rest.pulls.get = async () => assert.fail('rejected request must not fetch the PR');
    await run('Validate request and subscription setup', h, {HAS_SUBSCRIPTION_TOKEN: 'true'});
    assert.equal(h.failures.length, 1);
    assert.deepEqual(h.outputs, {});
    assert.equal(h.comments.length, 0);
  }
  assert.match(workflow, /on:\n  issue_comment:\n    types: \[created\]/);
  assert.doesNotMatch(workflow, /^  (?:push|schedule|pull_request|pull_request_target|workflow_dispatch):/m);
});

test('native tracking is subscription-only and cannot use implementation tools or custom recovery', () => {
  for (const contract of [
    'track_progress: true', 'claude_code_oauth_token: $' + '{{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}',
    'contents: read', 'pull-requests: write', '--permission-mode default', '--tools "Read,Glob,Grep"',
    '--disallowedTools "Bash,Edit,Write,NotebookEdit,Agent,Task,mcp__github_ci__*,mcp__github_file_ops__*"',
    'mcp__github_comment__update_claude_comment', '--setting-sources user', '--strict-mcp-config',
    '"disableAllHooks":true', '"Read(./.git)"', '"Read(./.git/**)"',
    '--max-turns 16', 'timeout-minutes: 8', 'show_full_output: false',
    'display_report: false', "steps.claude.outcome == 'success'",
  ]) assert.ok(workflow.includes(contract), 'missing contract: ' + contract);
  assert.doesNotMatch(workflow, /anthropic_api_key|openai-api-key|continue-on-error|upload-artifact|--resume|manual-review\.cjs|--allowedTools|bypassPermissions|--dangerously-skip-permissions/);
  assert.equal(fs.existsSync(path.join(__dirname, 'manual-review.cjs')), false);
});

test('preflight requires an open PR and subscription token without duplicating native comments', async () => {
  for (const [state, token, errors] of [['open', 'true', 0], ['open', 'false', 1], ['closed', 'true', 1]]) {
    const h = harness({state});
    await run('Validate request and subscription setup', h, {HAS_SUBSCRIPTION_TOKEN: token});
    assert.equal(h.failures.length, errors);
    assert.equal(h.comments.length, 0);
    assert.deepEqual(h.outputs, state === 'open' ? {head_sha: 'head', base_sha: 'base'} : {});
  }
});

test('revision guard fails closed on changed, closed, unavailable, or mismatched checkouts', async () => {
  for (const changed of [null, {head: 'new-head'}, {base: 'new-base'}, {state: 'closed'}, {lookupFails: true}, {checkout: 'other'}]) {
    const h = harness(changed ?? {});
    await run('Validate reviewed revision', h, {REVIEW_HEAD: 'head', REVIEW_BASE: 'base'});
    assert.equal(h.failures.length, changed ? 1 : 0);
    assert.equal(h.comments.length, changed ? 1 : 0);
    if (changed) {
      assert.match(h.comments[0].body, /STALE or UNVERIFIED/);
      assert.match(h.comments[0].body, /actions\/runs\/123/);
      assert.doesNotMatch(h.comments[0].body, /PRIVATE_ERROR/);
      assert.equal(h.comments[0].issue_number, 172);
    }
  }
});
