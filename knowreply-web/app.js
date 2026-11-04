'use strict';

// 管理台前端：零构建，直接调用网关（8080）与检索预览服务（8081）。
// 登录态只是联调用的简化流程：填租户与用户 → 换 JWT → 存 localStorage。

const LS = {
  apiBase: 'kr_api_base',
  kbBase: 'kr_kb_base',
  tenantId: 'kr_tenant_id',
  userId: 'kr_user_id',
  token: 'kr_token',
};

const el = (id) => document.getElementById(id);

const state = {
  token: localStorage.getItem(LS.token) || '',
  timer: null,
};

function apiBase() {
  return el('apiBase').value.trim().replace(/\/+$/, '');
}

function kbBase() {
  return el('kbBase').value.trim().replace(/\/+$/, '');
}

function tenantId() {
  return Number(el('tenantId').value) || 1;
}

function escapeHTML(str) {
  return String(str ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

function setMsg(id, text, kind) {
  const node = el(id);
  node.textContent = text || '';
  node.className = 'msg' + (kind ? ' ' + kind : '');
}

function renderTokenState() {
  const pill = el('tokenState');
  if (state.token) {
    pill.textContent = '已登录 · 租户 ' + tenantId();
    pill.className = 'pill pill-on';
  } else {
    pill.textContent = '未登录';
    pill.className = 'pill pill-off';
  }
}

async function request(base, path, options = {}) {
  const headers = Object.assign({}, options.headers || {});
  if (state.token) headers.Authorization = 'Bearer ' + state.token;

  const res = await fetch(base + path, Object.assign({}, options, { headers }));
  const text = await res.text();
  let body = null;
  try {
    body = text ? JSON.parse(text) : null;
  } catch {
    body = { raw: text };
  }

  if (!res.ok) {
    const detail = (body && (body.error || body.message)) || res.statusText;
    throw new Error(`${res.status} ${detail}`);
  }
  return body;
}

// ---------- 设置与登录 ----------

function loadSettings() {
  el('apiBase').value = localStorage.getItem(LS.apiBase) || 'http://127.0.0.1:8080';
  el('kbBase').value = localStorage.getItem(LS.kbBase) || 'http://127.0.0.1:8081';
  el('tenantId').value = localStorage.getItem(LS.tenantId) || '1';
  el('userId').value = localStorage.getItem(LS.userId) || 'admin';
}

function saveSettings() {
  localStorage.setItem(LS.apiBase, apiBase());
  localStorage.setItem(LS.kbBase, kbBase());
  localStorage.setItem(LS.tenantId, String(tenantId()));
  localStorage.setItem(LS.userId, el('userId').value.trim());
}

async function issueToken() {
  saveSettings();
  const btn = el('btnToken');
  btn.disabled = true;
  try {
    const body = await request(apiBase(), '/api/v1/auth/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenantId: tenantId(), userId: el('userId').value.trim() || 'admin' }),
    });
    state.token = body.token;
    localStorage.setItem(LS.token, body.token);
    renderTokenState();
    setMsg('uploadMsg', '令牌获取成功，有效期至 ' + new Date(body.expiresAt).toLocaleString(), 'ok');
    el('settingsPanel').classList.add('hidden');
    refreshDocs();
  } catch (err) {
    setMsg('uploadMsg', '获取令牌失败：' + err.message, 'err');
  } finally {
    btn.disabled = false;
  }
}

// ---------- 知识库 ----------

const statusText = {
  pending: '待处理',
  processing: '处理中',
  ready: '已入库',
  failed: '失败',
  offline: '已下线',
};

function renderDocs(items) {
  const tbody = el('docRows');
  if (!items.length) {
    tbody.innerHTML = '<tr><td colspan="5" class="empty">暂无文档，先上传一份试试</td></tr>';
    return;
  }

  tbody.innerHTML = items
    .map((d) => {
      const badge = `<span class="badge ${escapeHTML(d.status)}">${escapeHTML(
        statusText[d.status] || d.status
      )}</span>`;
      const time = d.createdAt ? new Date(d.createdAt).toLocaleString() : '—';
      const canOffline = d.status !== 'offline';
      return `<tr>
        <td>${d.id}</td>
        <td title="${escapeHTML(d.sourceUrl || '')}">${escapeHTML(d.title)}</td>
        <td>${badge}</td>
        <td>${escapeHTML(time)}</td>
        <td>
          <button class="link-btn" data-offline="${d.id}" ${canOffline ? '' : 'disabled'}>下线</button>
        </td>
      </tr>`;
    })
    .join('');
}

async function refreshDocs() {
  if (!state.token) {
    el('docRows').innerHTML = '<tr><td colspan="5" class="empty">请先在「连接设置」获取访问令牌</td></tr>';
    return;
  }

  try {
    const status = el('statusFilter').value;
    const query = '/api/v1/kb/documents?limit=50' + (status ? '&status=' + encodeURIComponent(status) : '');
    const body = await request(apiBase(), query);
    renderDocs(body.items || []);
    setMsg('listMsg', '');

    // 有在途任务时轮询，直到全部落定
    const pending = (body.items || []).some((d) => d.status === 'pending' || d.status === 'processing');
    if (pending && !state.timer) {
      state.timer = setTimeout(() => {
        state.timer = null;
        refreshDocs();
      }, 3000);
    }
  } catch (err) {
    setMsg('listMsg', '加载失败：' + err.message, 'err');
  }
}

async function upload() {
  const file = el('fileInput').files[0];
  if (!file) {
    setMsg('uploadMsg', '请先选择文件', 'err');
    return;
  }
  if (!state.token) {
    setMsg('uploadMsg', '请先获取访问令牌', 'err');
    return;
  }

  const btn = el('btnUpload');
  btn.disabled = true;
  setMsg('uploadMsg', '上传中…');

  try {
    const form = new FormData();
    form.append('file', file);
    const title = el('docTitle').value.trim();
    if (title) form.append('title', title);

    const body = await request(apiBase(), '/api/v1/kb/documents', { method: 'POST', body: form });
    setMsg('uploadMsg', `已入队：docId=${body.docId}，任务 ${body.taskId}，稍等几秒查看状态`, 'ok');
    el('fileInput').value = '';
    el('docTitle').value = '';
    refreshDocs();
  } catch (err) {
    setMsg('uploadMsg', '上传失败：' + err.message, 'err');
  } finally {
    btn.disabled = false;
  }
}

async function offlineDoc(docId) {
  if (!confirm(`确认下线文档 ${docId}？向量切片会被删除，记录保留。`)) return;
  try {
    await request(apiBase(), '/api/v1/kb/documents/' + docId, { method: 'DELETE' });
    setMsg('listMsg', `文档 ${docId} 已下线`, 'ok');
    refreshDocs();
  } catch (err) {
    setMsg('listMsg', '下线失败：' + err.message, 'err');
  }
}

// ---------- 检索测试 ----------

function renderHits(data) {
  el('searchMeta').innerHTML = [
    `向量召回 ${data.vectorHits}`,
    `全文本召回 ${data.textHits}`,
    `二元组召回 ${data.gramHits}`,
    `耗时 ${data.tookMs}ms`,
    `向量器 ${escapeHTML(data.embedder)}`,
  ]
    .map((t) => `<span>${t}</span>`)
    .join('');

  const box = el('hits');
  if (!data.hits || !data.hits.length) {
    box.innerHTML = '<div class="empty">没有命中任何切片</div>';
    return;
  }

  box.innerHTML = data.hits
    .map((h) => {
      const ranks = [];
      if (h.vectorRank) ranks.push(`向量#${h.vectorRank}`);
      if (h.textRank) ranks.push(`全文#${h.textRank}`);
      if (h.gramRank) ranks.push(`二元组#${h.gramRank}`);
      return `<article class="hit">
        <div class="hit-head">
          <span class="hit-title">${escapeHTML(h.headingPath || '（无标题路径）')}</span>
          <span class="hit-scores">doc ${h.docId} · ${ranks.join(' / ') || '—'} · fused ${h.fusedScore.toFixed(4)}</span>
        </div>
        <div class="hit-body">${escapeHTML(h.content)}</div>
      </article>`;
    })
    .join('');
}

async function search() {
  const query = el('queryInput').value.trim();
  if (!query) {
    el('searchMeta').textContent = '请输入查询内容';
    return;
  }

  const btn = el('btnSearch');
  btn.disabled = true;
  el('searchMeta').textContent = '检索中…';

  try {
    const data = await request(kbBase(), '/internal/kb/search', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenantId: tenantId(), query, limit: Number(el('topK').value) || 5 }),
    });
    renderHits(data);
  } catch (err) {
    el('searchMeta').textContent = '';
    el('hits').innerHTML = `<div class="msg err">检索失败：${escapeHTML(err.message)}</div>`;
  } finally {
    btn.disabled = false;
  }
}

