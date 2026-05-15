# notion-brief

A minimal terminal UI for your daily workflow. Pulls pages from your Notion workspace, summarizes them with an AI model of your choice, and gives you a focused task list that persists between sessions. Built in Go with Bubbletea.

```
 1 Briefing │ 2 Pages │ 3 Tasks │ 4 Done
────────────────────────────────────────
 You have three open design tasks and two
 unreviewed PRs. The highest priority item
 is the API refactor — it's blocking two
 other tasks. Consider starting there.

 j/k navigate  •  space toggle  •  n add  •  s save  •  m model  [Ollama — llama3.2]
```

---

## Features

- **Single-view TUI** — one focused panel at a time, switch with `1`/`2`/`3`/`4` or `tab`
- **Notion integration** — fetches your pages or a specific database, sorted by last edited
- **AI briefing** — summarizes page content + your current task state into a plain-language briefing
- **Multi-provider AI** — choose between Ollama (free, local), Claude, or OpenAI from inside the app
- **Persistent tasks** — tasks and done log survive between sessions, saved to disk automatically
- **Task-aware summaries** — the AI knows which tasks are pending and which are done, so the briefing stays relevant as you work
- **Copy to clipboard** — press `c` on the briefing view to copy the summary
- **Zero cloud dependency for Ollama** — runs entirely on your machine if you use a local model

---

## Project Structure

```
notion-brief/
├── main.go       # TUI — all screens, views, keybindings, state
├── ai.go         # AI provider layer — Ollama, Claude, OpenAI
├── notion.go     # Notion API — page listing and recursive block fetching
├── config.go     # Config persistence (~/.config/notion-brief/config.json)
├── tasks.go      # Task persistence (~/.local/share/notion-brief/tasks.json)
└── go.mod
```

---

## Installation

### Prerequisites

| Tool | Version | Link |
|------|---------|------|
| Go | 1.22+ | https://go.dev/dl |
| Git | any | https://git-scm.com |
| Ollama | latest (optional) | https://ollama.com/download |

### 1. Clone the repo

```bash
git clone https://github.com/yourname/notion-brief.git
cd notion-brief
```

### 2. Install Go dependencies

```bash
go mod tidy
```

This pulls in:
- `charmbracelet/bubbletea` — TUI framework
- `charmbracelet/bubbles` — text input, viewport, spinner components
- `charmbracelet/lipgloss` — terminal styling
- `atotto/clipboard` — clipboard support
- `joho/godotenv` — `.env` file loading

### 3. Set up Notion

**Create an integration:**

1. Go to https://www.notion.so/my-integrations
2. Click **New integration**
3. Give it a name (e.g. `notion-brief`), select your workspace
4. Copy the **Internal Integration Secret** — this is your `NOTION_API_KEY`

**Share pages with the integration:**

For every page or database you want to appear in the app:
1. Open the page in Notion
2. Click **Share** in the top right
3. Click **Invite**, search for your integration name, click **Invite**

If you skip this step, the app will return no pages.

**Optional — point to a specific database:**

If you want to fetch from one database instead of your whole workspace, copy the database ID from its URL:

```
https://www.notion.so/yourworkspace/THIS-IS-THE-DATABASE-ID?v=...
```

Set it as `NOTION_DATABASE_ID`.

### 4. Configure environment

Create a `.env` file in the project root:

```bash
# Required
NOTION_API_KEY=secret_your_key_here

# Optional — leave blank to search all shared pages
NOTION_DATABASE_ID=
```

API keys for Claude and OpenAI are entered inside the app via the model picker (`m` key) and stored in `~/.config/notion-brief/config.json`. You do not need to put them in `.env`.

### 5. (Optional) Set up Ollama

If you want to use a local model for free:

```bash
# Install Ollama (macOS/Linux)
curl -fsSL https://ollama.com/install.sh | sh

# Pull a model 
ollama pull llama3

# Start the Ollama server (keep this running in a separate terminal)
ollama serve
```

Other supported models out of the box: `mistral`, `phi4`, `gemma3`. You can add more by editing the `AvailableModels` slice in `ai.go`.

### 6. Build and run

**Run directly:**

```bash
source .env
go run .
```

**Build a binary** (recommended for daily use):

```bash
go build -o notion-brief .

# Move to somewhere on your PATH so you can run it from anywhere
mv notion-brief /usr/local/bin/
```

Then just run `notion-brief` from any terminal.

**Alias for convenience** (add to your `.bashrc` or `.zshrc`):

```bash
alias nb="cd /path/to/notion-brief && source .env && notion-brief"
```

---

## Usage

### Keybindings

#### Global

| Key | Action |
|-----|--------|
| `1` | Switch to Briefing view |
| `2` | Switch to Pages view |
| `3` | Switch to Tasks view |
| `4` | Switch to Done log |
| `tab` | Cycle through views |
| `m` | Open model picker |
| `s` | Save tasks to disk now |
| `ctrl+c` | Save and quit |

