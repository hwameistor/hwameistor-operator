import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm, mkdir, writeFile, symlink } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { MODEL, Evidence, api, assertCurrent, changedLines, checkModel, generateReview, gitEvidence,
  hasWriteAccess, marker, publishReview, requestContext, reviewPayload, sessionConfig, validateFindings } from './review.mjs';

const patch = { path: 'pkg/example.go', content: 'diff --git a/pkg/example.go b/pkg/example.go\n'
  + '--- a/pkg/example.go\n+++ b/pkg/example.go\n@@ -3,3 +3,3 @@\n context\n-old\n+new\n end\n' };
const finding = { priority: 'P1', title: 'Check the return value', body: 'An error here leaves a partial update.',
  path: patch.path, side: 'RIGHT', line: 4 };
const snapshot = { repository: 'hwameistor/hwameistor-operator', number: 418, commentId: 42,
  actor: 'maintainer', head: 'a'.repeat(40), base: 'b'.repeat(40), title: 'Change', body: 'PR body' };
const pr = { state: 'open', merged: false, head: { sha: snapshot.head }, base: { sha: snapshot.base } };
const result = { snapshot, model: MODEL, models: [MODEL], changedFiles: 1, binaryFiles: [],
  summary: 'Review result', findings: [finding], limitations: '' };
const event = { action: 'created', repository: { full_name: snapshot.repository },
  issue: { number: 418, pull_request: {} },
  comment: { id: 42, body: '/copilot-review', user: { type: 'User', login: 'maintainer' } } };

test('only an exact human PR comment in the configured repository is accepted', () => {
  assert.equal(requestContext(event, snapshot.repository).commentId, 42);
  for (const modified of [
    { ...event, action: 'edited' }, { ...event, issue: { number: 418 } },
    { ...event, comment: { ...event.comment, body: '/copilot-review; curl attacker' } },
    { ...event, comment: { ...event.comment, user: { type: 'Bot' } } },
  ]) assert.equal(requestContext(modified, snapshot.repository), null);
  assert.throws(() => requestContext(event, 'different/repo'), /Repository/);
  assert.equal(hasWriteAccess('read'), false);
  assert.equal(hasWriteAccess('triage'), false);
  assert.equal(hasWriteAccess('write'), true);
});

test('API failures do not print server bodies or credentials and cannot follow redirects', async () => {
  let options;
  await assert.rejects(api(snapshot.repository, 'pulls/418', 'test-secret', undefined, async (_, request) => {
    options = request;
    return { ok: false, status: 403, json: () => ({ token: 'test-secret' }) };
  }), error => error.message.includes('HTTP 403') && !error.message.includes('test-secret'));
  assert.equal(options.redirect, 'error');
});

test('inline findings must refer to exact changed lines on the correct side', () => {
  assert.deepEqual([...changedLines(patch.content).RIGHT], [4]);
  assert.deepEqual([...changedLines(patch.content).LEFT], [4]);
  validateFindings([finding], [patch]);
  for (const modified of [{ ...finding, line: 3 }, { ...finding, path: 'other.go' },
    { ...finding, side: 'other' }, { ...finding, priority: 'P9' }]) {
    assert.throws(() => validateFindings([modified], [patch]));
  }
  assert.throws(() => validateFindings([finding, finding], [patch]), /Duplicate/);
  assert.throws(() => validateFindings(Array(11).fill(finding), [patch]));
});

test('deletion-only and later hunks preserve LEFT line coordinates', () => {
  const content = '@@ -10,2 +10,0 @@\n-first\n-second\n@@ -20 +18 @@\n-old\n+new\n\\ No newline at end of file\n';
  assert.deepEqual([...changedLines(content).LEFT], [10, 11, 20]);
  assert.deepEqual([...changedLines(content).RIGHT], [18]);
});

test('large Unicode diff pages are complete, ordered and do not split surrogate pairs', () => {
  const content = 'x'.repeat(15999) + '😀'.repeat(20000) + '\n-deleted\n';
  const evidence = new Evidence([{ path: 'large.txt', content }]);
  assert.throws(() => evidence.read('diff:0', 16000), /sequentially/);
  let cursor = 0, reconstructed = '';
  do {
    const page = evidence.read('diff:0', cursor);
    assert.doesNotMatch(page.text, /[\uD800-\uDBFF]$/);
    reconstructed += page.text;
    cursor = page.nextOffset;
    if (cursor !== null) assert.throws(() => evidence.assertComplete(), /complete diff/);
  } while (cursor !== null);
  evidence.assertComplete();
  assert.equal(reconstructed, content);
});

