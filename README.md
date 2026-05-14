# notion-brief

Terminal UI daily briefing pulled from Notion, summarized by Ollama. Free to run, no cloud costs.

## Setup

### 1. Prerequisites

- Go 1.22+  →  https://go.dev/dl
- Ollama     →  https://ollama.com/download
- llama3.2   →  `ollama pull llama3`

### 2. Notion API key

- Go to https://www.notion.so/my-integrations
- Create a new integration, copy the secret as NOTION_API_KEY
- Open the Notion pages/database you want, click Share → Invite your integration

### 3. Install dependencies

```bash
cd notion-brief
go mod tidy
```

### 4. Run

```bash
export NOTION_API_KEY=secret_...
export NOTION_DATABASE_ID=        # optional

go run .
```

Or as a binary you can run every morning:

```bash
go build -o notion-brief .
./notion-brief
```

## Keybindings

| Key        | Action                        |
|------------|-------------------------------|
| j / k      | Move cursor up and down       |
| n or tab   | Add a new task                |
| space      | Toggle task done / undone     |
| d          | Delete selected task          |
| r          | Re-fetch Notion + regenerate  |
| ctrl+c     | Quit                          |

## Notes

- Ollama must be running (`ollama serve`) before you launch the app
- Tasks reset on quit — they are in memory only
- If NOTION_DATABASE_ID is blank, it searches your entire workspace
