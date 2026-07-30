/* ── State ────────────────────────────────────────────── */
const state = {
  path: '',
  files: [],
  selected: new Set(),
  clipboard: null,  // { mode: 'copy'|'move', paths: [] }
  showHidden: false,
  isTermux: false,
  hasStorage: false,
  root: '',
  rootLabel: '~',
  quickPaths: [],
  searchOpen: false,
  fabOpen: false,
  ctxTarget: null,  // FileInfo of right-clicked file
  previewPath: '',
};

/* ── API ──────────────────────────────────────────────── */
const api = {
  async call(method, path, body) {
    const opts = { method };
    if (body !== undefined) {
      if (body instanceof FormData) {
        opts.body = body;
      } else if (typeof body === 'string') {
        opts.body = body;
      } else {
        opts.headers = { 'Content-Type': 'application/json' };
        opts.body = JSON.stringify(body);
      }
    }
    const res = await fetch(path, opts);
    return res.json();
  },
  get: (p)      => api.call('GET', p),
  post: (p, b)  => api.call('POST', p, b),
  del: (p, b)   => api.call('DELETE', p, b),

  list:     (path)             => api.get(`/api/list?path=${enc(path)}`),
  read:     (path)             => api.get(`/api/read?path=${enc(path)}`),
  write:    (path, text)       => api.post(`/api/write?path=${enc(path)}`, text),
  remove:   (paths)            => api.del('/api/delete', { paths }),
  rename:   (path, name)       => api.post('/api/rename', { path, name }),
  move:     (src, dst)         => api.post('/api/move', { src, dst }),
  copy:     (src, dst)         => api.post('/api/copy', { src, dst }),
  mkdir:    (path)             => api.post('/api/mkdir', { path }),
  touch:    (path)             => api.post('/api/touch', { path }),
  search:   (path, q)         => api.get(`/api/search?path=${enc(path)}&q=${enc(q)}`),
  zip:      (path, files, name) => api.post('/api/zip', { path, files, name }),
  unzip:    (path, dest)       => api.post('/api/unzip', { path, dest }),
  untar:    (path, dest)       => api.post('/api/untar', { path, dest }),
  info:     ()                 => api.get('/api/info'),
  setRoot:  (path)             => api.post('/api/root', { path }),
  share:    (path)             => api.post('/api/termux/share', { path }),
  clipboard: (text)            => api.post('/api/termux/clipboard', { text }),
  exec:     (cmd)              => api.post('/api/termux/exec', { cmd }),
};

const enc = encodeURIComponent;