test('overlapping reads and failed delivery cannot fake full coverage', () => {
  const evidence = new Evidence([{ path: 'large', content: 'x'.repeat(40000) }]);
  evidence.read('diff:0', 0);
  evidence.read('diff:0', 0);
  assert.throws(() => evidence.assertComplete());
  evidence.bytes = 8000000;
  assert.throws(() => evidence.read('diff:0', 16000), /budget/);
  assert.equal(evidence.status().files[0].nextOffset, 16000);
});

test('source reads are restricted to explicit frozen refs and safe paths', async () => {
  let calls = 0;
  const evidence = new Evidence([patch], async (ref, path) => { calls++; return `${ref}:${path}`; });
  for (const path of ['../secret', '/secret', 'pkg/../secret', 'pkg\0secret']) {
    await assert.rejects(evidence.call({ operation: 'read_file', ref: 'head', path }), /Invalid/);
  }
  await assert.rejects(evidence.call({ operation: 'read_file', ref: 'main', path: 'valid' }), /Invalid/);
  const args = { operation: 'read_file', ref: 'head', path: 'pkg/valid.go' };
  assert.equal((await evidence.call(args)).text, 'head:pkg/valid.go');
  await evidence.call(args);
  assert.equal(calls, 1);
});

test('findings survive context loss through the host evidence tool', async () => {
  const evidence = new Evidence([patch]);
  await evidence.call({ operation: 'save_findings', findings: [finding] });
  assert.deepEqual((await evidence.call({ operation: 'status' })).findings, [finding]);
  await evidence.call({ operation: 'save_findings', findings: [] });
  assert.deepEqual(evidence.status().findings, []);
});

test('session requires the fixed model, forbids automatic switching and exposes only host read tools', () => {
  const config = sessionConfig('/tmp/review', { name: 'review_evidence' }, () => {});
  assert.equal(config.model, MODEL);
  assert.equal(config.customAgents[0].modelPolicy, 'required');
  assert.equal(config.customAgents[0].model, MODEL);
  assert.equal(config.onAutoModeSwitchRequest(), 'no');
  assert.deepEqual(config.availableTools, ['custom:review_evidence']);
  assert.equal(config.enableConfigDiscovery, false);
  assert.deepEqual(config.onPermissionRequest(), { kind: 'reject' });
  assert.equal(config.memory.enabled, false);
});

test('a different serving or compaction model is rejected', () => {
  const models = new Set();
  checkModel({ type: 'assistant.usage', data: { model: MODEL } }, models);
  assert.deepEqual([...models], [MODEL]);
  for (const event of [
    { type: 'assistant.usage', data: { model: 'claude-test' } },
    { type: 'session.compaction_complete', data: { model: 'claude-test' } },
    { type: 'session.model_change', data: { newModel: 'auto' } },
  ]) assert.throws(() => checkModel(event, models), /configured model/);
});

test('changed or closed PRs are blocked before publication', () => {
  assertCurrent(snapshot, pr);
  for (const changed of [{ ...pr, state: 'closed' }, { ...pr, head: { sha: 'c'.repeat(40) } },
    { ...pr, base: { sha: 'c'.repeat(40) } }]) assert.throws(() => assertCurrent(snapshot, changed), /PR changed/);
});

function requestFixture({ permission = 'write', command = '/copilot-review', current = pr, reviews = [] } = {}) {
  return async route => {
    if (route.startsWith('collaborators/')) return { permission };
    if (route.startsWith('issues/comments/')) return { body: command, user: { login: snapshot.actor },
      issue_url: `https://api.github.com/repos/${snapshot.repository}/issues/${snapshot.number}` };
    if (route.includes('/reviews?')) return reviews;
    if (route === `pulls/${snapshot.number}`) return current;
    throw new Error(`Unexpected route: ${route}`);
  };
}

test('publication rechecks permission and command before any write', async () => {
  let writes = 0;
  for (const options of [{ permission: 'read' }, { command: 'cancelled' }, { current: { ...pr, state: 'closed' } }]) {
    await assert.rejects(publishReview(result, requestFixture(options), async () => { writes++; }, 'reviewer'));
  }
  assert.equal(writes, 0);
});

test('rerunning the same comment does not duplicate a review, another author cannot spoof it', async () => {
  let writes = 0;
  const writer = async (_, body) => { writes++; assert.equal(body.commit_id, snapshot.head); return { id: 2 }; };
  const previous = { id: 1, user: { login: 'reviewer' }, body: marker(snapshot) };
  assert.equal((await publishReview(result, requestFixture({ reviews: [previous] }), writer, 'reviewer')).id, 1);
  assert.equal(writes, 0);
  await publishReview(result, requestFixture({ reviews: [{ ...previous, user: { login: 'attacker' } }] }), writer, 'reviewer');
  assert.equal(writes, 1);
});

