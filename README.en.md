# hunsu

[한국어](README.md) | English

A local, HTML-native design tool: edit pages by hand or have an LLM build and refine them, live in your browser.

![hunsu demo: select a heading, ask to make it amber and larger, check at mobile width, then undo](docs/demo.gif)

- **Build and edit by talking**: create pages and a design system from an empty folder, then refine with "make this button bigger".
- **Select elements**: click in the render view to say "this"; double-click to edit text in place.
- **Design system**: `DESIGN.md` (rules) and `design.css` (values), shared by every page and the LLM.
- **Assets**: drop in images and fonts; the LLM uses them in pages.
- **External agents (MCP)**: MCP clients such as Claude Code or Codex see your selection and edit files directly.
- **Undo and versions**: roll back a whole request in one step, or return to any thumbnailed version.
- **Export**: folder ZIP, single-file HTML per page, PNG.

The UI is currently Korean only; on-screen labels are given in parentheses.

## Requirements

- macOS
- LLM for the browser chat: [Claude Code](https://claude.com/claude-code) CLI (`claude`) or [Codex](https://github.com/openai/codex) CLI (`codex`), logged in
- Chrome (optional): lets the LLM look at the rendered result. Chromium, Edge and Brave work too; without one, only this feature is off
- Go 1.27+: build only

## Run

```bash
go build -o hunsu .
./hunsu ~/designs      # defaults to the current directory
```

Serves at `http://127.0.0.1:7777` and opens your browser. `-port 8888` changes the port, `-no-open` skips the browser. `-provider codex` picks the LLM (by default the first installed of claude, codex).

## Usage

![hunsu screen: file panel, mobile render at 75% with a heading selected, editor and chat](docs/screenshot.png)

The screen has a file panel, render view, and editor & chat panel. Drag borders to resize (double-click to reset); collapse panels from the toolbar.

**Chat**

- Ask "make a café landing page", "add a menu page with the same design", "round the buttons on every page". New pages open automatically.
- Progress shows like "writing index.html · 12.4KB · 23s". While running, Send (보내기) becomes Cancel (취소).
- The conversation continues per project. New chat (새 대화) starts over.
- Undo (되돌리기) restores the state before the last request; repeat to go further back.
- Versions (버전) lists the version saved after every request, with thumbnails. Restore any of them (Undo reverts a restore too), or save one yourself with Save current state (지금 상태 저장). The last 100 are kept.
- Models come from the provider. The claude provider defaults to opus, with sonnet and haiku. A new page takes about 1 min 40 s with opus, about 30 s with haiku.

**Select an element**

Click an element with the select tool (V); a selection chip appears above the input. Ask "make this red". The selection survives page reloads; clear it with Esc or ×.

| Tool | Behavior |
|---|---|
| Select (V) | Click to select, double-click to edit text. Links and buttons don't fire |
| Hand (H) | Drag to move inside the page. A click without dragging works normally |
| Mobile / Desktop | 390px / full render width |
| − 100% + | Zoom 25–300% |

**Edit text in place**

Double-click an element that holds only text (headings, buttons, links) to edit it. Enter or clicking elsewhere saves to the source; Esc cancels. Text the source doesn't contain, such as text written by page JavaScript, is not saved and you are told why. Like editor saves, this is not part of Undo.

**Design system**

| File | Contents |
|---|---|
| `design.css` | CSS variables and shared classes, linked by every page |
| `DESIGN.md` | Rules in prose, followed by the LLM |

Both are pinned at the top of the file panel (click to create if missing) and sent to the LLM on the first request of a conversation and whenever they change.

**Assets**

Drop images (png, jpg, gif, webp, avif, svg) or fonts (woff, woff2, ttf, otf) anywhere on the screen, or pick them with Upload (이미지·폰트 올리기), to save them in `assets/`. Up to 10 MB per file; existing names are rejected. The list is sent to the LLM with every request. Assets are not part of undo; delete them in Finder.

**Export**

Download from Export (내보내기) in the toolbar.

| Item | Contents |
|---|---|
| This page · single HTML file | HTML with `design.css` and assets inlined. Opens the same anywhere |
| This page · PNG | Full page at the width you are viewing (mobile 390px, desktop 1280px). Needs Chrome |
| Whole folder · ZIP | Design system, pages, assets (without `.hunsu/`) |

**Edit directly**: edit in the source editor and press ⌘S. Edits from other editors show up within 0.5 s and are reported to the LLM on the next request.

## External agents (MCP)

hunsu runs a standard MCP (Streamable HTTP) server. Agents read the browser state and edit files directly; hunsu redraws immediately.

1. On startup hunsu prints the MCP URL and token: `http://127.0.0.1:7777/mcp`, `Authorization: Bearer <token>`
2. Register it once in your MCP client. For Claude Code:
   ```bash
   claude mcp add --transport http -s user hunsu http://127.0.0.1:7777/mcp -H "Authorization: Bearer <token>"
   ```
3. Ask your agent from the design folder: "change the element I selected in hunsu to red, then make an about page and open it"

| Tool | Behavior |
|---|---|
| `get_context` | Folder path, pages, assets, design system, visible page, selected element |
| `checkpoint` | Save before editing; restorable with the browser's Undo |
| `undo` | Restore the last checkpoint |
| `open_page` | Open a page in the browser |
| `screenshot` | Rendered image of a page (when Chrome is available) |

The token is created once at `~/Library/Application Support/hunsu/mcp-token` (0600) and reused.

## Shortcuts

| Key | Action |
|---|---|
| V / H | Select / hand |
| Esc | Clear selection |
| ⌘+ / ⌘− / ⌘0, pinch | Zoom in / out / 100% |
| ⌘\\ | Render view only |
| ⌘S | Save |
| Enter / Shift+Enter | Send / new line |

## Design folder

```
designs/
  DESIGN.md, design.css   # design system
  *.html                  # pages (root .html files only)
  assets/                 # images and fonts
  .hunsu/                 # conversation ID, undo snapshots, versions
```

Pages keep CSS and JS inline and link only `design.css` and assets.

## LLM provider

The browser chat's LLM sits behind the `llm.Provider` interface. The provider is chosen only in `main.go`; the Anthropic API and others are added by implementing this interface ([`PLAN.md`](PLAN.md) section 5, Korean).

With Chrome available, the provider gets an MCP server holding only the `screenshot` tool (`/mcp/render`). After creating or substantially changing a page, the LLM looks at the render itself and fixes what it sees.

| Provider | Runs | Isolation | Models |
|---|---|---|---|
| claude | `claude -p` per request | Keeps your settings, hooks, and MCP servers out; runs no commands; only touches files in the folder | opus (default), sonnet, haiku. A small edit with haiku costs under a cent |
| codex | `codex exec` per request | Keeps your config, rules, and project AGENTS.md out; runs commands in a sandbox that can only write to the folder. Your global `~/.codex/AGENTS.md` is still loaded | gpt-5.6-sol (default), terra, luna. A small edit with luna takes about 14 s |

Measurements: [`PLAN.md`](PLAN.md) section 11.

## Development

```bash
go test ./... && node --test web/
```

| Path | Responsibility |
|---|---|
| `main.go` | Composition: flags, provider choice, server start |
| `internal/project` | Design folder access, blocking outside paths, assets, templates, design rules |
| `internal/watch` | File change detection |
| `internal/llm` | Provider contract |
| `internal/claude`, `internal/codex` | Providers (`claude -p`, `codex exec`) |
| `internal/history` | Snapshots and undo |
| `internal/render` | Headless Chrome screenshots |
| `internal/export` | ZIP and single-file HTML export |
| `internal/chat` | Request handling: one run at a time, prompt assembly, conversation |
| `internal/server` | HTTP API for the browser |
| `internal/ui` | Browser screen state and commands |
| `internal/mcp`, `internal/mcptools` | MCP server and auth, hunsu tools |
| `web/` | Browser UI (plain JS modules, no build) |

No external dependencies. Coding rules: [`CLAUDE.md`](CLAUDE.md); decisions and measurements: [`PLAN.md`](PLAN.md) (both Korean).

## Limitations

- localhost protection is partial (`/mcp` token, `/api/ui` origin check); the rest comes before the v1 release.
- Pages an agent creates without `open_page` appear in the list on the next interaction.
- Tested on macOS only. Only root `.html` files are pages.
- Classes or ids added by page JavaScript at runtime may end up in the selection path, so an LLM edit may not apply.
