const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const vm = require('node:vm');

const tick = () => new Promise(resolve => setImmediate(resolve));
function element() {
  return { value: '', textContent: '', disabled: false, open: false, children: [],
    append(...items) { this.children.push(...items); },
    replaceChildren(...items) { this.children = items; },
    attributes: {}, setAttribute(name, value) { this.attributes[name] = value; }, focus() { this.focused = true; }, showModal() { this.open = true; },
    close() { this.open = false; this.onclose?.(); } };
}
async function setup() {
  const nodes = new Map();
  const get = selector => { if (!nodes.has(selector)) nodes.set(selector, element()); return nodes.get(selector); };
  get('#create-form').elements = Object.fromEntries(['source', 'target', 'title'].map(key => [key, element()]));
  const pending = [], creates = [];
  const response = (body, ok = true, status = 200) => ({ ok, status, json: async () => body });
  const context = vm.createContext({ document: { querySelector: get, createElement: element }, location: { hash: '' },
    fetch: async (path, options) => {
      if (path === '/api/bootstrap') return response({ token: 'token', repository: 'repo', branches: ['feature', 'main', 'other'] });
      if (path.endsWith('/preview')) return new Promise(resolve => pending.push({ body: JSON.parse(options.body), resolve: (body, ok = true) => resolve(response(body, ok, ok ? 200 : 400)) }));
      if (options.method === 'POST') { creates.push(JSON.parse(options.body)); return response(context.createError || { id: 'created' }, !context.createError, context.createError ? 409 : 200); }
      return response([]);
    } });
  vm.runInContext(readFileSync(__dirname + '/assets/app.js', 'utf8'), context);
  await tick();
  vm.runInContext('openReview = async id => { globalThis.opened = id; };', context);
  return { get, fields: get('#create-form').elements, pending, creates, context };
}
const range = (head = 'a', count = 1) => ({ base: 'b'.repeat(40), head: head.repeat(40), source_oid: head.repeat(40), target_oid: 'b'.repeat(40), commits: Array(count).fill(head), diff: [{}] });
const submit = ui => ui.get('#create-form').onsubmit({ preventDefault() {} });

test('opening previews automatically; title edits preserve range; creation uses displayed range', async () => {
  const ui = await setup();
  ui.get('#new-review').onclick();
  assert.equal(ui.pending.length, 1);
  assert.equal(ui.get('#create-button').disabled, true);
  ui.pending[0].resolve(range()); await tick();
  assert.equal(ui.get('#create-button').disabled, true);
  ui.fields.title.value = 'Review'; ui.fields.title.oninput();
  assert.equal(ui.get('#create-button').disabled, false);
  assert.equal(ui.pending.length, 1);
  await submit(ui);
  assert.equal(ui.creates[0].preview.head, 'a'.repeat(40));
  assert.equal(ui.context.opened, 'created');
});

test('out of order previews and closed dialog responses cannot replace the current range', async () => {
  const ui = await setup();
  ui.fields.title.value = 'Review'; ui.get('#new-review').onclick();
  ui.fields.source.value = 'other'; ui.fields.source.onchange();
  ui.pending[1].resolve(range('c', 2)); await tick();
  ui.pending[0].resolve(range()); await tick();
  assert.match(ui.get('#preview').textContent, /2 个提交/);
  await submit(ui);
  assert.equal(ui.creates[0].preview.head, 'c'.repeat(40));
  ui.get('#new-review').onclick(); ui.get('#cancel-create').onclick();
  ui.get('#new-review').onclick();
  ui.pending[2].resolve(range()); await tick();
  assert.equal(ui.get('#create-button').disabled, true);
  ui.pending[3].resolve(range('d')); await tick();
  assert.match(ui.get('#preview').textContent, /dddddddddddd/);
});

test('preview errors offer retry and keep creation disabled', async () => {
  const ui = await setup();
  ui.fields.title.value = 'Review'; ui.get('#new-review').onclick();
  ui.pending[0].resolve({ error: 'no new commits' }, false); await tick();
  assert.equal(ui.get('#create-button').disabled, true);
  const retry = ui.get('#preview').children[1];
  assert.equal(retry.textContent, '重试');
  const retrying = retry.onclick(); ui.pending[1].resolve(range()); await retrying;
  assert.equal(ui.get('#create-button').disabled, false);
});

test('changed branches refresh the preview and require a second explicit submission', async () => {
  const ui = await setup();
  ui.fields.title.value = 'Review'; ui.get('#new-review').onclick();
  ui.pending[0].resolve(range()); await tick();
  ui.context.createError = { code: 'branches_changed', error: 'branches changed' };
  const submitting = submit(ui); await tick();
  assert.equal(ui.get('#create-button').disabled, true);
  ui.pending[1].resolve(range('c', 2)); await submitting;
  assert.equal(ui.creates.length, 1);
  assert.match(ui.get('#preview').textContent, /请确认后再次创建/);
  assert.equal(ui.get('#create-button').disabled, false);
  ui.context.createError = null; await submit(ui);
  assert.equal(ui.creates.length, 2);
  assert.equal(ui.creates[1].preview.head, 'c'.repeat(40));
});

test('title receives focus and shows an accessible error only after an empty blur', async () => {
  const ui = await setup();
  ui.get('#new-review').onclick();
  assert.equal(ui.fields.title.focused, true);
  assert.equal(ui.get('#title-error').hidden, true);
  ui.fields.title.onblur();
  assert.equal(ui.fields.title.attributes['aria-invalid'], 'true');
  assert.equal(ui.get('#title-error').hidden, false);
  ui.fields.title.value = '   '; ui.fields.title.oninput();
  assert.equal(ui.get('#title-error').hidden, false);
  ui.fields.title.value = 'Review'; ui.fields.title.oninput();
  assert.equal(ui.fields.title.attributes['aria-invalid'], 'false');
  assert.equal(ui.get('#title-error').hidden, true);
  assert.equal(ui.pending.length, 1);
  ui.pending[0].resolve(range()); await tick();
  assert.equal(ui.get('#create-button').disabled, false);
  ui.fields.title.value = ''; ui.fields.title.oninput();
  assert.equal(ui.get('#create-button').disabled, true);
  assert.equal(ui.get('#title-error').hidden, false);
  ui.get('#cancel-create').onclick(); ui.get('#new-review').onclick();
  assert.equal(ui.get('#title-error').hidden, true);
});
