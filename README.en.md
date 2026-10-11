# hunsu

[한국어](README.md) | English

A local, HTML-native design tool: edit pages by hand or have an LLM build and refine them, live in your browser.

![hunsu demo: select a heading, ask to make it amber and larger, check at mobile width, then undo](docs/demo.gif)

- **Build and edit by talking**: create pages and a design system from an empty folder, then refine with "make this button bigger".
- **Select elements**: click in the render view to say "this".
- **Design system**: `DESIGN.md` (rules) and `design.css` (values), shared by every page and the LLM.
- **External agents (MCP)**: MCP clients such as Claude Code or Codex see your selection and edit files directly.
- **Undo**: roll back a whole request in one step.

The UI is currently Korean only; on-screen labels are given in parentheses.

## Requirements

- macOS
- [Claude Code](https://claude.com/claude-code) CLI (`claude`), logged in: LLM provider for the browser chat (PoC)
- Go 1.27+: build only

## Run

```bash
go build -o hunsu .
./hunsu ~/designs      # defaults to the current directory
```

Serves at `http://127.0.0.1:7777` and opens your browser. `-port 8888` changes the port, `-no-open` skips the browser.

## Usage

![hunsu screen: file panel, mobile render at 75% with a heading selected, editor and chat](docs/screenshot.png)

The screen has a file panel, render view, and editor & chat panel. Drag borders to resize (double-click to reset); collapse panels from the toolbar.

**Chat**

- Ask "make a café landing page", "add a menu page with the same design", "round the buttons on every page". New pages open automatically.
- Progress shows like "writing index.html · 12.4KB · 23s". While running, Send (보내기) becomes Cancel (취소).
- The conversation continues per project. New chat (새 대화) starts over.
- Undo (되돌리기) restores the state before the last request; repeat to go further back.
- Models come from the provider. The claude provider defaults to opus, with sonnet and haiku. A new page takes about 1 min 40 s with opus, about 30 s with haiku.

**Select an element**

Click an element with the select tool (V); a selection chip appears above the input. Ask "make this red". The selection survives page reloads; clear it with Esc or ×.

| Tool | Behavior |
|---|---|
| Select (V) | Click to select. Links and buttons don't fire |
| Hand (H) | Drag to move inside the page. A click without dragging works normally |
| Mobile / Desktop | 390px / full render width |
| − 100% + | Zoom 25–300% |

**Design system**

| File | Contents |
|---|---|
| `design.css` | CSS variables and shared classes, linked by every page |
| `DESIGN.md` | Rules in prose, followed by the LLM |

Both are pinned at the top of the file panel (click to create if missing) and sent to the LLM with every request.

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
| `get_context` | Folder path, pages, design system, visible page, selected element |
| `checkpoint` | Save before editing; restorable with the browser's Undo |
| `undo` | Restore the last checkpoint |
| `open_page` | Open a page in the browser |

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
  .hunsu/                 # conversation ID, undo snapshots
```

Pages keep CSS and JS inline and link only `design.css`.

## LLM provider

The browser chat's LLM sits behind the `llm.Provider` interface. The provider is chosen only in `main.go`; the Anthropic API, Codex, and others are added by implementing this interface ([`PLAN.md`](PLAN.md) section 5, Korean).

The current provider is the Claude Code CLI, running `claude -p` in the design folder per request. It keeps your settings, hooks, and MCP servers out, runs no commands, and only touches files in the folder. A small edit with haiku costs under a cent; a new page with opus about $0.3 ([`PLAN.md`](PLAN.md) section 11).

## Development

```bash
go test ./... && node --test web/
```

| Path | Responsibility |
|---|---|
| `main.go` | Composition: flags, provider choice, server start |
| `internal/project` | Design folder access, blocking outside paths, templates, design rules |
| `internal/watch` | File change detection |
| `internal/llm` | Provider contract |
| `internal/claude` | PoC provider (`claude -p`) |
| `internal/history` | Snapshots and undo |
| `internal/chat` | Request handling: one run at a time, prompt assembly, conversation |
| `internal/server` | HTTP API for the browser |
| `internal/ui` | Browser screen state and commands |
| `internal/mcp`, `internal/mcptools` | MCP server and auth, hunsu tools |
| `web/` | Browser UI (plain JS modules, no build) |

No external dependencies. Coding rules: [`CLAUDE.md`](CLAUDE.md); decisions and measurements: [`PLAN.md`](PLAN.md) (both Korean).

## Limitations

- localhost protection is partial (`/mcp` token, `/api/ui` origin check); the rest is the next milestone.
- Only one provider so far: the Claude Code CLI.
- Pages an agent creates without `open_page` appear in the list on the next interaction.
- Tested on macOS only. Only root `.html` files are pages.
- Classes or ids added by page JavaScript at runtime may end up in the selection path, so an LLM edit may not apply.
- Undo snapshots are never cleaned up.
