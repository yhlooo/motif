'use strict';
const $ = s => document.querySelector(s);
const el = (tag, cls, text) => { const n = document.createElement(tag); if (cls) n.className = cls; if (text !== undefined) n.textContent = text; return n; };
const labels = { pending: '待审查', approved: '已批准', rejected: '要求修改', in_review: '审查中', changes_requested: '要求修改' };
let token = '', preview = null, current = null, anchor = 0, first = 0, last = 0, generation = 0, selectedLine = null, threadList = [];
const app = $('#app'), newReviewButton = $('#new-review');
function error(e) { $('#error').hidden = false; $('#error').textContent = e.message || String(e); }
function safe(fn) { return (...args) => { try { return Promise.resolve(fn(...args)).catch(error); } catch (e) { error(e); } }; }
async function api(path, method = 'GET', body) {
  $('#error').hidden = true;
  const response = await fetch('/api/' + path, { method, headers: { 'Content-Type': 'application/json', 'X-Motif-Token': token }, body: body === undefined ? undefined : JSON.stringify(body) });
  const data = await response.json().catch(() => ({ error: '服务返回了无效响应' }));
  if (!response.ok) { if(response.status === 409 && !data.code) data.error = '审查内容已更新，请刷新后重试。'; const e = new Error(data.error || '请求失败'); e.code = data.code; throw e; } return data;
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
  closeComposer();
  const sidebarScrollTop = $('.sidebar')?.scrollTop || 0;
  app.replaceChildren(); const top = el('section', 'review-top'), row = el('div', 'row');
  const back = el('a', '', '← 全部审查'); back.href = '/'; row.append(back, badge(current.status));
  if(current.refresh && current.refresh.code && current.refresh.code !== 'ok') top.append(el('p','description', `更新失败：${current.refresh.message}。以下内容及审阅操作针对已保存的 commit 范围。`));
  const range = el('div','muted', `范围 ${current.target_oid.slice(0,12)} → ${current.source_oid.slice(0,12)}`); range.title = `${current.target_oid} → ${current.source_oid}`;
  top.append(row, range, el('h1', '', current.title), el('div', 'muted', `${current.source} → ${current.target} · ${current.commits.length} 个提交 · ${date(current.created)}`), el('p', '', current.warning));
  const workspace = el('div', 'workspace'), sidebar = el('aside', 'sidebar'), content = el('section', 'content'); content.id = 'content';
  sidebar.append(el('p', '', '按意图审查 · Shift 点击连续选择'));
  let group = null, previousCommit;
  current.items.forEach((item, index) => {
    if (!group || item.commit_oid !== previousCommit) {
      group = el('section', 'commit-group');
      const heading = el('h3', 'commit-heading'), oid = item.commit_oid || '';
      const message = current.commit_messages?.[oid];
      const title = message === undefined ? (current.commit_titles?.[oid] ?? '提交标题不可用') : message.split(/\r?\n/, 1)[0];
      heading.append(el('code', '', oid ? oid.slice(0, 12) : '未归属提交'), el('span', '', oid ? (title || '（无提交标题）') : '未记录所属 commit'));
      heading.title = oid ? `${oid}\n${message === undefined ? title : (message || '（无提交说明）')}` : '未记录所属 commit';
      group.append(heading); sidebar.append(group); previousCommit = item.commit_oid;
    }
    const b = el('button', 'item' + (index >= first && index <= last ? ' selected' : '')); b.type = 'button'; b.dataset.index = index;
    const meta = el('span', 'item-meta'), lines = item.title.split(/\r?\n/);
    meta.append(el('span', 'number', String(index + 1).padStart(2, '0')), badge(item.decision));
    b.title = item.title;
    b.append(meta, el('strong', 'item-title', lines[0] + (lines.length > 1 ? '…' : '')));
    b.onclick = safe(async event => { if (event.shiftKey) { first = Math.min(anchor, index); last = Math.max(anchor, index); } else { first = last = anchor = index; } selectedLine = null; renderReview(); await renderSelection(); }); group.append(b);
  }); workspace.append(sidebar, content); app.append(top, workspace); sidebar.scrollTop = sidebarScrollTop;
}
function diffLineStats(file) {
  const stats = { added: 0, removed: 0, binary: file.binary ? 1 : 0, unavailable: file.old?.out_of_scope || file.new?.out_of_scope ? 1 : 0 };
  if (stats.binary || stats.unavailable) return stats;
  let inside = false;
  for (const line of file.patch.split('\n')) {
    if (/^@@ -\d+(?:,\d+)? \+\d+(?:,\d+)? @@/.test(line)) { inside = true; continue; }
    if (line.startsWith('diff --git ')) { inside = false; continue; }
    if (!inside) continue;
    if (line.startsWith('+')) stats.added++;
    else if (line.startsWith('-')) stats.removed++;
  }
  return stats;
}
function diffPathParts(file) {
  if (!file.path_base64) return file.path.split('/').map(name => ({ key: name, name }));
  return atob(file.path_base64).split('/').map(key => {
    const bytes = Uint8Array.from(key, c => c.charCodeAt(0));
    let name;
    try { name = new TextDecoder('utf-8', { fatal: true }).decode(bytes); }
    catch { name = Array.from(bytes, b => b >= 32 && b < 127 ? String.fromCharCode(b) : '\\x' + b.toString(16).padStart(2, '0')).join(''); }
    return { key, name };
  });
}
function buildDiffTree(diffs) {
  const makeNode = (name, directory) => ({ name, children: directory ? new Map() : null, added: 0, removed: 0, binary: 0, unavailable: 0 });
  const root = makeNode('', true);
  diffs.forEach((file, index) => {
    const stats = diffLineStats(file), parts = diffPathParts(file);
    let node = root;
    for (let i = 0; i < parts.length; i++) {
      for (const key of ['added', 'removed', 'binary', 'unavailable']) node[key] += stats[key];
      const directory = i < parts.length - 1, key = (directory ? 'dir:' : 'file:') + parts[i].key;
      if (!node.children.has(key)) node.children.set(key, makeNode(parts[i].name, directory));
      node = node.children.get(key);
    }
    Object.assign(node, stats, { index, path: file.path });
  });
  return root;
}
function diffStatsLabel(stats) {
  let label = `新增 ${stats.added} 行，删除 ${stats.removed} 行`;
  if (stats.binary) label += `；含 ${stats.binary} 个二进制文件，不计文本行数`;
  if (stats.unavailable) label += `；${stats.unavailable} 个文件内容未采集，不计文本行数`;
  return label;
}
function renderDiffStats(stats) {
  const counts = el('span', 'file-tree-stats');
  counts.title = diffStatsLabel(stats); counts.setAttribute('aria-label', counts.title);
  counts.append(el('span', 'diff-added-count', '+' + stats.added), el('span', 'diff-removed-count', '−' + stats.removed));
  if (stats.binary || stats.unavailable) counts.append(el('span', 'diff-count-note', '*'));
  return counts;
}
function renderDiffTree(diffs, targets) {
  const tree = buildDiffTree(diffs), nav = el('nav', 'file-tree');
  nav.setAttribute('aria-label', 'Diff 文件树');
  const heading = el('div', 'file-tree-heading');
  heading.append(el('strong', '', `变更文件 (${diffs.length})`), renderDiffStats(tree)); nav.append(heading);
  const fileButtons = [];
  function renderNodes(parent, depth) {
    const list = el('ul', 'file-tree-list');
    const nodes = [...parent.children.values()].sort((a, b) => Number(!!b.children) - Number(!!a.children) || a.name.localeCompare(b.name));
    for (const node of nodes) {
      const li = el('li'), directory = !!node.children;
      const row = el(directory ? 'summary' : 'button', 'file-tree-row');
      row.style.paddingLeft = `${8 + Math.min(depth, 6) * 12}px`;
      row.title = (node.path || node.name) + '\n' + diffStatsLabel(node);
      const chevron = el('span', 'file-tree-chevron' + (directory ? '' : ' spacer'));
      const icon = el('span', directory ? 'tree-folder-icon' : 'tree-file-icon');
      chevron.setAttribute('aria-hidden', 'true'); icon.setAttribute('aria-hidden', 'true');
      row.append(chevron, icon, el('span', 'file-tree-name', node.name), renderDiffStats(node));
      if (directory) {
        const details = el('details', 'file-tree-directory'); details.open = true;
        details.append(row, renderNodes(node, depth + 1)); li.append(details);
      } else {
        row.type = 'button'; fileButtons.push(row);
        row.onclick = () => {
          fileButtons.forEach(b => b.removeAttribute('aria-current'));
          row.setAttribute('aria-current', 'true');
          targets[node.index].scrollIntoView({ block: 'start' });
          targets[node.index].focus({ preventScroll: true });
        };
        li.append(row);
      }
      list.append(li);
    }
    return list;
  }
  nav.append(renderNodes(tree, 0));
  return nav;
}
async function renderSelection() {
  if (!current.items.length) { $('#content').replaceChildren(el('div','empty','当前源/目标分支之间没有待审阅修改。')); return; }
  const request = ++generation, reviewId = current.id, firstId = current.items[first].id, lastId = current.items[last].id;
  const [diffs, threads] = await Promise.all([api(`code-reviews/${reviewId}/diff?first=${encodeURIComponent(firstId)}&last=${encodeURIComponent(lastId)}&view_version=${current.view_version}&projection_id=${encodeURIComponent(current.projection_id)}`), api(`code-reviews/${reviewId}/threads?first=${encodeURIComponent(firstId)}&last=${encodeURIComponent(lastId)}&view_version=${current.view_version}&projection_id=${encodeURIComponent(current.projection_id)}`)]);
  if (request !== generation || current.id !== reviewId) return; threadList = threads;
  closeComposer();
  const view = { request, reviewId, firstID: firstId, lastID: lastId, version: current.view_version, projectionID: current.projection_id, files: new Map() };
  const root = $('#content'); root.replaceChildren();
  root.append(el('span', 'eyebrow', first === last ? 'INTENT BATCH' : 'COMBINED DIFF'), el('h2', '', first === last ? current.items[first].title : `合并审查第 ${first + 1}–${last + 1} 项`));
  for (let i = first; i <= last; i++) if (current.items[i].description) root.append(el('div', 'description', current.items[i].description));
  const actions = el('div', 'actions');
  for (const [decision, label, cls] of [['approved', '批准所选', ''], ['rejected', '要求修改', 'reject'], ['pending', '重置为待审查', 'secondary']]) {
    actions.append(button(label, async () => {
      const ids = current.items.slice(first, last + 1).map(i => i.id), id = current.id;
      await api(`code-reviews/${id}/decisions`, 'PATCH', { items: ids, decision, view_version: current.view_version, expected_revisions: Object.fromEntries(current.items.slice(first,last+1).map(i => [i.id,i.decision_revision])) });
      current = await api('code-reviews/' + id); renderReview(); await renderSelection();
    }, cls));
  } root.append(actions);
  const sources = el('div', 'source-list');
  current.items.slice(first, last + 1).flatMap(i => i.events).forEach(id => sources.append(button('原始事件 ' + id.slice(0, 12), async () => {
    const data = await api('patches/' + id); let node = root.querySelector('.source'); if (!node) { node = el('pre', 'source'); sources.after(node); }
    node.textContent = JSON.stringify(data.event, null, 2) + '\n\n' + data.diff.map(f => f.patch).join('\n');
  }))); if (sources.childNodes.length) root.append(sources);
  root.append(el('p', 'muted', '点击行号添加评论；Shift 点击同一侧的另一行选择行范围。'));
  if (!diffs.length) root.append(el('div', 'empty', '所选修改互相抵消，净差异为空。原始事件仍可追溯。'));
  const diffLayout = el('div', 'diff-layout'), diffFiles = el('div', 'diff-files'), fileTargets = [];
  view.diffFiles = diffFiles;
  diffs.forEach(file => {
    const fileView = { rows: { old: new Map(), new: new Map() } };
    const box = el('section', 'file'), code = el('div', 'diff'); box.append(el('div', 'file-title', file.path), code); box.tabIndex = -1; fileTargets.push(box); fileView.box = box; fileView.code = code; view.files.set(file.path_base64, fileView);
    let old = 0, newer = 0, inside = false;
    file.patch.split('\n').forEach(line => {
      const hunk = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(line);
      if (hunk) { old = Number(hunk[1]); newer = Number(hunk[2]); inside = true; code.append(el('div', 'diff-line hunk', line)); return; }
      if (!inside || !/^[ +\-]/.test(line)) { if (line) code.append(el('div', 'diff-line hunk', line)); return; }
      const row = el('div', 'diff-line' + (line[0] === '+' ? ' add' : line[0] === '-' ? ' del' : ''));
      const oldNo = line[0] === '+' ? null : old++, newNo = line[0] === '-' ? null : newer++;
      for (const [side, num] of [['old', oldNo], ['new', newNo]]) {
        if (num !== null) fileView.rows[side].set(num, row);
        const b = el('button', 'line-no', num === null ? '' : String(num)); b.type = 'button'; b.disabled = num === null;
        b.setAttribute('aria-label', `${side === 'old' ? '旧' : '新'}侧第 ${num} 行评论`);
        b.onclick = event => {
          if (event.shiftKey && selectedLine && selectedLine.path === file.path_base64 && selectedLine.side === side) { selectedLine.end = Math.max(selectedLine.anchor, num); selectedLine.start = Math.min(selectedLine.anchor, num); }
          else selectedLine = { path: file.path_base64, display: file.path, side, start: num, end: num, anchor: num };
          showComposer(view, b);
        }; row.append(b);
      } row.append(el('span', 'code', line)); code.append(row);
    }); diffFiles.append(box);
  });
  if (diffs.length) { diffLayout.append(diffFiles, renderDiffTree(diffs, fileTargets)); root.append(diffLayout); }
  else if (threads.length) root.append(diffFiles);
  renderThreads(view);
}
let commentComposer = null, commentRefreshGeneration = 0;
function isCurrentCommentView(view) {
  return generation === view.request && current?.id === view.reviewId && current.view_version === view.version && current.projection_id === view.projectionID;
}
function mappedCommentRanges(thread) {
  return thread.mapped_ranges?.length ? thread.mapped_ranges : [{ start: thread.mapped_start || thread.start, end: thread.mapped_end || thread.end }];
}
function commentAnchorRow(thread, file) {
  const rows = file.rows[thread.side], ranges = mappedCommentRanges(thread);
  let result = null;
  for (const [number, row] of rows) if (ranges.some(range => number >= range.start && number <= range.end)) result = row;
  return result;
}
function clearCommentSelection(view) {
  for (const file of view.files.values()) for (const rows of Object.values(file.rows)) for (const row of rows.values()) row.classList.remove('chosen');
}
function closeComposer(returnFocus = false) {
  if (!commentComposer) return;
  const state = commentComposer; commentComposer = null;
  state.box.remove(); state.observer?.disconnect();
  document.removeEventListener('scroll', state.position, true);
  window.removeEventListener('resize', state.position);
  document.removeEventListener('keydown', state.keydown);
  clearCommentSelection(state.view); selectedLine = null;
  if (returnFocus && state.anchor.isConnected) state.anchor.focus({ preventScroll: true });
}
async function refreshCommentThreads(view) {
  const request = ++commentRefreshGeneration;
  const threads = await api(`code-reviews/${view.reviewId}/threads?first=${encodeURIComponent(view.firstID)}&last=${encodeURIComponent(view.lastID)}&view_version=${view.version}&projection_id=${encodeURIComponent(view.projectionID)}`);
  if (!isCurrentCommentView(view) || request !== commentRefreshGeneration) return;
  const positions = [...view.files.values()].map(file => [file.code, file.code.scrollTop, file.code.scrollLeft]);
  const x = window.scrollX, y = window.scrollY;
  threadList = threads; renderThreads(view);
  positions.forEach(([code, top, left]) => { code.scrollTop = top; code.scrollLeft = left; });
  window.scrollTo(x, y);
  commentComposer?.position();
}
function showComposer(view, anchorButton) {
  if (commentComposer?.submitting || !isCurrentCommentView(view)) return;
  const selection = { ...selectedLine }, draft = commentComposer?.saved ? '' : (commentComposer?.text.value || '');
  closeComposer(); selectedLine = selection;
  const box = el('section', 'composer line-composer'), text = el('textarea'), status = el('p', 'comment-error');
  box.setAttribute('role', 'dialog'); box.setAttribute('aria-label', '添加行评论');
  status.setAttribute('role', 'alert'); status.hidden = true;
  box.append(el('p', 'composer-location', `${selection.display} · ${selection.side === 'old' ? '旧' : '新'}侧第 ${selection.start}–${selection.end} 行`));
  text.value = draft; text.placeholder = '针对这些代码写下评论…'; text.setAttribute('aria-label', '行评论');
  const state = { box, text, view, anchor: anchorButton, submitting: false, saved: false };
  const submit = button('发表评论', async () => {
    if (state.submitting) return;
    if (!isCurrentCommentView(view)) { status.hidden = false; status.textContent = '审查范围已变化，请重新选择代码行。'; return; }
    if (!text.value.trim()) { status.hidden = false; status.textContent = '请输入评论内容。'; return; }
    state.submitting = true; submit.disabled = true; text.disabled = true; status.hidden = true;
    try {
      if (!state.saved) {
        await api(`code-reviews/${view.reviewId}/threads`, 'POST', { first: view.firstID, last: view.lastID, path: selection.path, side: selection.side, start: selection.start, end: selection.end, body: text.value, view_version: view.version });
        state.saved = true;
      }
      await refreshCommentThreads(view);
      if (commentComposer === state) closeComposer(true);
    } catch (e) {
      status.hidden = false; status.textContent = (state.saved ? '评论已保存，加载失败：' : '') + e.message;
    } finally {
      state.submitting = false; submit.disabled = false; text.disabled = state.saved;
      submit.textContent = state.saved ? '重新加载评论' : '发表评论'; state.position();
    }
  });
  const actions = el('div', 'comment-actions'); actions.append(submit, button('取消', () => closeComposer(true), 'secondary'));
  box.append(text, status, actions);
  state.position = () => {
    if (commentComposer !== state) return;
    const rect = anchorButton.getBoundingClientRect(), clip = anchorButton.closest('.diff').getBoundingClientRect();
    const visible = anchorButton.isConnected && rect.bottom > Math.max(0, clip.top) && rect.top < Math.min(window.innerHeight, clip.bottom) && rect.right > Math.max(0, clip.left) && rect.left < Math.min(window.innerWidth, clip.right);
    box.hidden = !visible;
    if (!visible) return;
    const width = box.offsetWidth, height = box.offsetHeight, margin = 8;
    const left = Math.max(margin, Math.min(rect.right + 8, window.innerWidth - width - margin));
    let top = rect.bottom + 6;
    if (top + height > window.innerHeight - margin) top = Math.max(margin, rect.top - height - 6);
    box.style.left = left + 'px'; box.style.top = top + 'px';
  };
  state.keydown = event => {
    if (event.key === 'Escape') { event.preventDefault(); closeComposer(true); }
  };
  text.onkeydown = event => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); submit.click(); } };
  commentComposer = state; document.body.append(box);
  const file = view.files.get(selection.path);
  for (const [number, row] of file.rows[selection.side]) if (number >= selection.start && number <= selection.end) row.classList.add('chosen');
  document.addEventListener('scroll', state.position, true); window.addEventListener('resize', state.position); document.addEventListener('keydown', state.keydown);
  if (typeof ResizeObserver === 'function') { state.observer = new ResizeObserver(state.position); state.observer.observe(box); }
  state.position(); text.focus({ preventScroll: true });
}
function renderCommentThread(thread, view, outsideHunk) {
  const box = el('article', 'thread'); box.dataset.threadId = thread.id;
  const ranges = mappedCommentRanges(thread).map(range => range.start === range.end ? String(range.start) : `${range.start}–${range.end}`).join('、');
  box.append(el('div', 'muted', `${thread.side === 'old' ? '旧' : '新'}侧第 ${ranges} 行${outsideHunk ? ' · 当前 diff 未展开此处代码' : ''}`));
  thread.comments.forEach(comment => { box.append(el('div', 'comment-author', `${comment.author} · ${date(comment.created)}`), el('p', 'comment-body', comment.body)); });
  const reply = el('details', 'inline-reply'), text = el('textarea'), status = el('p', 'comment-error');
  const summary = el('summary', '', '回复'); text.placeholder = '回复这条评论…'; text.setAttribute('aria-label', '回复评论'); status.setAttribute('role', 'alert'); status.hidden = true;
  let saved = false, submitting = false;
  const submit = button('发送回复', async () => {
    if (submitting) return;
    if (!isCurrentCommentView(view)) { status.hidden = false; status.textContent = '审查范围已变化，请重新打开评论。'; return; }
    if (!text.value.trim()) { status.hidden = false; status.textContent = '请输入回复内容。'; return; }
    submitting = true; submit.disabled = true; text.disabled = true; status.hidden = true;
    try {
      if (!saved) { await api(`comment-threads/${thread.id}/replies`, 'POST', { body: text.value, view_version: view.version }); saved = true; }
      await refreshCommentThreads(view);
    } catch (e) { status.hidden = false; status.textContent = (saved ? '回复已保存，加载失败：' : '') + e.message; }
    finally { submitting = false; submit.disabled = false; text.disabled = saved; submit.textContent = saved ? '重新加载回复' : '发送回复'; }
  });
  reply.append(summary, text, status, submit); box.append(reply); return box;
}
function renderThreads(view) {
  view.threadContainers?.forEach(container => container.remove()); view.threadContainers = [];
  for (const [path, file] of view.files) if (file.commentOnly) { file.box.remove(); view.files.delete(path); }
  const groups = new Map();
  for (const thread of threadList) {
    const path = thread.mapped_path || thread.path;
    let file = view.files.get(path);
    if (!file) {
      const section = el('section', 'file comment-only-file'), code = el('div', 'diff');
      const display = diffPathParts({ path: '', path_base64: path }).map(part => part.name).join('/');
      section.append(el('div', 'file-title', display), code); view.diffFiles.append(section);
      file = { box: section, code, rows: { old: new Map(), new: new Map() }, commentOnly: true }; view.files.set(path, file);
    }
    const row = commentAnchorRow(thread, file), key = row || file.code;
    let group = groups.get(key);
    if (!group) {
      group = el('div', 'inline-threads');
      if (row) row.after(group); else file.code.append(group);
      groups.set(key, group); view.threadContainers.push(group);
    }
    group.append(renderCommentThread(thread, view, !row));
  }
}
let previewGeneration = 0, creatingReview = false, titleTouched = false;
const createForm = $('#create-form'), createDialog = $('#create-dialog');
function updateCreateButton() {
  $('#create-button').disabled = creatingReview || !preview || !createForm.elements.title.value.trim();
}
function validateTitle() {
  const invalid = titleTouched && !createForm.elements.title.value.trim();
  createForm.elements.title.setAttribute('aria-invalid', String(invalid));
  $('#title-error').hidden = !invalid;
  updateCreateButton();
}
async function previewRange(notice = '') {
  const request = ++previewGeneration;
  preview = null; updateCreateButton();
  const source = createForm.elements.source.value, target = createForm.elements.target.value;
  if (!source || !target) { $('#preview').textContent = '请选择源分支和目标分支。'; return; }
  $('#preview').textContent = '正在计算审查范围…';
  try {
    const result = await api('code-reviews/preview', 'POST', { source, target });
    if (request !== previewGeneration || !createDialog.open) return;
    preview = result;
    $('#preview').textContent = notice + `基准 ${preview.base.slice(0, 12)} → ${preview.head.slice(0, 12)}\n${preview.commits.length} 个提交 · ${preview.diff.length} 个变更文件`;
  } catch (e) {
    if (request !== previewGeneration || !createDialog.open) return;
    $('#preview').replaceChildren(el('span', '', '无法计算审查范围：' + e.message + ' '), button('重试', () => previewRange(notice), 'secondary'));
  }
  updateCreateButton();
}
newReviewButton.onclick = () => { titleTouched = false; validateTitle(); createDialog.showModal(); createForm.elements.title.focus(); previewRange(); };
$('#cancel-create').onclick = () => createDialog.close();
createDialog.onclose = () => { ++previewGeneration; preview = null; updateCreateButton(); };
createForm.elements.title.onblur = () => { titleTouched = true; validateTitle(); };
createForm.elements.title.oninput = validateTitle;
for (const field of ['source', 'target']) createForm.elements[field].onchange = () => previewRange();
createForm.onsubmit = safe(async event => {
  event.preventDefault();
  if (!preview || creatingReview || !createForm.elements.title.value.trim()) return;
  creatingReview = true; updateCreateButton();
  for (const field of ['source', 'target', 'title']) createForm.elements[field].disabled = true;
  newReviewButton.disabled = true;
  try {
    let r;
    try { r = await api('code-reviews', 'POST', { title: createForm.elements.title.value, preview }); }
    catch (e) {
      if (e.code !== 'branches_changed') throw e;
      if (createDialog.open) await previewRange('分支已变化，范围已更新，请确认后再次创建。\n');
      return;
    }
    createDialog.close(); await openReview(r.id);
  } finally {
    creatingReview = false;
    for (const field of ['source', 'target', 'title']) createForm.elements[field].disabled = false;
    newReviewButton.disabled = false; updateCreateButton();
  }
});
$('#filter').onchange = safe(listReviews);
safe(async () => {
  const data = await api('bootstrap'); token = data.token; $('#repository').textContent = data.repository;
  for (const field of ['source', 'target']) data.branches.forEach(branch => { const option = el('option', '', branch); option.value = branch; $('#create-form').elements[field].append(option); });
  const fields = $('#create-form').elements; fields.target.value = data.branches.includes('main') ? 'main' : data.branches.includes('master') ? 'master' : data.branches[0] || '';
  fields.source.value = data.branches.find(b => b !== fields.target.value) || fields.target.value;
  const match = /^#review=([a-zA-Z0-9-]+)$/.exec(location.hash); if (match) await openReview(match[1]); else await listReviews();
})();