// ---------- 事件绑定 ----------

function bind() {
  el('btnSettings').addEventListener('click', () => {
    el('settingsPanel').classList.toggle('hidden');
  });

  el('btnToken').addEventListener('click', issueToken);

  el('btnClearToken').addEventListener('click', () => {
    state.token = '';
    localStorage.removeItem(LS.token);
    renderTokenState();
    refreshDocs();
  });

  document.querySelectorAll('.tab').forEach((tab) => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.tab').forEach((t) => t.classList.remove('active'));
      tab.classList.add('active');
      const target = tab.dataset.tab;
      el('tab-kb').classList.toggle('hidden', target !== 'kb');
      el('tab-search').classList.toggle('hidden', target !== 'search');
      if (target === 'kb') refreshDocs();
    });
  });

  el('btnUpload').addEventListener('click', upload);
  el('btnRefresh').addEventListener('click', refreshDocs);
  el('statusFilter').addEventListener('change', refreshDocs);
  el('btnSearch').addEventListener('click', search);

  el('queryInput').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') search();
  });

  el('docRows').addEventListener('click', (e) => {
    const id = e.target?.dataset?.offline;
    if (id) offlineDoc(Number(id));
  });
}

(function init() {
  loadSettings();
  renderTokenState();
  bind();
  refreshDocs();
})();
