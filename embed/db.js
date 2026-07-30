/* ── DB browser panel ─────────────────────────────────── */
const dbState = {
  sessionId: '',
  driver: 'mysql',
  database: '',
  databases: [],
  tables: [],
  table: '',
  columns: [],
  rows: [],
  rowColumns: [],
  limit: 100,
  offset: 0,
  hasMore: false,
  hasPk: false,
  view: 'rows', // rows | sql
};

const dbApi = {
  async call(method, path, body) {
    const opts = { method, headers: {} };
    if (dbState.sessionId) opts.headers['X-DB-Session'] = dbState.sessionId;
    if (body !== undefined) {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(body);
    }
    const res = await fetch(path, opts);
    return res.json();
  },
  connect: (body) => dbApi.call('POST', '/api/db/connect', body),
  disconnect: () => dbApi.call('POST', '/api/db/disconnect'),
  databases: () => dbApi.call('GET', '/api/db/databases'),
  use: (database) => dbApi.call('POST', '/api/db/use', { database }),
  tables: () => dbApi.call('GET', '/api/db/tables'),
  columns: (table) => dbApi.call('GET', `/api/db/columns?table=${encodeURIComponent(table)}`),
  rows: (table, limit, offset) =>
    dbApi.call('GET', `/api/db/rows?table=${encodeURIComponent(table)}&limit=${limit}&offset=${offset}`),
  insert: (table, values) => dbApi.call('POST', '/api/db/row', { table, values }),
  update: (table, pk, values) => dbApi.call('PUT', '/api/db/row', { table, pk, values }),
  remove: (table, pk) => dbApi.call('DELETE', '/api/db/row', { table, pk }),
  query: (sql) => dbApi.call('POST', '/api/db/query', { sql }),
};

function isDBFile(file) {
  if (!file || file.is_dir) return false;
  const ext = (file.ext || '').toLowerCase();
  return ext === 'db' || ext === 'sqlite' || ext === 'sqlite3';
}

function isDBOpen() {
  return !$('#db-overlay').classList.contains('hidden');
}

async function openDBPanel(prefill) {
  $('#db-overlay').classList.remove('hidden');
  if (prefill && prefill.autoConnect) {
    if (dbState.sessionId) {
      await dbApi.disconnect().catch(() => {});
      dbState.sessionId = '';
    }
    showDBConnect(prefill);
    return;
  }
  if (dbState.sessionId) {
    showDBBrowser();
    return;
  }
  showDBConnect(prefill);
}

function closeDBPanel() {
  $('#db-overlay').classList.add('hidden');
}

function showDBConnect(prefill) {
  $('#db-connect').classList.remove('hidden');
  $('#db-browser').classList.add('hidden');
  $('#db-header-actions').classList.add('hidden');
  $('#db-title').textContent = 'Database';
  const driver = (prefill && prefill.driver) || 'mysql';
  setDBDriver(driver);
  if (prefill && prefill.path) {
    $('#db-path').value = prefill.path;
  }
  if (prefill && prefill.autoConnect && prefill.path) {
    doDBConnect();
  }
}

function showDBBrowser() {
  $('#db-connect').classList.add('hidden');
  $('#db-browser').classList.remove('hidden');
  $('#db-header-actions').classList.remove('hidden');
  updateDBHeader();
}

function setDBDriver(driver) {
  dbState.driver = driver;
  document.querySelectorAll('.db-driver-btn').forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.driver === driver);
  });
  const mysql = driver === 'mysql';
  $('#db-mysql-fields').classList.toggle('hidden', !mysql);
  $('#db-sqlite-fields').classList.toggle('hidden', mysql);
}

function updateDBHeader() {
  const label = dbState.driver === 'sqlite'
    ? (dbState.database || 'SQLite')
    : `${dbState.database || '(no db)'} · MySQL`;
  $('#db-title').textContent = label;
}

async function doDBConnect() {
  const driver = dbState.driver;
  let body;
  if (driver === 'sqlite') {
    const path = $('#db-path').value.trim();
    if (!path) {
      toast('SQLite path required', 'error');
      return;
    }
    body = { driver: 'sqlite', path };
  } else {
    body = {
      driver: 'mysql',
      host: $('#db-host').value.trim() || '127.0.0.1',
      port: parseInt($('#db-port').value, 10) || 3306,
      user: $('#db-user').value.trim() || 'root',
      password: $('#db-password').value,
      database: $('#db-database').value.trim(),
    };
  }
  $('#db-connect-btn').disabled = true;
  try {
    const res = await dbApi.connect(body);
    if (!res.ok) {
      toast(res.error || 'Connect failed', 'error');
      return;
    }
    dbState.sessionId = res.data.session_id;
    dbState.driver = res.data.driver;
    dbState.database = res.data.database || '';
    dbState.databases = res.data.databases || [];
    dbState.table = '';
    dbState.rows = [];
    showDBBrowser();
    renderDBDatabaseSelect();
    await loadDBTables();
    toast('Connected', 'success');
  } finally {
    $('#db-connect-btn').disabled = false;
  }
}

