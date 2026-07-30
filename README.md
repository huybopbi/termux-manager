# termux-manager

File manager + web terminal for [Termux](https://termux.dev) — one Go binary, open in the phone browser.

No Node/Python runtime. Frontend is embedded with `go:embed`.

Repo: [github.com/huybopbi/termux-manager](https://github.com/huybopbi/termux-manager)

---

## Install (Termux)

```bash
curl -fsSL https://raw.githubusercontent.com/huybopbi/termux-manager/main/install.sh | bash
```

Detects `arm64` / `arm` / `x86_64`, downloads the release binary into `$PREFIX/bin/manager`.

**Manual (arm64 example):**

```bash
wget https://github.com/huybopbi/termux-manager/releases/latest/download/manager-android-arm64 \
  -O $PREFIX/bin/manager
chmod +x $PREFIX/bin/manager
```

> If `$PREFIX/bin/manager` already exists as something else, install under another name (e.g. `termux-manager`).

---

## Usage

```bash
manager                    # :9876, auto-open browser
manager -port 8080
manager -listen 0.0.0.0    # LAN / ngrok (or use Settings in the UI)
manager -root $HOME
manager -root /sdcard      # needs storage access
manager -hidden            # show dotfiles
manager -no-open
manager -version
manager update             # download latest binary (uses curl/wget on Termux)
```

Stop with **Ctrl+C**. After `manager update`, restart the process to apply the new binary.

Default bind is **127.0.0.1**. Open **Settings** in the UI (or pass `-listen 0.0.0.0`) to allow LAN / tunnel access. Preferences are saved to `~/.config/termux-manager/config.json`.

**No auth** — if you expose the port on LAN or via ngrok, treat it as full access to the chosen root.

---

## Features

### File manager
- **Termux Midnight UI**: emerald dark surfaces, consistent SVG icons, compact mobile header
- Browse with breadcrumb, parent `..`, Esc → up
- Create folder/file (FAB), rename, delete, copy/cut/paste selection
- Upload files or folders via FAB **+** (or drag-and-drop) with progress bar, download
  (server streams to disk — low RAM use for multi‑GB files)
- Search under current directory
- Zip / unzip
- **Quick paths**: Home, Storage (`/sdcard`), Download, DCIM, Shared, Prefix
- **Image preview** (png/jpg/webp/…)
- **PDF preview** (PDF.js CDN canvas render — works on Android browsers)
- **Markdown preview** (`.md` / `.markdown`; Edit opens Ace)
- **Video play** via Plyr (CDN, lazy-loaded): mp4/webm/mov… ; mkv/avi may need download
- Hidden files toggle in **Settings** (also `-hidden` / config)
- **Settings**: listen address (localhost / all interfaces) and port; saved to `~/.config/termux-manager/config.json`

### Editor ([Ace](https://ace.c9.io/))
- Virtualized editor (better with multi‑MB text than a plain textarea)
- Syntax modes: shell, JS/TS, Python, Go, PHP, C/C++, Rust, JSON, YAML, Markdown, …
- Also editable as text: `.pem` / `.key` / `.crt` / `.cer`, config backups, …
- Save (Ctrl+S), find/replace, go to line, wrap, font size, undo/redo, dirty indicator
- Files without extension open as text

### Terminal
- Real PTY over WebSocket (`xterm.js`)
- **Persistent sessions** — refresh / hide panel keeps the shell; ✕ kills
- Multi-tab, resize, fullscreen page (`/term.html`)
- Mobile toolbar (Esc, Tab, Ctrl keys, arrows, Copy/Paste)
- Nerd Font for Powerlevel10k icons; strips inherited `SSH_*` env

### Database
- Toolbar **DB**: connect to **MySQL/MariaDB** or **SQLite** (pure Go, `CGO_ENABLED=0`)
- Browse tables, paginated rows, edit/insert/delete when a primary key exists
- SQL console; context menu **Open as DB** for `.db` / `.sqlite` / `.sqlite3`
- In-memory sessions only (idle TTL ~30m); no saved passwords

### Termux hooks
- Share file (`termux-share`)
- Clipboard path/text
- Auto-open URL (`termux-open-url`)

---

## Build from source

**Go 1.21+**

```bash
git clone https://github.com/huybopbi/termux-manager.git
cd termux-manager
go mod tidy

make android-arm64   # → dist/manager-android-arm64
make build           # native binary → ./manager
make all             # arm64 + arm + amd64
```

Windows (PowerShell) quick cross-build:

```powershell
$env:GOOS='android'; $env:GOARCH='arm64'; $env:CGO_ENABLED='0'
go build -o dist/manager-android-arm64 .
```

---

## Architecture

```
termux-manager/
├── main.go
├── server/          # HTTP API + persistent PTY WebSocket + DB handlers
├── fs/              # list/read/write/copy/zip/…
├── db/              # MySQL + SQLite session store / query helpers
├── termux/          # Termux bridge + quick paths
├── embed/           # UI (go:embed), Ace under embed/vendor/ace/
├── Makefile
├── install.sh
└── .github/workflows/release.yml
```

1. Binary serves embedded static files  
2. REST under `/api/*` for files  
3. `/ws/terminal?id=` attaches to a durable PTY session  
4. `/api/db/*` opens an in-memory DB session (`X-DB-Session` header)

---

## REST / WS (summary)

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/list?path=` | Directory listing |
| GET/POST | `/api/read`, `/api/write` | Text edit |
| GET | `/api/download?path=&inline=1` | Download / inline preview |
| GET | `/api/info` | Root, Termux flags, `quick_paths` |
| GET/PUT | `/api/settings` | Listen address + port (persisted) |
| POST | `/api/root` | Switch browse root (quick path) |
| POST | `/api/untar` | Extract `.tar` / `.tar.gz` / `.tgz` |
| POST | `/api/unzip` | Extract `.zip` |
| GET/DELETE | `/api/terminal/sessions` | List / kill PTY sessions |
| WS | `/ws/terminal?id=` | Persistent terminal |
| POST | `/api/db/connect` | MySQL or SQLite → `session_id` |
| POST | `/api/db/disconnect` | Close session (`X-DB-Session`) |
| GET | `/api/db/databases` | List schemas (MySQL) |
| POST | `/api/db/use` | Switch schema (MySQL) |
| GET | `/api/db/tables` | List tables/views |
| GET | `/api/db/columns?table=` | Column meta + PK |
| GET | `/api/db/rows?table=&limit=&offset=` | Browse rows (capped) |
| POST/PUT/DELETE | `/api/db/row` | Insert / update / delete by PK |
| POST | `/api/db/query` | SQL console (15s timeout) |

JSON shape: `{"ok":true,"data":...}` or `{"ok":false,"error":"..."}`.

Terminal messages: `input` / `resize` / `kill` (client), `output` / `history` / `ready` (server).

DB panel: toolbar **DB**, or context menu **Open as DB** on `.db` / `.sqlite` / `.sqlite3`. Sessions idle out after ~30 minutes. Passwords are not persisted.

---

## Security

Default listen is **127.0.0.1**. You can switch to **0.0.0.0** in Settings (or `-listen`) for LAN/ngrok. **No auth** by default — treat any non-localhost exposure as full access to the chosen root.

Same-origin UI only: the server rejects requests whose `Origin` does not match `Host`, and when bound to loopback it also rejects non-loopback `Host` values (DNS-rebinding). Cross-origin CORS headers are not set.

---

## Roadmap

### Done
- [x] Ace editor (replace textarea + highlight.js)
- [x] Quick paths (Home / Storage / Download / DCIM / Shared / Prefix)
- [x] Image preview
- [x] Persistent terminal sessions (reconnect after refresh; ✕ kills)
- [x] Extract `.tar.gz` / `.tgz` / `.tar` and `.zip` (editable full path → open folder)
- [x] Colored file-type badges (folder keeps emoji `📁`)
- [x] Open extensionless files in editor
- [x] Editor extras: dirty ●, Ctrl+S, find/replace, goto, wrap, font ±, undo/redo
- [x] Terminal: Nerd Font, mobile Copy/Paste, popout `/term.html`, clean `SSH_*` env
- [x] DB panel: connect MySQL/MariaDB + SQLite, browse/edit rows (PK), SQL console
- [x] Termux Midnight theme: refined dark surfaces, SVG icon system, top header nav on mobile
- [x] URL hash path persistence (F5 restores current directory)
- [x] GitHub Releases with pre-built binaries (android-arm64, android-arm, linux-amd64, linux-arm64, darwin-arm64)
- [x] `manager update` — self-update from latest GitHub Release
- [x] Settings UI + `-listen` / config for LAN & tunnel bind (`0.0.0.0`)
- [x] Upload progress bar (multi-file / large files, cancel)
- [x] Upload folders (webkitdirectory + drag-drop, preserves relative paths)
- [x] Streamed upload (MultipartReader) — large files without buffering into RAM
- [x] Video playback with Plyr CDN (lazy-loaded)
- [x] Path containment hardening (`Within` / zip-slip via `filepath.Rel`) + Origin/Host guard (no `ACAO:*`)
- [x] PDF preview via PDF.js (CDN) + Markdown rendered preview (marked CDN)
- [x] Edit cert/key text files (`.pem` / `.key` / `.crt` / `.cer`)

### Planned
- [ ] Basic auth (`-auth`)
- [ ] Light theme
- [ ] Receive Android shares into current folder
- [ ] Warn / limit opening very large files in editor

---

## License

MIT
