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
manager -root $HOME
manager -root /sdcard      # needs storage access
manager -hidden            # show dotfiles
manager -no-open
manager -version
```

Stop with **Ctrl+C**.

Bind is **127.0.0.1 only**. From a PC: SSH tunnel, then open `http://127.0.0.1:<port>`.

---

## Features

### File manager
- Browse with breadcrumb, parent `..`, Esc → up
- Create folder/file (FAB), rename, delete, copy/cut/paste selection
- Upload (button or drag-and-drop), download
- Search under current directory
- Zip / unzip
- **Quick paths**: Home, Storage (`/sdcard`), Download, DCIM, Shared, Prefix
- **Image preview** (png/jpg/webp/…)
- Hidden files toggle

### Editor ([Ace](https://ace.c9.io/))
- Virtualized editor (better with multi‑MB text than a plain textarea)
- Syntax modes: shell, JS/TS, Python, Go, PHP, C/C++, Rust, JSON, YAML, Markdown, …
- Save (Ctrl+S), find/replace, go to line, wrap, font size, undo/redo, dirty indicator
- Files without extension open as text

### Terminal
- Real PTY over WebSocket (`xterm.js`)
- **Persistent sessions** — refresh / hide panel keeps the shell; ✕ kills
- Multi-tab, resize, fullscreen page (`/term.html`)
- Mobile toolbar (Esc, Tab, Ctrl keys, arrows, Copy/Paste)
- Nerd Font for Powerlevel10k icons; strips inherited `SSH_*` env

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
├── server/          # HTTP API + persistent PTY WebSocket
├── fs/              # list/read/write/copy/zip/…
├── termux/          # Termux bridge + quick paths
├── embed/           # UI (go:embed), Ace under embed/vendor/ace/
├── Makefile
├── install.sh
└── .github/workflows/release.yml
```

1. Binary serves embedded static files  
2. REST under `/api/*` for files  
3. `/ws/terminal?id=` attaches to a durable PTY session  

---

## REST / WS (summary)

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/list?path=` | Directory listing |
| GET/POST | `/api/read`, `/api/write` | Text edit |
| GET | `/api/download?path=&inline=1` | Download / inline preview |
| GET | `/api/info` | Root, Termux flags, `quick_paths` |
| POST | `/api/root` | Switch browse root (quick path) |
| GET/DELETE | `/api/terminal/sessions` | List / kill PTY sessions |
| WS | `/ws/terminal?id=` | Persistent terminal |

JSON shape: `{"ok":true,"data":...}` or `{"ok":false,"error":"..."}`.

Terminal messages: `input` / `resize` / `kill` (client), `output` / `history` / `ready` (server).

---

## Security

Listens on **localhost only**. No auth by default. If you tunnel or proxy the port, treat it as full access to the chosen root.

---

## Roadmap

- [ ] Basic auth (`-auth`)
- [ ] Light theme
- [x] `.tar.gz` / `.tgz` / `.tar` extract (and `.zip`)
- [ ] Receive Android shares into current folder

---

## License

MIT