async function doDBDisconnect() {
  if (dbState.sessionId) {
    await dbApi.disconnect().catch(() => {});
  }
  dbState.sessionId = '';
  dbState.database = '';
  dbState.databases = [];
  dbState.tables = [];
  dbState.table = '';
  dbState.rows = [];
  showDBConnect();
  toast('Disconnected', 'success');
}

function renderDBDatabaseSelect() {
  const wrap = $('#db-db-select-wrap');
  const sel = $('#db-db-select');
  if (dbState.driver !== 'mysql') {
    wrap.classList.add('hidden');
    return;
  }
  wrap.classList.remove('hidden');
  sel.innerHTML = '';
  const empty = document.createElement('option');
  empty.value = '';
  empty.textContent = '— select database —';
  sel.appendChild(empty);
  (dbState.databases || []).forEach((name) => {
    const opt = document.createElement('option');
    opt.value = name;
    opt.textContent = name;
    if (name === dbState.database) opt.selected = true;
    sel.appendChild(opt);
  });
}

async function loadDBTables() {
  const res = await dbApi.tables();
  if (!res.ok) {
    toast(res.error || 'List tables failed', 'error');
    return;
  }
  dbState.tables = res.data || [];
  renderDBTableList();
}

function renderDBTableList() {
  const list = $('#db-table-list');
  list.innerHTML = '';
  if (!dbState.tables.length) {
    list.innerHTML = '<div class="db-empty">No tables</div>';
    return;
  }
  dbState.tables.forEach((t) => {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'db-table-item' + (t.name === dbState.table ? ' active' : '');
    btn.textContent = t.name;
    if (t.type === 'view') {
      const tag = document.createElement('span');
      tag.className = 'db-view-tag';
      tag.textContent = 'view';
      btn.appendChild(tag);
    }
    btn.onclick = () => selectDBTable(t.name);
    list.appendChild(btn);
  });
}

async function selectDBTable(name) {
  dbState.table = name;
  dbState.offset = 0;
  dbState.view = 'rows';
  renderDBTableList();
  setDBView('rows');
  await Promise.all([loadDBColumns(), loadDBRows()]);
}

async function loadDBColumns() {
  if (!dbState.table) return;
  const res = await dbApi.columns(dbState.table);
  if (!res.ok) {
    toast(res.error || 'Columns failed', 'error');
    return;
  }
  dbState.columns = res.data || [];
  dbState.hasPk = dbState.columns.some((c) => c.primary);
  $('#db-insert-btn').disabled = !dbState.table;
  $('#db-no-pk-hint').classList.toggle('hidden', dbState.hasPk || !dbState.table);
}

async function loadDBRows() {
  if (!dbState.table) {
    $('#db-grid').innerHTML = '<div class="db-empty">Select a table</div>';
    return;
  }
  const res = await dbApi.rows(dbState.table, dbState.limit, dbState.offset);
  if (!res.ok) {
    toast(res.error || 'Load rows failed', 'error');
    return;
  }
  dbState.rowColumns = res.data.columns || [];
  dbState.rows = res.data.rows || [];
  dbState.hasMore = !!res.data.has_more;
  renderDBGrid(dbState.rowColumns, dbState.rows, true);
  $('#db-page-info').textContent = `${dbState.offset + 1}–${dbState.offset + dbState.rows.length}`;
  $('#db-prev').disabled = dbState.offset <= 0;
  $('#db-next').disabled = !dbState.hasMore;
}

