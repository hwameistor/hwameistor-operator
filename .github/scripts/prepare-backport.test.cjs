'use strict';

const assert = require('node:assert/strict');
const { test } = require('node:test');
const { targetsForLabels, isBackportOf, prepareBackport } = require('./prepare-backport.cjs');

const repository = 'hwameistor/hwameistor-operator';
const target = 'release/v1.1';
const labels = (...names) => names.map(name => ({ name }));
const manualBody = 'Backport of the migration fix from #393 to `release/v1.1`, addressing #392.';
function candidate(overrides = {}) {
  return {
    number: 394,
    html_url: 'https://github.com/hwameistor/hwameistor-operator/pull/394',
    state: 'open', merged_at: null, body: manualBody,
    base: { ref: target },
    head: { ref: 'manual/backport-393-v1.1', repo: { full_name: 'contributor/hwameistor-operator' } },
    ...overrides,
  };
}

function harness({ pull = {}, candidates = [], listError } = {}) {
  const calls = [];
  const outputs = {};
  const summary = [];
  const github = {
    rest: { pulls: {
      get: async args => {
        calls.push(['get', args]);
        return { data: {
          merged: true, base: { ref: 'main', repo: { full_name: repository } },
          labels: labels('cherry-pick-v1.1'), ...pull,
        } };
      },
      list: () => {},
    } },
    paginate: async (_, args) => {
      calls.push(['list', args]);
      if (listError) throw listError;
      return candidates;
    },
  };
  const core = {
    info: () => {},
    setOutput: (key, value) => { outputs[key] = value; },
    summary: {
      addHeading(value) { summary.push(value); return this; },
      addRaw(value) { summary.push(value); return this; },
      async write() {},
    },
  };
  return {
    calls, outputs, summary,
    run: (pullNumber = '393') => prepareBackport({
      github, core, pullNumber,
      context: {
        repo: { owner: 'hwameistor', repo: 'hwameistor-operator' },
        payload: { pull_request: { labels: labels('cherry-pick-v1') } },
      },
    }),
  };
}

test('routes supported labels and deduplicates the legacy v1.0 alias', () => {
  assert.deepEqual(targetsForLabels(labels('cherry-pick-v1.1', 'cherry-pick-v1', 'cherry-pick-v1.0')),
    [target, 'release/v1.0.0']);
  assert.deepEqual(targetsForLabels(labels('cherry-pick', 'cherry-pick-v1.2', 'backport main')), []);
});

test('recognizes manual backports without mistaking other references for the source', () => {
  assert.equal(isBackportOf(candidate(), 393, target, repository), true);
  assert.equal(isBackportOf(candidate(), 392, target, repository), false);
  assert.equal(isBackportOf(candidate({ body: 'Backport of #3930' }), 393, target, repository), false);
  assert.equal(isBackportOf(candidate({ body: 'Fixes #393' }), 393, target, repository), false);
});

test('recognizes the generated branch only in the current repository', () => {
  const head = { ref: 'backport-393-to-release/v1.1', repo: { full_name: repository } };
  assert.equal(isBackportOf(candidate({ body: null, head }), 393, target, repository), true);
  head.repo.full_name = 'contributor/hwameistor-operator';
  assert.equal(isBackportOf(candidate({ body: null, head }), 393, target, repository), false);
});

test('skips open and merged PRs but permits closed unmerged PRs and other targets', () => {
  assert.equal(isBackportOf(candidate({ state: 'closed', merged_at: '2026-01-01' }), 393, target, repository), true);
  assert.equal(isBackportOf(candidate({ state: 'closed' }), 393, target, repository), false);
  assert.equal(isBackportOf(candidate(), 393, 'release/v1.0.0', repository), false);
});

test('reads current API labels instead of stale event labels', async () => {
  const h = harness();
  await h.run();
  assert.deepEqual(h.outputs, { pull_number: '393', targets: target });
  assert.deepEqual(h.calls[1][1], {
    owner: 'hwameistor', repo: 'hwameistor-operator', base: target, state: 'all', per_page: 100,
  });
});

for (const [selected, expected] of [
  [['cherry-pick-v1.0'], 'release/v1.0.0'],
  [['cherry-pick-v1.0', 'cherry-pick-v1.1'], 'release/v1.0.0 release/v1.1'],
  [['cherry-pick-v1', 'cherry-pick-v1.0'], 'release/v1.0.0'],
]) {
  test(`selects pending branches for ${selected.join(', ')}`, async () => {
    const h = harness({ pull: { labels: labels(...selected) } });
    await h.run();
    assert.equal(h.outputs.targets, expected);
    assert.equal(h.calls.filter(([method]) => method === 'list').length, expected.split(' ').length);
  });
}

test('skips only targets with an existing open or merged backport and links to it', async () => {
  const h = harness({
    pull: { labels: labels('cherry-pick-v1', 'cherry-pick-v1.0', 'cherry-pick-v1.1') },
    candidates: [candidate()],
  });
  await h.run();
  assert.equal(h.outputs.targets, 'release/v1.0.0');
  assert.equal(h.calls.filter(([method]) => method === 'list').length, 2);
  assert.match(h.summary.join('\n'), /\[#394\]\(https:\/\/github.com\/hwameistor\/hwameistor-operator\/pull\/394\)/);
});

test('does no backport work if labels were removed while the event was queued', async () => {
  const h = harness({ pull: { labels: [] } });
  await h.run();
  assert.equal(h.outputs.targets, '');
  assert.equal(h.calls.length, 1);
});

test('produces no targets when all backports already exist', async () => {
  const h = harness({ candidates: [candidate()] });
  await h.run();
  assert.equal(h.outputs.targets, '');
});

test('allows retrying a closed unmerged backport', async () => {
  const h = harness({ candidates: [candidate({ state: 'closed' })] });
  await h.run();
  assert.equal(h.outputs.targets, target);
});

test('rejects unmerged, non-main and foreign-repository source PRs before target discovery', async () => {
  for (const pull of [
    { merged: false },
    { base: { ref: target, repo: { full_name: repository } } },
    { base: { ref: 'main', repo: { full_name: 'other/repo' } } },
  ]) {
    const h = harness({ pull });
    await assert.rejects(h.run(), /Only merged PRs targeting main/);
    assert.equal(h.calls.length, 1);
    assert.deepEqual(h.outputs, {});
  }
});

test('rejects malformed or unsafe PR numbers before making API calls', async () => {
  for (const number of ['', '0', '-1', '01', '1.5', '1e3', ' 393 ', '393; echo nope', '9007199254740992']) {
    const h = harness();
    await assert.rejects(h.run(number), /positive integer/);
    assert.equal(h.calls.length, 0);
  }
});

test('fails closed if duplicate discovery fails', async () => {
  const h = harness({ listError: new Error('API unavailable') });
  await assert.rejects(h.run(), /API unavailable/);
  assert.deepEqual(h.outputs, {});
});

test('recognizes source references when the source fork has been deleted', () => {
  const pull = candidate({ head: { ref: 'deleted-branch', repo: null } });
  assert.equal(isBackportOf(pull, 393, target, repository), true);
  assert.equal(isBackportOf({ ...pull, body: null }, 393, target, repository), false);
});