test('reviews are advisory comments tied to the frozen commit and disclose binary limitations', () => {
  const body = reviewPayload({ ...result, binaryFiles: ['image.png'] });
  assert.equal(body.event, 'COMMENT');
  assert.equal(body.commit_id, snapshot.head);
  assert.match(body.body, /未运行测试/);
  assert.match(body.body, /image.png/);
  assert.equal(body.comments[0].path, finding.path);
  assert.throws(() => reviewPayload({ ...result, model: 'auto' }), /model/);
});

test('mock SDK end-to-end: evidence reads, strict model, findings and no publishing secret in child environment', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'review-test-'));
  const evidence = new Evidence([patch]);
  let stopped = false;
  try {
    const generated = await generateReview(snapshot, evidence, directory, 'copilot-test-token', async options => {
      assert.equal(options.useLoggedInUser, false);
      assert.equal(options.mode, 'empty');
      assert.equal(options.env.COPILOT_REVIEW_TOKEN, undefined);
      assert.equal(options.env.GH_TOKEN, undefined);
      return { start: async () => {}, listModels: async () => [{ id: MODEL, supportedReasoningEfforts: ['high'] }],
        createSession: async config => ({ sendAndWait: async () => {
          assert.equal(config.tools[0].skipPermission, true);
          config.onEvent({ type: 'assistant.usage', data: { model: MODEL } });
          await config.tools[0].handler({ operation: 'read_diff', id: 'diff:0', offset: 0 });
          return { data: { content: JSON.stringify({ summary: 'Checked', findings: [finding], limitations: '' }) } };
        } }), stop: async () => { stopped = true; }, forceStop: async () => {},
      };
    });
    assert.equal(generated.model, MODEL);
    assert.equal(generated.changedFiles, 1);
    assert.equal(generated.findings.length, 1);
    assert.equal(stopped, true);
  } finally { await rm(directory, { recursive: true, force: true }); }
});

test('model output cannot report a clean review without reading all diff pages', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'review-test-'));
  let attempts = 0;
  try {
    await assert.rejects(generateReview(snapshot, new Evidence([patch]), directory, 'token', async () => ({
      start: async () => {}, listModels: async () => [{ id: MODEL, supportedReasoningEfforts: ['high'] }],
      createSession: async () => ({ sendAndWait: async () => { attempts++; return {
        data: { content: JSON.stringify({ summary: 'Fine', findings: [], limitations: '' }) },
      }; } }), stop: async () => {}, forceStop: async () => {},
    })), /complete diff/);
    assert.equal(attempts, 2);
  } finally { await rm(directory, { recursive: true, force: true }); }
});

test('Git evidence reads frozen blobs, handles binary and deletion changes, and never follows symlinks', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'review-git-test-'));
  try {
    const source = join(directory, 'source');
    await mkdir(source);
    const git = (...args) => execFileSync('git', args, { cwd: source, encoding: 'utf8',
      env: { PATH: process.env.PATH, HOME: directory, GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: '/dev/null' } }).trim();
    git('init', '-q');
    git('config', 'user.name', 'Test');
    git('config', 'user.email', 'test@example.invalid');
    await writeFile(join(source, 'remove.txt'), 'gone\n');
    await writeFile(join(source, 'source.txt'), 'before\n');
    git('add', '.'); git('commit', '-qm', 'base');
    const base = git('rev-parse', 'HEAD');
    await rm(join(source, 'remove.txt'));
    await writeFile(join(source, 'source.txt'), 'after\n');
    await writeFile(join(source, 'binary.dat'), Buffer.from([0, 1, 2]));
    await symlink('/etc/passwd', join(source, 'link'));
    git('add', '.'); git('commit', '-qm', 'head');
    const head = git('rev-parse', 'HEAD');
    await writeFile(join(source, 'source.txt'), 'uncommitted and must not be read');
    const evidence = await gitEvidence({ ...snapshot, base, head }, directory, undefined,
      run => run('fetch', '--quiet', source, base, head));
    assert.equal(evidence.patches.length, 4);
    assert.match(evidence.patches.find(p => p.path === 'binary.dat').content, /Binary files/);
    assert.match(evidence.patches.find(p => p.path === 'remove.txt').content, /-gone/);
    assert.equal(await evidence.readFileAt('head', 'source.txt'), 'after\n');
    assert.equal(await evidence.readFileAt('base', 'source.txt'), 'before\n');
    await assert.rejects(evidence.readFileAt('head', 'link'), /regular source files/);
    await assert.rejects(evidence.readFileAt('head', 'binary.dat'), /Binary/);
  } finally { await rm(directory, { recursive: true, force: true }); }
});