function renderDBGrid(columns, rows, editable) {
  const grid = $('#db-grid');
  if (!columns.length) {
    grid.innerHTML = '<div class="db-empty">No columns</div>';
    return;
  }
  const table = document.createElement('table');
  table.className = 'db-table';
  const thead = document.createElement('thead');
  const hr = document.createElement('tr');
  if (editable && dbState.hasPk) {
    const th = document.createElement('th');
    th.className = 'db-actions-col';
    th.textContent = '';
    hr.appendChild(th);
  }
  columns.forEach((c) => {
    const th = document.createElement('th');
    const colMeta = dbState.columns.find((x) => x.name === c);
    th.textContent = c;
    if (colMeta && colMeta.primary) th.classList.add('pk');
    hr.appendChild(th);
  });
  thead.appendChild(hr);
  table.appendChild(thead);
  const tbody = document.createElement('tbody');
  if (!rows.length) {
    const tr = document.createElement('tr');
    const td = document.createElement('td');
    td.colSpan = columns.length + (editable && dbState.hasPk ? 1 : 0);
    td.className = 'db-empty';
    td.textContent = 'No rows';
    tr.appendChild(td);
    tbody.appendChild(tr);
  } else {
    rows.forEach((row, rowIdx) => {
      const tr = document.createElement('tr');
      if (editable && dbState.hasPk) {
        const tdAct = document.createElement('td');
        tdAct.className = 'db-actions-col';
        const del = document.createElement('button');
        del.type = 'button';
        del.className = 'db-row-del';
        del.title = 'Delete row';
        del.textContent = '✕';
        del.onclick = (e) => {
          e.stopPropagation();
          deleteDBRow(row);
        };
        tdAct.appendChild(del);
        tr.appendChild(tdAct);
      }
      columns.forEach((c) => {
        const td = document.createElement('td');
        const val = row[c];
        td.textContent = val === null || val === undefined ? 'NULL' : String(val);
        if (val === null || val === undefined) td.classList.add('null');
        if (editable && dbState.hasPk) {
          td.classList.add('editable');
          td.onclick = () => editDBCell(row, c, val);
        }
        tr.appendChild(td);
      });
      tbody.appendChild(tr);
    });
  }
  table.appendChild(tbody);
  grid.innerHTML = '';
  grid.appendChild(table);
}

function pkFromRow(row) {
  const pk = {};
  dbState.columns.filter((c) => c.primary).forEach((c) => {
    pk[c.name] = row[c.name];
  });
  return pk;
}

async function editDBCell(row, col, current) {
  if (!dbState.hasPk) return;
  const input = document.createElement('input');
  input.type = 'text';
  input.value = current === null || current === undefined ? '' : String(current);
  const nullLabel = document.createElement('label');
  nullLabel.className = 'db-null-check';
  const nullCb = document.createElement('input');
  nullCb.type = 'checkbox';
  nullCb.checked = current === null || current === undefined;
  nullLabel.appendChild(nullCb);
  nullLabel.appendChild(document.createTextNode(' NULL'));
  const wrap = document.createElement('div');
  wrap.appendChild(input);
  wrap.appendChild(nullLabel);
  showModal({
    title: `Edit ${col}`,
    body: wrap,
    confirmText: 'Save',
    onConfirm: async () => {
      const values = {};
      values[col] = nullCb.checked ? null : input.value;
      const res = await dbApi.update(dbState.table, pkFromRow(row), values);
      if (!res.ok) {
        toast(res.error || 'Update failed', 'error');
        return;
      }
      toast('Updated', 'success');
      loadDBRows();
    },
  });
}

async function deleteDBRow(row) {
  const ok = await confirmModal('Delete row', 'Delete this row?', true);
  if (!ok) return;
  const res = await dbApi.remove(dbState.table, pkFromRow(row));
  if (!res.ok) {
    toast(res.error || 'Delete failed', 'error');
    return;
  }
  toast('Deleted', 'success');
  loadDBRows();
}

async function insertDBRow() {
  if (!dbState.table || !dbState.columns.length) return;
  const form = document.createElement('div');
  form.className = 'db-insert-form';
  const inputs = {};
  dbState.columns.forEach((c) => {
    const row = document.createElement('div');
    row.className = 'db-insert-row';
    const lab = document.createElement('label');
    lab.textContent = c.name + (c.primary ? ' (PK)' : '');
    const input = document.createElement('input');
    input.type = 'text';
    input.placeholder = c.type || '';
    inputs[c.name] = input;
    row.appendChild(lab);
    row.appendChild(input);
    form.appendChild(row);
  });
  showModal({
    title: `Insert into ${dbState.table}`,
    body: form,
    confirmText: 'Insert',
    onConfirm: async () => {
      const values = {};
      Object.keys(inputs).forEach((k) => {
        const v = inputs[k].value;
        if (v !== '') values[k] = v;
      });
      const res = await dbApi.insert(dbState.table, values);
      if (!res.ok) {
        toast(res.error || 'Insert failed', 'error');
        return;
      }
      toast('Inserted', 'success');
      loadDBRows();
    },
  });
}

