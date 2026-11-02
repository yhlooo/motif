'use strict';
const $ = s => document.querySelector(s);
const el = (tag, cls, text) => { const n = document.createElement(tag); if (cls) n.className = cls; if (text !== undefined) n.textContent = text; return n; };
const labels = { pending: '待审查', approved: '已批准', rejected: '要求修改', in_review: '审查中', changes_requested: '要求修改' };
let token = '', preview = null, current = null, anchor = 0, first = 0, last = 0, generation = 0, selectedLine = null, threadList = [];
const app = $('#app');
function error(e) { $('#error').hidden = false; $('#error').textContent = e.message || String(e); }
function safe(fn) { return (...args) => { try { return Promise.resolve(fn(...args)).catch(error); } catch (e) { error(e); } }; }
async function api(path, method = 'GET', body) {
  $('#error').hidden = true;
  const response = await fetch('/api/' + path, { method, headers: { 'Content-Type': 'application/json', 'X-Motif-Token': token }, body: body === undefined ? undefined : JSON.stringify(body) });
  const data = await response.json().catch(() => ({ error: '服务返回了无效响应' }));
  if (!response.ok) { if(response.status === 409) data.error = '审查内容已更新，请刷新后重试。'; throw new Error(data.error || '请求失败'); } return data;
}
function badge(state) { return el('span', 'badge ' + state, labels[state] || state); }
function button(text, fn, cls) { const b = el('button', cls, text); b.type = 'button'; b.onclick = safe(fn); return b; }
function date(value) { return new Date(value / 1000).toLocaleString('zh-CN'); }
async function listReviews() {
  const reviews = await api('code-reviews'), filter = $('#filter').value, root = $('#reviews'); root.replaceChildren();
  const filtered = reviews.filter(r => filter === 'all' || (filter === 'approved' ? r.status === 'approved' : r.status !== 'approved'));
  if (!filtered.length) { root.append(el('div', 'empty', '还没有审查记录。创建审查，开始按意图阅读代码。')); return; }
  filtered.forEach(r => {
    const card = el('article', 'review-card'), info = el('div'); card.tabIndex = 0; card.setAttribute('role', 'button');
    info.append(el('h2', '', r.title), el('div', 'muted', `${r.source} → ${r.target} · ${date(r.created)}`));
    info.append(el('div','muted', `范围 ${r.target_oid.slice(0,12)} → ${r.source_oid.slice(0,12)}`));
    if(r.refresh && r.refresh.code && r.refresh.code !== 'ok') info.append(el('p','muted','上次更新失败，已保留原 commit 范围。'));
    const status = el('div'); status.append(badge(r.status), el('p', 'muted', `${r.items.filter(i => i.decision === 'approved').length} / ${r.items.length} 项已批准`));
    card.append(info, status); card.onclick = safe(() => openReview(r.id)); card.onkeydown = e => { if (e.key === 'Enter') card.click(); }; root.append(card);
  });
}
async function openReview(id) {
  let refreshError = null;
  try { await api('code-reviews/' + id + '/refresh', 'POST'); } catch(e) { refreshError = e; }
  current = await api('code-reviews/' + id); first = last = anchor = 0; history.replaceState(null, '', '#review=' + id); renderReview(); await renderSelection(); if(refreshError) error(refreshError);
}
function renderReview() {
  app.replaceChildren(); const top = el('section', 'review-top'), row = el('div', 'row');
  const back = el('a', '', '← 全部审查'); back.href = '/'; row.append(back, badge(current.status));
  row.append(button('刷新审查', () => openReview(current.id), 'secondary'));
  if(current.refresh && current.refresh.code && current.refresh.code !== 'ok') top.append(el('p','description', `更新失败：${current.refresh.message}。以下内容及审阅操作针对已保存的 commit 范围。`));
  const range = el('div','muted', `范围 ${current.target_oid.slice(0,12)} → ${current.source_oid.slice(0,12)}`); range.title = `${current.target_oid} → ${current.source_oid}`;
  top.append(row, range, el('h1', '', current.title), el('div', 'muted', `${current.source} → ${current.target} · ${current.commits.length} 个提交 · ${date(current.created)}`), el('p', '', current.warning));
  const workspace = el('div', 'workspace'), sidebar = el('aside', 'sidebar'), content = el('section', 'content'); content.id = 'content';
  sidebar.append(el('p', '', '按意图审查 · Shift 点击连续选择'));
  current.items.forEach((item, index) => {
    const b = el('button', 'item' + (index >= first && index <= last ? ' selected' : '')); b.type = 'button'; b.dataset.index = index;
    b.append(el('span', 'number', `${String(index + 1).padStart(2, '0')} / ${item.kind}`), el('strong', '', item.title), el('br'), badge(item.decision));
    b.onclick = safe(async event => { if (event.shiftKey) { first = Math.min(anchor, index); last = Math.max(anchor, index); } else { first = last = anchor = index; } selectedLine = null; renderReview(); await renderSelection(); }); sidebar.append(b);
  }); workspace.append(sidebar, content); app.append(top, workspace);
}
async function renderSelection() {
  if (!current.items.length) { $('#content').replaceChildren(el('div','empty','当前源/目标分支之间没有待审阅修改。')); return; }
  const request = ++generation, reviewId = current.id, firstId = current.items[first].id, lastId = current.items[last].id;
  const [diffs, threads] = await Promise.all([api(`code-reviews/${reviewId}/diff?first=${encodeURIComponent(firstId)}&last=${encodeURIComponent(lastId)}&view_version=${current.view_version}`), api(`code-reviews/${reviewId}/threads?first=${encodeURIComponent(firstId)}&last=${encodeURIComponent(lastId)}&view_version=${current.view_version}`)]);
  if (request !== generation || current.id !== reviewId) return; threadList = threads;
  const root = $('#content'); root.replaceChildren();
  root.append(el('span', 'eyebrow', first === last ? 'INTENT BATCH' : 'COMBINED DIFF'), el('h2', '', first === last ? current.items[first].title : `合并审查第 ${first + 1}–${last + 1} 项`));
  for (let i = first; i <= last; i++) if (current.items[i].description) root.append(el('div', 'description', current.items[i].description));
  const actions = el('div', 'actions');
  for (const [decision, label, cls] of [['approved', '批准所选', ''], ['rejected', '要求修改', 'reject'], ['pending', '重置为待审查', 'secondary']]) {
    actions.append(button(label, async () => {
      const ids = current.items.slice(first, last + 1).map(i => i.id), id = current.id;
      await api(`code-reviews/${id}/decisions`, 'PATCH', { items: ids, decision, view_version: current.view_version });
      current = await api('code-reviews/' + id); renderReview(); await renderSelection();
    }, cls));
  } root.append(actions);
  const sources = el('div', 'source-list');
  current.items.slice(first, last + 1).flatMap(i => i.events).forEach(id => sources.append(button('原始事件 ' + id.slice(0, 12), async () => {
    const data = await api('patches/' + id); let node = root.querySelector('.source'); if (!node) { node = el('pre', 'source'); sources.after(node); }
    node.textContent = JSON.stringify(data.event, null, 2) + '\n\n' + data.diff.map(f => f.patch).join('\n');
  }))); if (sources.childNodes.length) root.append(sources);
  root.append(el('p', 'muted', '点击行号添加评论；Shift 点击同一侧的另一行选择行范围。'));
  const composer = el('div'); composer.id = 'composer'; root.append(composer);
  if (!diffs.length) root.append(el('div', 'empty', '所选修改互相抵消，净差异为空。原始事件仍可追溯。'));
  diffs.forEach(file => {
    const box = el('section', 'file'), code = el('div', 'diff'); box.append(el('div', 'file-title', file.path), code);
    let old = 0, newer = 0, inside = false;
    file.patch.split('\n').forEach(line => {
      const hunk = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(line);
      if (hunk) { old = Number(hunk[1]); newer = Number(hunk[2]); inside = true; code.append(el('div', 'diff-line hunk', line)); return; }
      if (!inside || !/^[ +\-]/.test(line)) { if (line) code.append(el('div', 'diff-line hunk', line)); return; }
      const row = el('div', 'diff-line' + (line[0] === '+' ? ' add' : line[0] === '-' ? ' del' : ''));
      const oldNo = line[0] === '+' ? null : old++, newNo = line[0] === '-' ? null : newer++;
      for (const [side, num] of [['old', oldNo], ['new', newNo]]) {
        const b = el('button', 'line-no', num === null ? '' : String(num)); b.type = 'button'; b.disabled = num === null;
        b.setAttribute('aria-label', `${side === 'old' ? '旧' : '新'}侧第 ${num} 行评论`);
        b.onclick = event => {
          if (event.shiftKey && selectedLine && selectedLine.path === file.path_base64 && selectedLine.side === side) { selectedLine.end = Math.max(selectedLine.anchor, num); selectedLine.start = Math.min(selectedLine.anchor, num); }
          else selectedLine = { path: file.path_base64, display: file.path, side, start: num, end: num, anchor: num };
          showComposer(firstId, lastId);
        }; row.append(b);
      } row.append(el('span', 'code', line)); code.append(row);
    }); root.append(box);
  });
  renderThreads(firstId, lastId);
}
function showComposer(firstId, lastId) {
  const root = $('#composer'); root.replaceChildren(); const box = el('section', 'composer'), selection = { ...selectedLine };
  box.append(el('p', '', `${selection.display} · ${selection.side === 'old' ? '旧' : '新'}侧 ${selection.start}–${selection.end} 行`));
  const text = el('textarea'); text.setAttribute('aria-label', '行评论'); box.append(text);
  box.append(button('发表行评论', async () => {
    if (!text.value.trim()) throw new Error('请输入评论内容');
    await api(`code-reviews/${current.id}/threads`, 'POST', { first: firstId, last: lastId, path: selection.path, side: selection.side, start: selection.start, end: selection.end, body: text.value,view_version:current.view_version }); selectedLine = null; await renderSelection();
  }), button('取消', () => { root.replaceChildren(); selectedLine = null; }, 'secondary')); root.append(box); text.focus();
}
function renderThreads(firstId, lastId) {
  const root = el('section', 'threads'); root.append(el('h2', '', '当前视图评论'));
  const matching = threadList;
  if (!matching.length) root.append(el('p', 'muted', '暂无评论。批准和评论均保存在本机，不会执行 Git 合并。'));
  matching.forEach(thread => {
    const box = el('article', 'thread'); box.append(el('div', 'muted', `${thread.side === 'old' ? '旧' : '新'}侧第 ${(thread.mapped_ranges || [{start:thread.start,end:thread.end}]).map(r => `${r.start}–${r.end}`).join('、')} 行 · 内容 ${thread.hash.slice(0, 12)}`));
    thread.comments.forEach(c => { box.append(el('div', 'muted', `${c.author} · ${date(c.created)}`), el('p', '', c.body)); });
    const text = el('textarea'); text.placeholder = '回复这条评论…'; text.setAttribute('aria-label', '回复评论');
    box.append(text, button('回复', async () => { if (!text.value.trim()) throw new Error('请输入回复内容'); await api(`comment-threads/${thread.id}/replies`, 'POST', { body: text.value, view_version: current.view_version }); await renderSelection(); })); root.append(box);
  }); $('#content').append(root);
}
$('#new-review').onclick = () => { preview = null; $('#preview').textContent = ''; $('#create-button').disabled = true; $('#create-dialog').showModal(); };
$('#cancel-create').onclick = () => $('#create-dialog').close();
$('#create-form').oninput = () => { preview = null; $('#create-button').disabled = true; $('#preview').textContent = ''; };
$('#preview-button').onclick = safe(async () => {
  const form = $('#create-form'); preview = await api('code-reviews/preview', 'POST', { source: form.elements.source.value, target: form.elements.target.value });
  $('#preview').textContent = `基准 ${preview.base.slice(0, 12)} → ${preview.head.slice(0, 12)}\n${preview.commits.length} 个提交 · ${preview.diff.length} 个变更文件`; $('#create-button').disabled = false;
});
$('#create-form').onsubmit = safe(async event => { event.preventDefault(); if (!preview) return; const b = $('#create-button'); b.disabled = true; try { const r = await api('code-reviews', 'POST', { title: $('#create-form').elements.title.value, preview }); $('#create-dialog').close(); await openReview(r.id); } finally { b.disabled = !preview; } });
$('#filter').onchange = safe(listReviews);
safe(async () => {
  const data = await api('bootstrap'); token = data.token; $('#repository').textContent = data.repository;
  for (const field of ['source', 'target']) data.branches.forEach(branch => { const option = el('option', '', branch); option.value = branch; $('#create-form').elements[field].append(option); });
  const fields = $('#create-form').elements; fields.target.value = data.branches.includes('main') ? 'main' : data.branches.includes('master') ? 'master' : data.branches[0] || '';
  fields.source.value = data.branches.find(b => b !== fields.target.value) || fields.target.value;
  const match = /^#review=([a-zA-Z0-9-]+)$/.exec(location.hash); if (match) await openReview(match[1]); else await listReviews();
})();
