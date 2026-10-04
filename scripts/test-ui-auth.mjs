import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createContext, runInContext } from 'node:vm';
import test from 'node:test';

const source = readFileSync(new URL('../internal/actelyohub/ui/src/js/04-api.js', import.meta.url), 'utf8');
test('late unauthorized responses retry the newly accepted key without another prompt', async () => {
  let prompts = 0;
  const requests = [];
  const context = createContext({
    localStorage: { getItem: () => '', setItem: () => {} },
    location: { pathname: '/' }, AbortController, setTimeout, clearTimeout,
    t: value => value,
    askPrompt: async () => { prompts++; return 'replacement'; },
    fetch: async (_url, options) => {
      requests.push(options.headers.Authorization || '');
      if (requests.length === 1) {
        runInContext("TOKEN = 'newly-accepted-key'", context);
        return { status: 401 };
      }
      return { status: 200 };
    },
  });
  runInContext(source, context);
  assert.equal((await runInContext("jfetch('/api/status')", context)).status, 200);
  assert.equal(prompts, 0);
  assert.deepEqual(requests, ['', 'Bearer newly-accepted-key']);
});
test('a rejected current key still requires a replacement', async () => {
  let prompts = 0;
  let calls = 0;
  const context = createContext({
    localStorage: { getItem: () => 'invalid-key', setItem: () => {} },
    location: { pathname: '/' }, AbortController, setTimeout, clearTimeout,
    t: value => value,
    askPrompt: async () => { prompts++; return 'replacement'; },
    fetch: async (_url, options) => {
      calls++;
      return { status: options.headers.Authorization === 'Bearer replacement' ? 200 : 401 };
    },
  });
  runInContext(source, context);
  assert.equal((await runInContext("jfetch('/api/status')", context)).status, 200);
  assert.equal(prompts, 1);
  assert.equal(calls, 2);
});