function setDBView(view) {
  dbState.view = view;
  $('#db-rows-view').classList.toggle('hidden', view !== 'rows');
  $('#db-sql-view').classList.toggle('hidden', view !== 'sql');
  $('#db-view-rows').classList.toggle('active', view === 'rows');
  $('#db-view-sql').classList.toggle('active', view === 'sql');
}

async function runDBSQL() {
  const sql = $('#db-sql-input').value.trim();
  if (!sql) return;
  const res = await dbApi.query(sql);
  if (!res.ok) {
    toast(res.error || 'Query failed', 'error');
    $('#db-sql-result').innerHTML = `<div class="db-empty error">${escHtml(res.error || 'Error')}</div>`;
    return;
  }
  if (res.data.is_select) {
    // temporarily render into sql result without edit
    const prevHasPk = dbState.hasPk;
    dbState.hasPk = false;
    const grid = $('#db-sql-result');
    const columns = res.data.columns || [];
    const rows = res.data.rows || [];
    if (!columns.length) {
      grid.innerHTML = '<div class="db-empty">Empty result</div>';
    } else {
      const table = document.createElement('table');
      table.className = 'db-table';
      const thead = document.createElement('thead');
      const hr = document.createElement('tr');
      columns.forEach((c) => {
        const th = document.createElement('th');
        th.textContent = c;
        hr.appendChild(th);
      });
      thead.appendChild(hr);
      table.appendChild(thead);
      const tbody = document.createElement('tbody');
      rows.forEach((row) => {
        const tr = document.createElement('tr');
        columns.forEach((c) => {
          const td = document.createElement('td');
          const val = row[c];
          td.textContent = val === null || val === undefined ? 'NULL' : String(val);
          if (val === null || val === undefined) td.classList.add('null');
          tr.appendChild(td);
        });
        tbody.appendChild(tr);
      });
      table.appendChild(tbody);
      grid.innerHTML = '';
      grid.appendChild(table);
    }
    dbState.hasPk = prevHasPk;
    toast(`${rows.length} row(s)`, 'success');
  } else {
    $('#db-sql-result').innerHTML =
      `<div class="db-empty">Rows affected: ${res.data.rows_affected ?? 0}</div>`;
    toast('OK', 'success');
    if (dbState.table) loadDBRows();
    loadDBTables();
  }
}

function escHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function openSQLiteFile(file) {
  openDBPanel({ driver: 'sqlite', path: file.path, autoConnect: true });
}

function initDBPanel() {
  $('#btn-db').onclick = () => openDBPanel();
  $('#db-back').onclick = () => closeDBPanel();
  $('#db-disconnect').onclick = () => doDBDisconnect();

  document.querySelectorAll('.db-driver-btn').forEach((btn) => {
    btn.onclick = () => setDBDriver(btn.dataset.driver);
  });
  $('#db-connect-btn').onclick = () => doDBConnect();
  ['#db-host', '#db-port', '#db-user', '#db-password', '#db-database', '#db-path'].forEach((sel) => {
    const el = $(sel);
    if (el) {
      el.onkeydown = (e) => {
        if (e.key === 'Enter') doDBConnect();
      };
    }
  });

  $('#db-db-select').onchange = async () => {
    const name = $('#db-db-select').value;
    if (!name) return;
    const res = await dbApi.use(name);
    if (!res.ok) {
      toast(res.error || 'USE failed', 'error');
      return;
    }
    dbState.database = name;
    dbState.table = '';
    updateDBHeader();
    await loadDBTables();
    $('#db-grid').innerHTML = '<div class="db-empty">Select a table</div>';
  };

  $('#db-refresh').onclick = async () => {
    if (dbState.driver === 'mysql') {
      const res = await dbApi.databases();
      if (res.ok) {
        dbState.databases = res.data || [];
        renderDBDatabaseSelect();
      }
    }
    await loadDBTables();
    if (dbState.table) await loadDBRows();
  };
  $('#db-insert-btn').onclick = () => insertDBRow();
  $('#db-view-rows').onclick = () => {
    setDBView('rows');
    if (dbState.table) loadDBRows();
  };
  $('#db-view-sql').onclick = () => setDBView('sql');
  $('#db-sql-run').onclick = () => runDBSQL();
  $('#db-prev').onclick = async () => {
    dbState.offset = Math.max(0, dbState.offset - dbState.limit);
    await loadDBRows();
  };
  $('#db-next').onclick = async () => {
    if (!dbState.hasMore) return;
    dbState.offset += dbState.limit;
    await loadDBRows();
  };

  setDBDriver('mysql');
}