/* ── Helpers ──────────────────────────────────────────── */
function formatSize(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024, units = ['B','KB','MB','GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(i ? 1 : 0) + ' ' + units[i];
}

function formatDate(iso) {
  const d = new Date(iso);
  const now = new Date();
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
}

function fileIcon(file) {
  if (file.is_dir) return '📁';
  const ext = (file.ext || '').toLowerCase();
  const map = {
    // images
    png:'🖼', jpg:'🖼', jpeg:'🖼', gif:'🖼', webp:'🖼', svg:'🖼', bmp:'🖼',
    // video
    mp4:'🎬', mkv:'🎬', avi:'🎬', mov:'🎬', webm:'🎬',
    // audio
    mp3:'🎵', flac:'🎵', ogg:'🎵', wav:'🎵', m4a:'🎵', aac:'🎵',
    // code
    js:'📝', ts:'📝', go:'📝', py:'📝', sh:'📝', bash:'📝', php:'📝',
    html:'📝', css:'📝', json:'📝', yaml:'📝', yml:'📝', toml:'📝',
    c:'📝', cpp:'📝', h:'📝', rs:'📝', java:'📝', kt:'📝',
    // docs
    pdf:'📄', doc:'📄', docx:'📄', ppt:'📄', pptx:'📄', xls:'📄', xlsx:'📄',
    // archives
    zip:'📦', tar:'📦', gz:'📦', bz2:'📦', xz:'📦', rar:'📦', '7z':'📦',
    // apk
    apk:'📱',
    // text
    txt:'📃', md:'📃', log:'📃', csv:'📃',
  };
  return map[ext] || '📄';
}

function isEditable(file) {
  if (!file || file.is_dir) return false;
  const ext = (file.ext || '').toLowerCase();
  // No extension (Makefile, LICENSE, README, …) → open in editor as text
  if (!ext) return true;
  const editable = ['txt','md','json','yaml','yml','toml','sh','bash',
    'js','ts','css','html','py','go','php','c','cpp','h','rs','java',
    'kt','conf','ini','env','log','csv','xml','sql',
    'gitignore','dockerignore','editorconfig','npmrc','prettierrc'];
  return editable.includes(ext);
}

function isImage(file) {
  if (!file || file.is_dir) return false;
  const ext = (file.ext || '').toLowerCase();
  return ['png','jpg','jpeg','gif','webp','bmp','svg','ico','heic','avif'].includes(ext);
}

function isZipArchive(file) {
  if (!file || file.is_dir) return false;
  return (file.ext || '').toLowerCase() === 'zip';
}

function isTarArchive(file) {
  if (!file || file.is_dir) return false;
  const name = (file.name || '').toLowerCase();
  const ext = (file.ext || '').toLowerCase();
  return name.endsWith('.tar.gz') || ext === 'tgz' || ext === 'tar';
}

function isArchive(file) {
  return isZipArchive(file) || isTarArchive(file);
}

function archiveDestPath(file) {
  const name = file.name || '';
  const lower = name.toLowerCase();
  let base = name;
  for (const suf of ['.tar.gz', '.tgz', '.tar', '.zip']) {
    if (lower.endsWith(suf)) {
      base = name.slice(0, name.length - suf.length);
      break;
    }
  }
  const parent = file.path.includes('/')
    ? file.path.slice(0, file.path.lastIndexOf('/'))
    : '';
  return joinPath(parent, base || (name + '_out'));
}

const PLACE_ICONS = {
  home: '🏠',
  sdcard: '💾',
  download: '⬇',
  dcim: '📷',
  shared: '📂',
  prefix: '⚙️',
};

function applyRootInfo(data) {
  if (!data) return;
  state.root = data.root || '';
  state.quickPaths = data.quick_paths || state.quickPaths || [];
  if (data.has_storage !== undefined) state.hasStorage = data.has_storage;
  const match = state.quickPaths.find(p => p.path === state.root);
  state.rootLabel = match ? match.label : (state.root ? state.root.split(/[/\\]/).filter(Boolean).pop() : '~');
}

function joinPath(...parts) {
  return parts.filter(Boolean).join('/').replace(/\/+/g, '/');
}

/* ── Toast ────────────────────────────────────────────── */
let toastContainer = null;
function toast(msg, type = '') {
  if (!toastContainer) {
    toastContainer = document.createElement('div');
    toastContainer.id = 'toast-container';
    document.body.appendChild(toastContainer);
  }
  const el = document.createElement('div');
  el.className = 'toast ' + type;
  el.textContent = msg;
  toastContainer.appendChild(el);
  setTimeout(() => el.remove(), 2800);
}

/* ── Modal ────────────────────────────────────────────── */
function showModal({ title, body, confirmText = 'OK', danger = false, onConfirm }) {
  const overlay = $('#modal-overlay');
  $('#modal-title').textContent = title;
  $('#modal-body').innerHTML = '';
  if (typeof body === 'string') {
    $('#modal-body').innerHTML = body;
  } else {
    $('#modal-body').appendChild(body);
  }
  $('#modal-confirm').textContent = confirmText;
  $('#modal-confirm').className = danger ? 'danger' : 'primary';
  overlay.classList.remove('hidden');

  const input = $('#modal-body').querySelector('input');
  if (input) { input.focus(); input.select(); }

  const doConfirm = () => {
    overlay.classList.add('hidden');
    onConfirm(input ? input.value.trim() : null);
  };

  $('#modal-confirm').onclick = doConfirm;
  $('#modal-cancel').onclick = () => overlay.classList.add('hidden');
  if (input) {
    input.onkeydown = (e) => { if (e.key === 'Enter') doConfirm(); };
  }
}

function promptModal(title, defaultVal = '') {
  return new Promise(resolve => {
    const input = document.createElement('input');
    input.type = 'text';
    input.value = defaultVal;
    showModal({ title, body: input, onConfirm: v => resolve(v || null) });
    $('#modal-cancel').onclick = () => {
      $('#modal-overlay').classList.add('hidden');
      resolve(null);
    };
  });
}

function confirmModal(title, msg, danger = false) {
  return new Promise(resolve => {
    showModal({
      title, danger,
      body: `<p>${msg}</p>`,
      confirmText: danger ? 'Delete' : 'OK',
      onConfirm: () => resolve(true),
    });
    $('#modal-cancel').onclick = () => {
      $('#modal-overlay').classList.add('hidden');
      resolve(false);
    };
  });
}

/* ── Breadcrumb ───────────────────────────────────────── */
function parentPath(path) {
  if (!path) return '';
  const parts = path.replace(/\\/g, '/').split('/').filter(Boolean);
  parts.pop();
  return parts.join('/');
}

function updateBackButton() {
  const btn = $('#btn-back');
  if (!btn) return;
  btn.classList.toggle('hidden', !state.path);
}

function goBack() {
  if (!state.path) return;
  navigate(parentPath(state.path));
}

function renderBreadcrumb(path) {
  const bc = $('#breadcrumb');
  bc.innerHTML = '';

  const norm = (path || '').replace(/\\/g, '/');
  const parts = norm ? norm.split('/').filter(Boolean) : [];
  const home = document.createElement('span');
  home.className = 'breadcrumb-item' + (parts.length === 0 ? ' active' : '');
  home.textContent = state.rootLabel || '~';
  home.onclick = () => navigate('');
  bc.appendChild(home);

  parts.forEach((part, i) => {
    const sep = document.createElement('span');
    sep.className = 'breadcrumb-sep';
    sep.textContent = '/';
    bc.appendChild(sep);

    const el = document.createElement('span');
    el.className = 'breadcrumb-item' + (i === parts.length - 1 ? ' active' : '');
    el.textContent = part;
    const targetPath = parts.slice(0, i + 1).join('/');
    el.onclick = () => navigate(targetPath);
    bc.appendChild(el);
  });

  updateBackButton();

  // Scroll breadcrumb to end
  bc.scrollLeft = bc.scrollWidth;
}

/* ── File list rendering ──────────────────────────────── */
function renderFiles(files) {
  const list = $('#file-list');
  list.innerHTML = '';

  // Parent directory row (when not at root)
  if (state.path) {
    const up = document.createElement('div');
    up.className = 'file-row parent-row';
    up.innerHTML = `
      <div class="file-checkbox" style="visibility:hidden"></div>
      <div class="file-icon">⬆️</div>
      <div class="file-info">
        <div class="file-name">..</div>
        <div class="file-meta"><span>Parent folder</span></div>
      </div>
    `;
    up.addEventListener('click', goBack);
    list.appendChild(up);
  }

  if (files.length === 0) {
    $('#empty-state').classList.toggle('hidden', !!state.path);
    if (!state.path) return;
    // Still show parent row when empty subfolder
    return;
  }
  $('#empty-state').classList.add('hidden');

  files.forEach(file => {
    const row = document.createElement('div');
    row.className = 'file-row' + (state.selected.has(file.path) ? ' selected' : '');
    row.dataset.path = file.path;

    row.innerHTML = `
      <div class="file-checkbox">${state.selected.has(file.path) ? '✓' : ''}</div>
      <div class="file-icon">${fileIcon(file)}</div>
      <div class="file-info">
        <div class="file-name">${escHtml(file.name)}</div>
        <div class="file-meta">
          ${file.is_dir ? '<span>Folder</span>' : `<span>${formatSize(file.size)}</span>`}
          <span>${formatDate(file.mod_time)}</span>
        </div>
      </div>
      <div class="file-actions">
        <button class="file-more icon-btn" title="More">⋯</button>
      </div>
    `;

    // Checkbox → always toggle selection
    row.querySelector('.file-checkbox').addEventListener('click', (e) => {
      e.stopPropagation();
      e.preventDefault();
      toggleSelect(file.path);
    });

    // Tap row → open / select (not when tapping checkbox or more)
    row.addEventListener('click', (e) => {
      if (e.target.closest('.file-more') || e.target.closest('.file-checkbox')) return;
      if (state.selected.size > 0) {
        toggleSelect(file.path);
        return;
      }
      openFile(file);
    });

    // Long press → select (ignore when starting on checkbox)
    let longTimer;
    row.addEventListener('pointerdown', (e) => {
      if (e.target.closest('.file-checkbox') || e.target.closest('.file-more')) return;
      longTimer = setTimeout(() => toggleSelect(file.path), 400);
    });
    row.addEventListener('pointerup', () => clearTimeout(longTimer));
    row.addEventListener('pointermove', () => clearTimeout(longTimer));
    row.addEventListener('pointercancel', () => clearTimeout(longTimer));

    // Context menu button
    row.querySelector('.file-more').addEventListener('click', (e) => {
      e.stopPropagation();
      showCtxMenu(e, file);
    });

    // Right-click
    row.addEventListener('contextmenu', (e) => {
      e.preventDefault();
      showCtxMenu(e, file);
    });

    list.appendChild(row);
  });

  updateSelectAllCheckbox();
}

function escHtml(str) {
  return str.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

/* ── Navigation ───────────────────────────────────────── */
async function navigate(path) {
  state.path = path;
  state.selected.clear();
  updateSelectionBar();
  setLoading(true);

  const res = await api.list(path);
  setLoading(false);

  if (!res.ok) { toast(res.error, 'error'); return; }

  state.files = res.data.files || [];
  renderBreadcrumb(state.path);
  renderFiles(state.files);
}

function setLoading(on) {
  $('#loading').classList.toggle('hidden', !on);
  $('#file-list').classList.toggle('hidden', on);
}

function openFile(file) {
  if (file.is_dir) {
    navigate(file.path);
    return;
  }
  if (isImage(file)) {
    openPreview(file);
    return;
  }
  if (isArchive(file)) {
    doExtract(file);
    return;
  }
  if (isEditable(file)) {
    openEditor(file);
    return;
  }
  // Non-editable: trigger download
  window.location.href = `/api/download?path=${enc(file.path)}`;
}

/* ── Quick paths ──────────────────────────────────────── */
function openPlaces() {
  const list = $('#places-list');
  list.innerHTML = '';
  const paths = (state.quickPaths || []).filter(p => p.available);
  if (!paths.length) {
    list.innerHTML = '<p style="padding:12px;color:var(--text2);font-size:13px">No quick paths available</p>';
  } else {
    paths.forEach(p => {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'place-item' + (p.path === state.root ? ' active' : '');
      btn.innerHTML = `
        <span class="place-icon">${PLACE_ICONS[p.id] || '📁'}</span>
        <span class="place-meta">
          <div class="place-label">${escHtml(p.label)}</div>
          <div class="place-path">${escHtml(p.path)}</div>
        </span>
      `;
      btn.onclick = () => switchRoot(p.path);
      list.appendChild(btn);
    });
  }
  $('#places-current').textContent = 'Current: ' + (state.root || '—');
  $('#places-overlay').classList.remove('hidden');
}

function closePlaces() {
  $('#places-overlay').classList.add('hidden');
}

async function switchRoot(path) {
  if (path === state.root) {
    closePlaces();
    navigate('');
    return;
  }
  const res = await api.setRoot(path);
  if (!res.ok) {
    toast(res.error || 'Failed to switch root', 'error');
    return;
  }
  applyRootInfo(res.data);
  closePlaces();
  toast('Browsing ' + state.rootLabel, 'success');
  navigate('');
}

/* ── Image preview ────────────────────────────────────── */
function openPreview(file) {
  state.previewPath = file.path;
  $('#preview-filename').textContent = file.name;
  const url = `/api/download?path=${enc(file.path)}&inline=1`;
  const img = $('#preview-img');
  img.onload = () => {};
  img.onerror = () => toast('Failed to load image', 'error');
  img.src = url;
  $('#preview-download').href = `/api/download?path=${enc(file.path)}`;
  $('#preview-download').setAttribute('download', file.name);
  $('#preview-overlay').classList.remove('hidden');
}

function closePreview() {
  $('#preview-overlay').classList.add('hidden');
  $('#preview-img').removeAttribute('src');
  state.previewPath = '';
}

function isPreviewOpen() {
  return !$('#preview-overlay').classList.contains('hidden');
}

/* ── Selection ────────────────────────────────────────── */
function toggleSelect(path) {
  if (state.selected.has(path)) {
    state.selected.delete(path);
  } else {
    state.selected.add(path);
  }
  // Update row UI
  const row = document.querySelector(`[data-path="${CSS.escape(path)}"]`);
  if (row) {
    row.classList.toggle('selected', state.selected.has(path));
    row.querySelector('.file-checkbox').textContent = state.selected.has(path) ? '✓' : '';
  }
  updateSelectionBar();
}

function clearSelection() {
  state.selected.clear();
  document.querySelectorAll('.file-row.selected').forEach(r => {
    r.classList.remove('selected');
    const cb = r.querySelector('.file-checkbox');
    if (cb) cb.textContent = '';
  });
  updateSelectionBar();
}

function selectAllFiles() {
  (state.files || []).forEach(f => {
    if (!f || !f.path) return;
    state.selected.add(f.path);
    const row = document.querySelector(`[data-path="${CSS.escape(f.path)}"]`);
    if (row) {
      row.classList.add('selected');
      const cb = row.querySelector('.file-checkbox');
      if (cb) cb.textContent = '✓';
    }
  });
  updateSelectionBar();
}

function toggleSelectAll() {
  const total = (state.files || []).length;
  if (!total) return;
  if (state.selected.size >= total) clearSelection();
  else selectAllFiles();
}

function updateSelectAllCheckbox() {
  const cb = $('#select-all-checkbox');
  const countBtn = $('#selection-count');
  const master = $('#btn-select-all');
  const total = (state.files || []).length;
  const n = state.selected.size;
  const allOn = total > 0 && n >= total;
  const partial = n > 0 && n < total;

  if (cb) {
    cb.classList.toggle('partial', partial);
    if (allOn) {
      cb.textContent = '✓';
      cb.classList.remove('partial');
    } else if (partial) {
      cb.textContent = '−';
    } else {
      cb.textContent = '';
      cb.classList.remove('partial');
    }
  }
  if (countBtn) {
    countBtn.textContent = total ? `${n}/${total}` : `${n} selected`;
    countBtn.title = allOn ? 'Deselect all' : 'Select all';
  }
  if (master) {
    master.title = allOn ? 'Deselect all' : 'Select all';
  }
}

function updateSelectionBar() {
  const bar = $('#selection-bar');
  if (state.selected.size > 0) {
    bar.classList.remove('hidden');
  } else {
    bar.classList.add('hidden');
  }
  updateSelectAllCheckbox();
}

/* ── Context menu ─────────────────────────────────────── */
function showCtxMenu(e, file) {
  state.ctxTarget = file;
  const menu = $('#ctx-menu');
  menu.classList.remove('hidden');

  // Hide items not applicable
  menu.querySelector('[data-ctx="edit"]').style.display = isEditable(file) ? '' : 'none';
  menu.querySelector('[data-ctx="preview"]').style.display = isImage(file) ? '' : 'none';
  menu.querySelector('[data-ctx="extract"]').style.display = isArchive(file) ? '' : 'none';
  menu.querySelector('[data-ctx="open"]').style.display = file.is_dir ? 'none' : '';
  menu.querySelector('[data-ctx="share"]').style.display = state.isTermux ? '' : 'none';

  const x = Math.min(e.clientX, window.innerWidth - 180);
  const y = Math.min(e.clientY, window.innerHeight - 280);
  menu.style.left = x + 'px';
  menu.style.top = y + 'px';
}

function hideCtxMenu() { $('#ctx-menu').classList.add('hidden'); }

/* ── Editor (Ace) ─────────────────────────────────────── */
let aceEditor = null;
let editorLang = 'text';
let editorSavedValue = '';
let editorFilePath = '';
let editorFontSize = 13;
let editorWrap = false;
let editorFindTimer = null;
let editorFindIndex = 0;
let editorFindTotal = 0;

function aceModeFor(file) {
  const name = (file.name || '').toLowerCase();
  const ext = (file.ext || '').toLowerCase();
  if (name === 'dockerfile' || name.startsWith('dockerfile.')) return 'ace/mode/dockerfile';
  if (name === 'makefile' || name.startsWith('makefile.') || name === 'gnumakefile') return 'ace/mode/makefile';
  const map = {
    md: 'ace/mode/markdown', markdown: 'ace/mode/markdown',
    json: 'ace/mode/json',
    yaml: 'ace/mode/yaml', yml: 'ace/mode/yaml',
    toml: 'ace/mode/toml',
    sh: 'ace/mode/sh', bash: 'ace/mode/sh', zsh: 'ace/mode/sh',
    js: 'ace/mode/javascript', mjs: 'ace/mode/javascript', cjs: 'ace/mode/javascript',
    ts: 'ace/mode/typescript',
    css: 'ace/mode/css',
    html: 'ace/mode/html', htm: 'ace/mode/html',
    xml: 'ace/mode/xml',
    py: 'ace/mode/python',
    go: 'ace/mode/golang',
    php: 'ace/mode/php',
    c: 'ace/mode/c_cpp', h: 'ace/mode/c_cpp', cpp: 'ace/mode/c_cpp', cc: 'ace/mode/c_cpp', hpp: 'ace/mode/c_cpp',
    rs: 'ace/mode/rust',
    java: 'ace/mode/java',
    kt: 'ace/mode/kotlin', kts: 'ace/mode/kotlin',
    sql: 'ace/mode/sql',
    ini: 'ace/mode/ini', conf: 'ace/mode/ini', cfg: 'ace/mode/ini', env: 'ace/mode/sh',
    txt: 'ace/mode/text', log: 'ace/mode/text',
  };
  return map[ext] || 'ace/mode/text';
}

function langLabel(mode) {
  return (mode || '').replace(/^ace\/mode\//, '') || 'text';
}

function isEditorOpen() {
  return !$('#editor-overlay').classList.contains('hidden');
}

function isEditorDirty() {
  return !!aceEditor && aceEditor.getValue() !== editorSavedValue;
}

function updateEditorDirty() {
  const dirty = isEditorDirty();
  $('#editor-dirty').classList.toggle('hidden', !dirty);
  $('#editor-filename').classList.toggle('dirty', dirty);
}

function updateEditorStatus() {
  if (!aceEditor) return;
  const pos = aceEditor.getCursorPosition();
  $('#editor-pos').textContent = `Ln ${pos.row + 1}, Col ${pos.column + 1}`;
  const range = aceEditor.getSelectionRange();
  const sel = aceEditor.session.getTextRange(range);
  $('#editor-sel').textContent = sel ? `${sel.length} selected` : '';
  $('#editor-lang').textContent = editorLang;
  updateEditorDirty();
}

function applyEditorFont() {
  if (aceEditor) aceEditor.setFontSize(editorFontSize);
}

function applyEditorWrap() {
  if (aceEditor) aceEditor.setOption('wrap', editorWrap);
  $('#editor-wrap').classList.toggle('active', editorWrap);
}

function ensureAce() {
  if (aceEditor) return aceEditor;
  if (typeof ace === 'undefined') {
    toast('Ace editor failed to load', 'error');
    return null;
  }
  ace.config.set('basePath', '/vendor/ace');
  ace.config.set('modePath', '/vendor/ace');
  ace.config.set('themePath', '/vendor/ace');
  ace.config.set('workerPath', '/vendor/ace');

  aceEditor = ace.edit('editor-ace');
  aceEditor.setTheme('ace/theme/one_dark');
  aceEditor.setOptions({
    fontSize: editorFontSize,
    showPrintMargin: false,
    wrap: editorWrap,
    useSoftTabs: true,
    tabSize: 2,
    navigateWithinSoftTabs: true,
    scrollPastEnd: 0.25,
    useWorker: false,
    fixedWidthGutter: true,
    highlightActiveLine: true,
    animatedScroll: false,
  });
  aceEditor.session.setUseWrapMode(editorWrap);

  try { aceEditor.commands.removeCommand('find'); } catch (_) {}
  try { aceEditor.commands.removeCommand('replace'); } catch (_) {}

  aceEditor.commands.addCommand({
    name: 'tmSave',
    bindKey: { win: 'Ctrl-S', mac: 'Command-S' },
    exec: () => { saveEditor(); },
  });
  aceEditor.commands.addCommand({
    name: 'tmFind',
    bindKey: { win: 'Ctrl-F', mac: 'Command-F' },
    exec: () => { toggleFindBar(true); },
  });
  aceEditor.commands.addCommand({
    name: 'tmGoto',
    bindKey: { win: 'Ctrl-G', mac: 'Command-G' },
    exec: () => { goToLine(); },
  });

  aceEditor.on('change', () => {
    updateEditorDirty();
    if (!$('#editor-findbar').classList.contains('hidden')) scheduleFindCount();
  });
  aceEditor.selection.on('changeCursor', updateEditorStatus);
  aceEditor.selection.on('changeSelection', updateEditorStatus);

  return aceEditor;
}

function editorUndo() {
  if (!aceEditor) return;
  aceEditor.undo();
  aceEditor.focus();
  updateEditorStatus();
}

function editorRedo() {
  if (!aceEditor) return;
  aceEditor.redo();
  aceEditor.focus();
  updateEditorStatus();
}

function findOpts(extra) {
  return Object.assign({
    wrap: true,
    caseSensitive: false,
    wholeWord: false,
    regExp: false,
    preventScroll: false,
  }, extra || {});
}

function scheduleFindCount() {
  clearTimeout(editorFindTimer);
  editorFindTimer = setTimeout(updateFindCount, 120);
}

function updateFindCount() {
  const el = $('#editor-find-count');
  const q = $('#editor-find-input').value;
  if (!q || !aceEditor) {
    el.textContent = '';
    editorFindTotal = 0;
    return;
  }
  const len = aceEditor.session.getValue().length;
  if (len > 2_000_000) {
    el.textContent = 'â€¦';
    return;
  }
  try {
    const Search = ace.require('ace/search').Search;
    const search = new Search();
    search.set({ needle: q, caseSensitive: false, regExp: false, wholeWord: false });
    const ranges = search.findAll(aceEditor.session) || [];
    editorFindTotal = ranges.length;
    if (!ranges.length) {
      el.textContent = '0';
      editorFindIndex = 0;
      return;
    }
    const cur = aceEditor.getSelectionRange();
    let idx = ranges.findIndex(r =>
      r.start.row === cur.start.row && r.start.column === cur.start.column);
    if (idx < 0) idx = 0;
    editorFindIndex = idx;
    el.textContent = `${idx + 1}/${ranges.length}`;
  } catch (_) {
    el.textContent = '';
  }
}

function runFind(dir) {
  const q = $('#editor-find-input').value;
  if (!q || !aceEditor) {
    updateFindCount();
    return;
  }
  const backwards = dir < 0;
  aceEditor.find(q, findOpts({ backwards, skipCurrent: dir !== 0 }));
  updateFindCount();
  updateEditorStatus();
}

function replaceOne() {
  const q = $('#editor-find-input').value;
  const rep = $('#editor-replace-input').value;
  if (!q || !aceEditor) return;
  const sel = aceEditor.getSelectedText();
  if (sel && sel.toLowerCase() === q.toLowerCase()) {
    aceEditor.session.replace(aceEditor.getSelectionRange(), rep);
  } else {
    const found = aceEditor.find(q, findOpts({ skipCurrent: false }));
    if (!found) { toast('No matches', 'error'); updateFindCount(); return; }
    aceEditor.session.replace(aceEditor.getSelectionRange(), rep);
  }
  aceEditor.find(q, findOpts({ skipCurrent: false }));
  updateFindCount();
  updateEditorStatus();
}

function replaceAll() {
  const q = $('#editor-find-input').value;
  const rep = $('#editor-replace-input').value;
  if (!q || !aceEditor) return;
  // Ensure needle is current, then replace all
  aceEditor.find(q, findOpts({ skipCurrent: false, preventScroll: true }));
  const n = aceEditor.replaceAll(rep);
  updateFindCount();
  updateEditorStatus();
  if (!n) toast('No matches', 'error');
  else toast(`Replaced ${n}`, 'success');
}

function toggleFindBar(force) {
  const bar = $('#editor-findbar');
  const open = force !== undefined ? force : bar.classList.contains('hidden');
  bar.classList.toggle('hidden', !open);
  $('#editor-find-btn').classList.toggle('active', open);
  if (open) {
    if (aceEditor) {
      const sel = aceEditor.getSelectedText();
      if (sel && !sel.includes('\n')) $('#editor-find-input').value = sel;
    }
    $('#editor-find-input').focus();
    $('#editor-find-input').select();
    runFind(0);
    requestAnimationFrame(() => { if (aceEditor) aceEditor.resize(); });
  } else if (aceEditor) {
    aceEditor.focus();
    requestAnimationFrame(() => aceEditor.resize());
  }
}

async function goToLine() {
  if (!aceEditor) return;
  const raw = await promptModal('Go to line', String(aceEditor.getCursorPosition().row + 1));
  if (!raw) return;
  const n = parseInt(raw, 10);
  if (!n || n < 1) return;
  const max = aceEditor.session.getLength();
  const line = Math.min(n, max);
  aceEditor.gotoLine(line, 0, true);
  aceEditor.focus();
  updateEditorStatus();
}

async function saveEditor() {
  if (!editorFilePath || !aceEditor) return false;
  const text = aceEditor.getValue();
  const r = await api.write(editorFilePath, text);
  if (r.ok) {
    editorSavedValue = text;
    updateEditorDirty();
    toast('Saved', 'success');
    return true;
  }
  toast(r.error, 'error');
  return false;
}

async function closeEditor() {
  if (isEditorDirty()) {
    const ok = await new Promise(resolve => {
      showModal({
        title: 'Unsaved changes',
        body: '<p>Discard unsaved changes?</p>',
        confirmText: 'Discard',
        danger: true,
        onConfirm: () => resolve(true),
      });
      $('#modal-cancel').onclick = () => {
        $('#modal-overlay').classList.add('hidden');
        resolve(false);
      };
    });
    if (!ok) return;
  }
  clearTimeout(editorFindTimer);
  toggleFindBar(false);
  $('#editor-overlay').classList.add('hidden');
}

function bindEditorChrome() {
  if (bindEditorChrome.done) return;
  bindEditorChrome.done = true;

  $('#editor-back').onclick = () => closeEditor();
  $('#editor-save').onclick = () => saveEditor();
  $('#editor-undo').onclick = () => editorUndo();
  $('#editor-redo').onclick = () => editorRedo();
  $('#editor-find-btn').onclick = () => toggleFindBar();
  $('#editor-find-close').onclick = () => toggleFindBar(false);
  $('#editor-find-next').onclick = () => runFind(1);
  $('#editor-find-prev').onclick = () => runFind(-1);
  $('#editor-replace-one').onclick = () => replaceOne();
  $('#editor-replace-all').onclick = () => replaceAll();
  $('#editor-find-input').oninput = () => {
    editorFindIndex = 0;
    runFind(0);
  };
  $('#editor-find-input').onkeydown = (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      runFind(e.shiftKey ? -1 : 1);
    }
  };
  $('#editor-replace-input').onkeydown = (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (e.shiftKey) replaceOne();
      else runFind(1);
    }
  };
  $('#editor-goto').onclick = () => goToLine();
  $('#editor-wrap').onclick = () => {
    editorWrap = !editorWrap;
    applyEditorWrap();
  };
  $('#editor-font-inc').onclick = () => {
    editorFontSize = Math.min(22, editorFontSize + 1);
    applyEditorFont();
  };
  $('#editor-font-dec').onclick = () => {
    editorFontSize = Math.max(11, editorFontSize - 1);
    applyEditorFont();
  };

  window.addEventListener('resize', () => {
    if (isEditorOpen() && aceEditor) aceEditor.resize();
  });
}

async function openEditor(file) {
  const res = await api.read(file.path);
  if (!res.ok) { toast('Cannot read file', 'error'); return; }

  bindEditorChrome();
  const ed = ensureAce();
  if (!ed) return;

  const mode = aceModeFor(file);
  editorLang = langLabel(mode);
  editorFilePath = file.path;
  editorSavedValue = res.data;
  editorFindIndex = 0;
  editorFindTotal = 0;

  $('#editor-filename').textContent = file.name;
  $('#editor-overlay').classList.remove('hidden');
  $('#editor-findbar').classList.add('hidden');
  $('#editor-find-btn').classList.remove('active');

  ed.setValue(res.data, -1);
  ed.session.setMode(mode);
  ed.session.setUndoManager(new ace.UndoManager());
  applyEditorFont();
  applyEditorWrap();
  updateEditorDirty();
  updateEditorStatus();

  requestAnimationFrame(() => {
    ed.resize();
    ed.focus();
    ed.gotoLine(1, 0, false);
  });
}


/* ── Upload ───────────────────────────────────────────── */
async function uploadFiles(files) {
  for (const file of files) {
    const fd = new FormData();
    fd.append('file', file);
    const res = await fetch(`/api/upload?path=${enc(state.path)}`, {
      method: 'POST', body: fd,
    }).then(r => r.json());
    if (res.ok) {
      toast(`Uploaded ${file.name}`, 'success');
    } else {
      toast(`Upload failed: ${res.error}`, 'error');
    }
  }
  navigate(state.path);
}

/* ── Actions ──────────────────────────────────────────── */
async function doDelete(paths) {
  const ok = await confirmModal('Delete', `Delete ${paths.length} item(s)? This cannot be undone.`, true);
  if (!ok) return;
  const res = await api.remove(paths);
  if (res.ok) { toast('Deleted', 'success'); navigate(state.path); }
  else toast(res.error, 'error');
}

async function doRename(file) {
  const name = await promptModal('Rename', file.name);
  if (!name || name === file.name) return;
  const res = await api.rename(file.path, name);
  if (res.ok) { toast('Renamed', 'success'); navigate(state.path); }
  else toast(res.error, 'error');
}

async function doMkdir() {
  const name = await promptModal('New Folder', 'New Folder');
  if (!name) return;
  const res = await api.mkdir(joinPath(state.path, name));
  if (res.ok) { toast('Folder created', 'success'); navigate(state.path); }
  else toast(res.error, 'error');
}

async function doTouch() {
  const name = await promptModal('New File', 'untitled.txt');
  if (!name) return;
  const res = await api.touch(joinPath(state.path, name));
  if (res.ok) { toast('File created', 'success'); navigate(state.path); }
  else toast(res.error, 'error');
}

async function doZip(paths) {
  const name = await promptModal('Archive name', `archive_${Date.now()}.zip`);
  if (!name) return;
  const files = paths.map(p => p.split('/').pop());
  const res = await api.zip(state.path, files, name);
  if (res.ok) { toast(`Created ${name}`, 'success'); navigate(state.path); }
  else toast(res.error, 'error');
}

async function doExtract(file) {
  if (!isArchive(file)) return;
  const dest = archiveDestPath(file);
  const ok = await confirmModal('Extract', `Extract ${file.name} → ${dest.split('/').pop() || dest}?`);
  if (!ok) return;
  setLoading(true);
  const res = isZipArchive(file)
    ? await api.unzip(file.path, dest)
    : await api.untar(file.path, dest);
  setLoading(false);
  if (res.ok) {
    toast('Extracted', 'success');
    navigate(state.path);
  } else {
    toast(res.error || 'Extract failed', 'error');
  }
}

/* ── Search ───────────────────────────────────────────── */
let searchTimer;
async function doSearch(q) {
  clearTimeout(searchTimer);
  if (!q) { navigate(state.path); return; }
  searchTimer = setTimeout(async () => {
    setLoading(true);
    const res = await api.search(state.path, q);
    setLoading(false);
    if (!res.ok) { toast(res.error, 'error'); return; }
    state.files = res.data || [];
    renderFiles(state.files);
  }, 300);
}

/* ── DOM helpers ──────────────────────────────────────── */
function $(sel) { return document.querySelector(sel); }

/* ── Init ─────────────────────────────────────────────── */
async function init() {
  // Load server info
  const info = await api.info();
  if (info.ok) {
    state.isTermux = info.data.is_termux;
    state.hasStorage = info.data.has_storage;
    state.showHidden = info.data.show_hidden;
    applyRootInfo(info.data);
  }

  // Initial load
  navigate('');

  // Header buttons
  $('#btn-back').onclick = goBack;
  $('#btn-places').onclick = openPlaces;
  $('#places-close').onclick = closePlaces;
  $('#places-overlay').onclick = (e) => {
    if (e.target === $('#places-overlay')) closePlaces();
  };
  $('#preview-close').onclick = closePreview;
  $('#preview-body').onclick = (e) => {
    if (e.target === $('#preview-body')) closePreview();
  };

  $('#btn-search').onclick = () => {
    state.searchOpen = !state.searchOpen;
    $('#search-bar').classList.toggle('hidden', !state.searchOpen);
    if (state.searchOpen) { $('#search-input').focus(); $('#search-input').value = ''; }
    else navigate(state.path);
  };
  $('#btn-search-close').onclick = () => {
    state.searchOpen = false;
    $('#search-bar').classList.add('hidden');
    navigate(state.path);
  };
  $('#search-input').oninput = (e) => doSearch(e.target.value.trim());

  $('#btn-hidden').onclick = () => {
    state.showHidden = !state.showHidden;
    $('#btn-hidden').classList.toggle('active', state.showHidden);
    navigate(state.path);
  };

  $('#btn-upload').onclick = () => $('#upload-input').click();
  $('#upload-input').onchange = (e) => {
    if (e.target.files.length) uploadFiles([...e.target.files]);
    e.target.value = '';
  };

  // FAB
  $('#fab-main').onclick = () => {
    state.fabOpen = !state.fabOpen;
    $('#fab-main').classList.toggle('open', state.fabOpen);
    $('#fab-menu').classList.toggle('hidden', !state.fabOpen);
  };
  $('#fab-menu').querySelectorAll('[data-fab]').forEach(btn => {
    btn.onclick = () => {
      state.fabOpen = false;
      $('#fab-main').classList.remove('open');
      $('#fab-menu').classList.add('hidden');
      if (btn.dataset.fab === 'mkdir') doMkdir();
      if (btn.dataset.fab === 'touch') doTouch();
    };
  });

  // Selection bar: master checkbox + count both toggle select-all
  $('#btn-select-all').onclick = () => toggleSelectAll();
  $('#selection-count').onclick = () => toggleSelectAll();
  $('#btn-deselect').onclick = () => clearSelection();

  document.querySelectorAll('[data-action]').forEach(btn => {
    btn.onclick = async () => {
      const paths = [...state.selected];
      if (btn.dataset.action === 'delete-sel') await doDelete(paths);
      if (btn.dataset.action === 'zip-sel') await doZip(paths);
      if (btn.dataset.action === 'copy-sel') {
        state.clipboard = { mode: 'copy', paths };
        toast(`${paths.length} item(s) copied`, 'success');
        state.selected.clear();
        updateSelectionBar();
      }
      if (btn.dataset.action === 'move-sel') {
        state.clipboard = { mode: 'move', paths };
        toast(`${paths.length} item(s) cut`, 'success');
        state.selected.clear();
        updateSelectionBar();
      }
    };
  });

  // Context menu actions
  $('#ctx-menu').querySelectorAll('[data-ctx]').forEach(btn => {
    btn.onclick = async () => {
      const file = state.ctxTarget;
      hideCtxMenu();
      if (!file) return;

      switch (btn.dataset.ctx) {
        case 'open':
          openFile(file);
          break;
        case 'preview':
          openPreview(file);
          break;
        case 'edit':
          openEditor(file);
          break;
        case 'download':
          window.location.href = `/api/download?path=${enc(file.path)}`;
          break;
        case 'rename':
          await doRename(file);
          break;
        case 'copy':
          await api.clipboard(file.path);
          navigator.clipboard?.writeText(file.path).catch(() => {});
          toast('Path copied', 'success');
          break;
        case 'share':
          await api.share(file.path);
          break;
        case 'zip-one':
          await doZip([file.path]);
          break;
        case 'extract':
          await doExtract(file);
          break;
        case 'delete':
          await doDelete([file.path]);
          break;
      }
    };
  });

  // Close context menu on outside click
  document.addEventListener('click', (e) => {
    if (!$('#ctx-menu').contains(e.target)) hideCtxMenu();
  });

  // Keyboard shortcuts
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      hideCtxMenu();
      if (isPreviewOpen()) {
        closePreview();
        return;
      }
      if (!$('#places-overlay').classList.contains('hidden')) {
        closePlaces();
        return;
      }
      if (isEditorOpen()) {
        if (!$('#editor-findbar').classList.contains('hidden')) {
          toggleFindBar(false);
          return;
        }
        closeEditor();
        return;
      }
      if (!$('#modal-overlay').classList.contains('hidden')) {
        $('#modal-overlay').classList.add('hidden');
        return;
      }
      if (state.path) {
        goBack();
      }
    }
  });

  // Drag-and-drop upload
  document.addEventListener('dragover', (e) => e.preventDefault());
  document.addEventListener('drop', (e) => {
    e.preventDefault();
    const files = [...e.dataTransfer.files];
    if (files.length) uploadFiles(files);
  });
}

document.addEventListener('DOMContentLoaded', init);