#### Briefing view

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll up and down |
| `c` | Copy briefing to clipboard |

#### Pages view

| Key | Action |
|-----|--------|
| `j` / `k` | Move cursor up and down |
| `enter` | Summarize selected page |
| `r` | Re-fetch pages from Notion |

#### Tasks view

| Key | Action |
|-----|--------|
| `j` / `k` | Move cursor up and down |
| `space` | Toggle task done / undone |
| `n` | Add a new task |
| `d` | Delete selected task |

#### Model picker

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate models |
| `enter` | Select model (prompts for key if needed) |
| `x` | Clear stored API key for current selection |
| `esc` / `m` | Close picker |

### Typical daily workflow

1. Run `notion-brief` (or your alias)
2. You land on the **Pages** view — pick a page with `j`/`k` and press `enter` to generate today's briefing
3. Press `1` to read the briefing
4. Press `3` to go to **Tasks** — add what you need to do with `n`, check things off with `space` as you go
5. Press `4` to see what you've completed today
6. `ctrl+c` when done — tasks save automatically

---

## AI Model Configuration

Press `m` at any time to open the model picker.

### Ollama (free, local, recommended if you're on a budget)

No API key needed. Just have `ollama serve` running. The default model is `llama3`. To use a different model, select it in the picker — if you want to add one not in the list, add an entry to `AvailableModels` in `ai.go`:

```go
{ProviderOllama, "Ollama — your-model", "your-model", false, ""},
```

Then `go build` again.

### Claude (Anthropic)

1. Get an API key at https://console.anthropic.com
2. Press `m` in the app, navigate to a Claude model, press `enter`
3. Paste your key when prompted — it is stored locally in `~/.config/notion-brief/config.json` with `0600` permissions

### OpenAI

1. Get an API key at https://platform.openai.com/api-keys
2. Same flow as Claude — press `m`, select a GPT model, enter key

To clear a stored key, navigate to that model in the picker and press `x`.

---

## Data Storage

| File | Contents |
|------|----------|
| `~/data/config.json` | Selected model index, API keys |
| `~/data/tasks.json` | Tasks (text + done state) and done log with timestamps |

Tasks are saved automatically on quit (`ctrl+c`) and manually with `s`. The done log resets each day is not currently automatic — see Future Scope below.

---

## Troubleshooting

**No pages appearing**

Make sure you have shared at least one page with your Notion integration (Share → Invite → your integration name). The API will return an empty list silently if nothing is shared.

**Ollama not reachable**

Run `ollama serve` in a separate terminal before launching the app. The error message in the briefing panel will tell you if it can't connect.

**API key errors for Claude / OpenAI**

Press `m`, navigate to the model, press `x` to clear the key, then `enter` to re-enter it. Keys are masked as you type.

**Blank screen on launch**

Resize your terminal window — the app needs at least ~80 columns to render properly.

---

## Future Scope

### Near-term

- **Daily log reset** — automatically archive the done log at midnight and start a fresh one each day, keeping a `~/.local/share/notion-brief/YYYY-MM-DD.json` history
- **Task editing** — press `e` on a selected task to rename it in-place rather than delete and re-add
- **Task reordering** — drag tasks up and down with `J`/`K` (shift variants) to set priority
- **Notion to-do sync** — write completed tasks back to Notion as checked `to_do` blocks, so your TUI and Notion stay in sync
- **Multiple page summaries** — select several pages and generate one unified briefing across all of them
- **Search** — press `/` to fuzzy-filter the pages list by title

### Medium-term

- **Additional Notion block types** — currently supports paragraphs, headings, lists, to-dos, quotes, callouts. Add support for tables, toggles, code blocks, and linked databases
- **Pomodoro timer integration** — press `p` on a task to start a focused work session with a countdown in the status bar
- **Weekly summary mode** — a dedicated view that summarizes everything edited in the last 7 days into a week-in-review
- **Export** — press `e` in the done log to export today's completed tasks as Markdown or plain text

### Longer-term

- **Slack / Gmail as additional sources** — reintroduce the multi-source architecture from the original `daybrief` project, with source toggles so you can enable only what you want
- **Plugin system** — a simple Go interface so additional data sources (Linear, GitHub Issues, Jira) can be dropped in as separate files without touching core code
- **Config TUI** — a proper settings screen inside the app for things like Notion database ID, refresh interval, and default view on startup, so you never have to edit `.env` manually
- **Streamed summaries** — use streaming APIs so the briefing appears word-by-word rather than waiting for the full response, making it feel faster on large pages

---

## Contributing

This is a personal daily-driver tool, but PRs are welcome. If you add a new AI provider or Notion block type, keep the same pattern — one function per provider in `ai.go`, one block type case in the switch in `notion.go`.

---
