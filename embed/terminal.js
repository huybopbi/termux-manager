/* ═══════════════════════════════════════════════════════
   termux-manager — Terminal emulator
   Uses xterm.js (v5) + FitAddon + WebSocket PTY backend
   ═══════════════════════════════════════════════════════ */

const Term = (() => {
  /* ── State ────────────────────────────────────────── */
  let tabs = [];       // { id, sessionId, xterm, fitAddon, ws, mountEl, tabEl, dead }
  let activeId = null;
  let tabCounter = 0;
  let panelOpen = false;
  let expanded = false;
  let restoring = false;
  const STORE_KEY = 'tm-term-sessions';
  const standalone = () => document.body.classList.contains('term-only');

  /* ── DOM refs ─────────────────────────────────────── */
  const panel      = () => document.getElementById('term-panel');
  const container  = () => document.getElementById('term-container');
  const tabsEl     = () => document.getElementById('term-tabs');
  const handle     = () => document.getElementById('term-resize-handle');
  const app        = () => document.getElementById('app');

  /* ── Persist session ids across refresh ───────────── */
  function loadStoredIds() {
    try { return JSON.parse(localStorage.getItem(STORE_KEY) || '[]'); }
    catch { return []; }
  }
  function saveStoredIds(ids) {
    localStorage.setItem(STORE_KEY, JSON.stringify([...new Set(ids)]));
  }
  function rememberSession(sid) {
    if (!sid) return;
    const ids = loadStoredIds().filter(x => x !== sid);
    ids.push(sid);
    saveStoredIds(ids);
  }
  function forgetSession(sid) {
    if (!sid) return;
    saveStoredIds(loadStoredIds().filter(x => x !== sid));
  }

  /* ── xterm theme ──────────────────────────────────── */
  const THEME = {
    background:    '#0d1117',
    foreground:    '#e6edf3',
    cursor:        '#58a6ff',
    cursorAccent:  '#0d1117',
    selectionBackground: 'rgba(88,166,255,0.25)',
    black:   '#0d1117', brightBlack:   '#6e7681',
    red:     '#ff7b72', brightRed:     '#ffa198',
    green:   '#3fb950', brightGreen:   '#56d364',
    yellow:  '#d29922', brightYellow:  '#e3b341',
    blue:    '#58a6ff', brightBlue:    '#79c0ff',
    magenta: '#bc8cff', brightMagenta: '#d2a8ff',
    cyan:    '#39c5cf', brightCyan:    '#56d4dd',
    white:   '#b1bac4', brightWhite:   '#f0f6fc',
  };

  function setTabLabel(tab, sid) {
    const label = tab.tabEl.querySelector('.term-tab-label');
    if (!label) return;
    label.textContent = sid ? `bash ${sid.slice(0, 4)}` : `bash ${tab.id}`;
  }

  function bindSocket(tab, sessionId) {
    const wsProto = location.protocol === 'https:' ? 'wss' : 'ws';
    const q = sessionId ? `?id=${encodeURIComponent(sessionId)}` : '';
    const ws = new WebSocket(`${wsProto}://${location.host}/ws/terminal${q}`);
    tab.ws = ws;
    tab.dead = false;

    const overlay = document.createElement('div');
    overlay.className = 'term-connecting';
    overlay.innerHTML = '<div class="spinner"></div><span>Connecting…</span>';
    tab.mountEl.appendChild(overlay);

    ws.onopen = () => {
      overlay.remove();
      try { tab.fitAddon.fit(); } catch (_) {}
      sendResize(ws, tab.xterm);
      tab.xterm.focus();
    };

    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data);
        switch (msg.type) {
          case 'ready':
            if (tab.sessionId && tab.sessionId !== msg.id) forgetSession(tab.sessionId);
            tab.sessionId = msg.id;
            rememberSession(msg.id);
            setTabLabel(tab, msg.id);
            break;
          case 'history':
            tab.xterm.reset();
            if (msg.data) tab.xterm.write(msg.data);
            break;
          case 'output':
            tab.xterm.write(msg.data || '');
            break;
          case 'error':
            tab.xterm.write('\r\n\x1b[31m' + (msg.data || 'error') + '\x1b[0m');
            break;
          default:
            if (msg.data) tab.xterm.write(msg.data);
        }
      } catch {
        tab.xterm.write(ev.data);
      }
    };

    ws.onerror = () => {
      overlay.remove();
      tab.xterm.writeln('\r\n\x1b[31mWebSocket error — is the server running?\x1b[0m');
    };

    ws.onclose = () => {
      overlay.remove();
      if (tab.dead) return;
      tab.xterm.writeln('\r\n\x1b[33m[Disconnected — PTY still running. Press any key to reconnect.]\x1b[0m');
    };
  }

  /* ── Create a new terminal tab (optional: reattach) ─ */
  function newTab(sessionId) {
    const id = ++tabCounter;

    const xterm = new Terminal({
      theme: THEME,
      fontFamily: '"JetBrainsMono Nerd Font Mono", "JetBrains Mono", "Cascadia Code", monospace',
      fontSize: 13,
      lineHeight: 1.4,
      cursorBlink: true,
      cursorStyle: 'bar',
      allowProposedApi: true,
      scrollback: 5000,
      macOptionIsMeta: true,
      rightClickSelectsWord: true,
      overviewRulerWidth: 0,
    });

    const fitAddon = new FitAddon.FitAddon();
    const linksAddon = new WebLinksAddon.WebLinksAddon();
    xterm.loadAddon(fitAddon);
    xterm.loadAddon(linksAddon);

    const mountEl = document.createElement('div');
    mountEl.className = 'term-instance';
    mountEl.style.cssText = 'width:100%;height:100%;display:none;';
    container().appendChild(mountEl);
    xterm.open(mountEl);

    const refit = () => { try { fitAddon.fit(); } catch (_) {} };
    if (document.fonts && document.fonts.load) {
      document.fonts.load('13px "JetBrainsMono Nerd Font Mono"').then(refit).catch(() => {});
      document.fonts.ready.then(refit).catch(() => {});
    }

    const tabEl = document.createElement('div');
    tabEl.className = 'term-tab';
    tabEl.dataset.id = id;
    tabEl.innerHTML = `<span class="term-tab-label">bash …</span><button class="term-tab-close" title="Kill session">✕</button>`;
    tabEl.querySelector('.term-tab-close').onclick = (e) => {
      e.stopPropagation();
      closeTab(id);
    };
    tabEl.onclick = () => switchTab(id);
    tabsEl().appendChild(tabEl);

    const tab = {
      id,
      sessionId: sessionId || null,
      xterm,
      fitAddon,
      ws: null,
      mountEl,
      tabEl,
      dead: false,
    };
    if (sessionId) setTabLabel(tab, sessionId);
    tabs.push(tab);

    // Input / resize once per tab — always use current tab.ws
    xterm.onData((data) => {
      if (tab.dead) return;
      if (!tab.ws || tab.ws.readyState !== WebSocket.OPEN) {
        reconnectTab(id);
        return;
      }
      tab.ws.send(JSON.stringify({ type: 'input', data }));
    });
    xterm.onResize(({ cols, rows }) => {
      if (tab.ws && tab.ws.readyState === WebSocket.OPEN) {
        tab.ws.send(JSON.stringify({ type: 'resize', cols, rows }));
      }
    });

    bindSocket(tab, sessionId);
    if (!restoring) switchTab(id);
    return tab;
  }

  function reconnectTab(id) {
    const tab = tabs.find(t => t.id === id);
    if (!tab || tab.dead) return;
    if (tab.ws && (tab.ws.readyState === WebSocket.CONNECTING || tab.ws.readyState === WebSocket.OPEN)) {
      return;
    }
    tab.xterm.write('\r\n\x1b[36m[Reconnecting…]\x1b[0m\r\n');
    bindSocket(tab, tab.sessionId);
  }

  async function restoreSessions() {
    restoring = true;
    let remoteIds = new Set();
    try {
      const res = await fetch('/api/terminal/sessions');
      const j = await res.json();
      if (j.ok && Array.isArray(j.data)) {
        j.data.forEach(s => remoteIds.add(s.id));
      }
    } catch (_) {}

    let ids = loadStoredIds().filter(id => remoteIds.has(id));
    saveStoredIds(ids);

    if (ids.length === 0) {
      restoring = false;
      newTab();
      return;
    }
    ids.forEach(sid => newTab(sid));
    restoring = false;
    switchTab(tabs[0].id);
  }

  /* ── Switch active tab ────────────────────────────── */
  function switchTab(id) {
    tabs.forEach(t => {
      const isActive = t.id === id;
      t.mountEl.style.display = isActive ? 'block' : 'none';
      t.tabEl.classList.toggle('active', isActive);
    });
    activeId = id;
    const tab = tabs.find(t => t.id === id);
    if (tab) {
      requestAnimationFrame(() => {
        tab.fitAddon.fit();
        tab.xterm.focus();
      });
    }
  }

  /* ── Close a tab (kills persistent PTY) ───────────── */
  function closeTab(id) {
    const idx = tabs.findIndex(t => t.id === id);
    if (idx === -1) return;
    const tab = tabs[idx];
    tab.dead = true;
    forgetSession(tab.sessionId);
    try {
      if (tab.ws && tab.ws.readyState === WebSocket.OPEN) {
        tab.ws.send(JSON.stringify({ type: 'kill' }));
      }
    } catch (_) {}
    if (tab.sessionId) {
      fetch(`/api/terminal/sessions?id=${encodeURIComponent(tab.sessionId)}`, { method: 'DELETE' }).catch(() => {});
    }
    try { tab.ws && tab.ws.close(); } catch (_) {}
    tab.xterm.dispose();
    tab.mountEl.remove();
    tab.tabEl.remove();
    tabs.splice(idx, 1);

    if (tabs.length === 0) {
      closePanel();
    } else {
      const next = tabs[Math.min(idx, tabs.length - 1)];
      switchTab(next.id);
    }
  }

  /* ── Send resize message ──────────────────────────── */
  function sendResize(ws, xterm) {
    if (ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({
      type: 'resize',
      cols: xterm.cols,
      rows: xterm.rows,
    }));
  }

  /* ── Fit all tabs to panel size ───────────────────── */
  function fitAll() {
    tabs.forEach(t => {
      try { t.fitAddon.fit(); } catch {}
    });
  }

  /* ── Open / close panel ───────────────────────────── */
  function openPanel() {
    if (panelOpen) { switchToPanel(); return; }
    panelOpen = true;
    panel().classList.remove('hidden');
    if (!standalone()) {
      const h = handle();
      if (h) h.classList.remove('hidden');
      app().classList.add('term-open');
      const btn = document.getElementById('btn-terminal');
      if (btn) btn.classList.add('active');
    } else {
      expanded = true;
      panel().classList.add('expanded', 'standalone');
      app().classList.add('term-open', 'term-expanded');
    }
    if (tabs.length === 0) {
      restoreSessions().then(() => {
        requestAnimationFrame(() => {
          adaptToVisualViewport();
          fitAll();
        });
      });
    } else {
      switchTab(activeId || tabs[0].id);
      requestAnimationFrame(() => {
        adaptToVisualViewport();
        fitAll();
      });
    }
  }

  /* Hide panel — PTY sessions keep running (persist) */
  function closePanel() {
    if (standalone()) {
      location.href = '/';
      return;
    }
    panelOpen = false;
    expanded = false;
    panel().classList.add('hidden');
    panel().classList.remove('expanded');
    const h = handle();
    if (h) h.classList.add('hidden');
    app().classList.remove('term-open', 'term-expanded');
    const btn = document.getElementById('btn-terminal');
    if (btn) btn.classList.remove('active');
    resetViewportStyles();
  }

  /* Header ✕ — kill every tab's PTY, then hide */
  function killAllAndClose() {
    const ids = tabs.map(t => t.id);
    if (ids.length === 0) {
      closePanel();
      return;
    }
    ids.forEach(id => closeTab(id));
    // closeTab already hides panel when last tab dies; ensure hide on standalone
    if (standalone() && tabs.length === 0) {
      location.href = '/';
    }
  }

  function switchToPanel() {
    if (tabs.length > 0) {
      switchTab(activeId || tabs[0].id);
    }
  }

  function toggleExpand() {
    if (standalone()) return;
    expanded = !expanded;
    panel().classList.toggle('expanded', expanded);
    app().classList.toggle('term-expanded', expanded);
    const h = handle();
    if (h) h.style.bottom = expanded ? '90vh' : '45vh';
    requestAnimationFrame(fitAll);
  }

  function openPopout() {
    window.open('/term.html', '_blank', 'noopener');
  }

  /* ── Drag-resize handle ───────────────────────────── */
  function initResizeHandle() {
    const h = handle();
    if (!h || standalone()) return;
    let dragging = false;
    let startY, startH;

    h.addEventListener('pointerdown', (e) => {
      dragging = true;
      startY = e.clientY;
      startH = panel().offsetHeight;
      h.setPointerCapture(e.pointerId);
      document.body.style.userSelect = 'none';
    });

    h.addEventListener('pointermove', (e) => {
      if (!dragging) return;
      const dy = startY - e.clientY;
      const newH = Math.max(150, Math.min(window.innerHeight * 0.92, startH + dy));
      panel().style.height = newH + 'px';
      h.style.bottom = newH + 'px';
      app().style.setProperty('--term-h', newH + 'px');
      fitAll();
    });

    h.addEventListener('pointerup', () => {
      dragging = false;
      document.body.style.userSelect = '';
    });
  }

  /* ── Clipboard helpers (mobile-friendly) ──────────── */
  function activeTab() {
    return tabs.find(t => t.id === activeId);
  }

  function notify(msg, type) {
    if (typeof toast === 'function') toast(msg, type || '');
  }

  function bufferLines(xterm, from, to) {
    const buf = xterm.buffer.active;
    const start = Math.max(0, from);
    const end = Math.min(buf.length, to);
    const lines = [];
    for (let i = start; i < end; i++) {
      const line = buf.getLine(i);
      if (line) lines.push(line.translateToString(true));
    }
    return lines.join('\n').replace(/\s+$/g, '');
  }

  function getViewportText(xterm) {
    const buf = xterm.buffer.active;
    const top = buf.viewportY;
    return bufferLines(xterm, top, top + xterm.rows);
  }

  function getScrollbackText(xterm, maxLines) {
    const buf = xterm.buffer.active;
    const start = Math.max(0, buf.length - (maxLines || 300));
    return bufferLines(xterm, start, buf.length);
  }

  async function writeClipboard(text) {
    if (!text) {
      notify('Nothing to copy', 'error');
      return false;
    }
    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(text);
        notify('Copied', 'success');
        return true;
      }
    } catch (_) { /* fall through */ }

    const ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.cssText = 'position:fixed;left:-9999px;top:0;opacity:0;';
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    ta.setSelectionRange(0, ta.value.length);
    let ok = false;
    try { ok = document.execCommand('copy'); } catch (_) {}
    ta.remove();
    notify(ok ? 'Copied' : 'Copy failed', ok ? 'success' : 'error');
    return ok;
  }

  async function readClipboard() {
    try {
      if (navigator.clipboard && navigator.clipboard.readText) {
        return await navigator.clipboard.readText();
      }
    } catch (_) { /* fall through */ }
    if (typeof promptModal === 'function') {
      return await promptModal('Paste into terminal', '');
    }
    return window.prompt('Paste into terminal', '') || '';
  }

  async function copyFromTerminal(fullHistory) {
    const tab = activeTab();
    if (!tab) return;
    let text = '';
    if (!fullHistory && tab.xterm.hasSelection()) {
      text = tab.xterm.getSelection();
    } else if (fullHistory) {
      text = getScrollbackText(tab.xterm, 500);
    } else {
      text = getViewportText(tab.xterm);
    }
    await writeClipboard(text);
  }

  async function pasteToTerminal() {
    const tab = activeTab();
    if (!tab || tab.ws.readyState !== WebSocket.OPEN) return;
    const text = await readClipboard();
    if (!text) return;
    tab.ws.send(JSON.stringify({ type: 'input', data: text }));
    tab.xterm.focus();
    notify('Pasted', 'success');
  }

  /* ── Mobile key toolbar ───────────────────────────── */
  function initMobileToolbar() {
    document.querySelectorAll('.mob-key[data-key]').forEach(btn => {
      btn.addEventListener('click', () => {
        const tab = activeTab();
        if (!tab) return;
        const key = btn.dataset.key;
        if (tab.ws.readyState === WebSocket.OPEN) {
          tab.ws.send(JSON.stringify({ type: 'input', data: key }));
        }
        tab.xterm.focus();
      });
    });

    const copyBtn = document.getElementById('btn-term-copy');
    const pasteBtn = document.getElementById('btn-term-paste');
    if (copyBtn) {
      // Long-press / right-click → copy more history; tap → screen/selection
      let holdTimer = null;
      let holdFired = false;
      copyBtn.addEventListener('pointerdown', () => {
        holdFired = false;
        holdTimer = setTimeout(() => {
          holdTimer = null;
          holdFired = true;
          copyFromTerminal(true);
        }, 450);
      });
      const clearHold = () => {
        if (holdTimer) { clearTimeout(holdTimer); holdTimer = null; }
      };
      copyBtn.addEventListener('pointerup', clearHold);
      copyBtn.addEventListener('pointerleave', clearHold);
      copyBtn.addEventListener('pointercancel', clearHold);
      copyBtn.addEventListener('click', (e) => {
        if (holdFired) {
          e.preventDefault();
          holdFired = false;
          return;
        }
        copyFromTerminal(false);
      });
      copyBtn.addEventListener('contextmenu', (e) => {
        e.preventDefault();
        holdFired = true;
        copyFromTerminal(true);
      });
    }
    if (pasteBtn) {
      pasteBtn.addEventListener('click', () => pasteToTerminal());
    }
  }

  /* ── Keep panel/toolbar above soft keyboard ───────── */
  function adaptToVisualViewport() {
    const p = panel();
    if (!p || (!panelOpen && !standalone())) return;

    const vv = window.visualViewport;
    if (!vv) {
      fitAll();
      return;
    }

    if (standalone()) {
      // Shrink fullscreen page to the visible viewport (above keyboard)
      p.style.top = vv.offsetTop + 'px';
      p.style.left = '0px';
      p.style.right = '0px';
      p.style.bottom = 'auto';
      p.style.height = vv.height + 'px';
      p.style.maxHeight = vv.height + 'px';
    } else if (panelOpen) {
      // Dock bottom sheet above the occluded keyboard region
      const occluded = Math.max(0, window.innerHeight - vv.height - vv.offsetTop);
      p.style.bottom = occluded + 'px';
      const h = handle();
      if (h && !h.classList.contains('hidden')) {
        const panelH = p.offsetHeight || 0;
        h.style.bottom = (occluded + panelH) + 'px';
      }
    }

    fitAll();
  }

  function resetViewportStyles() {
    const p = panel();
    if (!p) return;
    if (standalone()) return;
    p.style.bottom = '';
    const h = handle();
    if (h) h.style.bottom = expanded ? '90vh' : '45vh';
  }

  /* ── Window / visual viewport resize ──────────────── */
  let resizeTimer;
  function scheduleViewportAdapt() {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(adaptToVisualViewport, 40);
  }
  window.addEventListener('resize', scheduleViewportAdapt);

  /* ── Public init ──────────────────────────────────── */
  function init() {
    // Prefetch nerd font so first prompt icons aren't □
    if (document.fonts && document.fonts.load) {
      document.fonts.load('13px "JetBrainsMono Nerd Font Mono"').catch(() => {});
      document.fonts.load('700 13px "JetBrainsMono Nerd Font Mono"').catch(() => {});
    }

    const btnTerm = document.getElementById('btn-terminal');
    if (btnTerm) {
      btnTerm.onclick = () => {
        panelOpen ? closePanel() : openPanel();
      };
    }

    const btnNew = document.getElementById('btn-term-new');
    if (btnNew) btnNew.onclick = () => newTab();

    const btnClose = document.getElementById('btn-term-close');
    if (btnClose) btnClose.onclick = () => killAllAndClose();

    const btnResize = document.getElementById('btn-term-resize');
    if (btnResize) {
      if (standalone()) btnResize.classList.add('hidden');
      else btnResize.onclick = () => toggleExpand();
    }

    ['btn-term-popout', 'btn-term-popout-panel'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.onclick = (e) => {
        e.preventDefault();
        openPopout();
      };
    });

    initResizeHandle();
    initMobileToolbar();

    if (window.visualViewport) {
      visualViewport.addEventListener('resize', scheduleViewportAdapt);
      visualViewport.addEventListener('scroll', scheduleViewportAdapt);
    }

    // Fullscreen terminal page: start immediately
    if (standalone()) {
      openPanel();
      setTimeout(adaptToVisualViewport, 50);
      setTimeout(adaptToVisualViewport, 300);
    }
  }

  return { init, open: openPanel, close: closePanel, newTab, popout: openPopout };
})();

document.addEventListener('DOMContentLoaded', () => Term.init());
