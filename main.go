package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

// -----------------------------------------------------------------------
// Styles
// -----------------------------------------------------------------------

var (
	accent = lipgloss.Color("#819C91")
	muted  = lipgloss.Color("#555555")
	done   = lipgloss.Color("#5A9E6F")
	dim    = lipgloss.Color("#D0D0D0")

	styleTabActive = lipgloss.NewStyle().Foreground(accent).Bold(true).PaddingLeft(1).PaddingRight(1)
	styleTabInactive = lipgloss.NewStyle().Foreground(muted).PaddingLeft(1).PaddingRight(1)
	styleTabBar    = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(muted).
			MarginBottom(1)

	styleHint     = lipgloss.NewStyle().Foreground(muted)
	styleCursor   = lipgloss.NewStyle().Foreground(accent).Bold(true)
	styleDone     = lipgloss.NewStyle().Foreground(done).Strikethrough(true)
	stylePending  = lipgloss.NewStyle().Foreground(dim)
	styleBrief    = lipgloss.NewStyle().Foreground(dim)
	styleErr      = lipgloss.NewStyle().Foreground(lipgloss.Color("#AA4444"))
	styleLogTime  = lipgloss.NewStyle().Foreground(accent)
	styleLogText  = lipgloss.NewStyle().Foreground(muted)
	styleSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).PaddingLeft(1).PaddingRight(1).Background(accent).Bold(true)
	styleSuccess  = lipgloss.NewStyle().Foreground(done)

	stylePickerTitle = lipgloss.NewStyle().Foreground(accent).Bold(true).MarginBottom(1)
	stylePickerItem  = lipgloss.NewStyle().Foreground(dim).PaddingLeft(2).PaddingRight(2)
	stylePickerSel   = lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(accent).Bold(true).PaddingLeft(2).PaddingRight(1)
	stylePickerHint  = lipgloss.NewStyle().Foreground(muted).MarginTop(1)
	stylePickerKey   = lipgloss.NewStyle().Foreground(accent).PaddingRight(1)
)

// -----------------------------------------------------------------------
// Screen
// -----------------------------------------------------------------------

type screen int

const (
	screenMain   screen = iota
	screenPicker        // model picker overlay
)

// -----------------------------------------------------------------------
// Views (main screen)
// -----------------------------------------------------------------------

type view int

const (
	viewBriefing view = iota
	viewPages
	viewTasks
	viewDone
)

var viewNames = []string{"Briefing", "Pages", "Tasks", "Done"}

// -----------------------------------------------------------------------
// Messages
// -----------------------------------------------------------------------

type fetchedPagesMsg struct {
	pages []NotionPage
	err   error
}

type summaryMsg struct {
	summary string
	err     error
}

type savedTasksMsg struct{ err error }

// -----------------------------------------------------------------------
// Data types
// -----------------------------------------------------------------------

type Task struct {
	Text string
	Done bool
}

type LogEntry struct {
	Text string
	At   time.Time
}

// -----------------------------------------------------------------------
// Model
// -----------------------------------------------------------------------

type model struct {
	width  int
	height int

	activeScreen screen
	activeView   view

	// fetch / summarize state
	loading     bool
	summarizing bool
	spinner     spinner.Model
	pages       []NotionPage
	summary     string
	fetchErr    string
	saveMsg     string // transient feedback after saving

	// viewports
	briefViewport viewport.Model
	pagesViewport viewport.Model
	taskViewport  viewport.Model
	doneViewport  viewport.Model

	pageCursor int

	tasks  []Task
	cursor int

	inputActive bool
	input       textinput.Model

	log []LogEntry

	// model picker
	pickerCursor  int
	pickerKeyInput textinput.Model // for entering API keys
	pickerKeyMode  bool            // true when typing a key
	selectedModel  AIModel
	cfg            Config
}

func newModel() model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(accent)

	ti := textinput.New()
	ti.Placeholder = "New task..."
	ti.CharLimit = 120

	keyInput := textinput.New()
	keyInput.Placeholder = "Paste API key here..."
	keyInput.CharLimit = 200
	keyInput.EchoMode = textinput.EchoPassword
	keyInput.EchoCharacter = '•'

	// Load persisted config and tasks
	cfg, _ := loadConfig()
	if cfg.SelectedModel < 0 || cfg.SelectedModel >= len(AvailableModels) {
		cfg.SelectedModel = 0
	}
	selectedModel := AvailableModels[cfg.SelectedModel]

	tasks, log, _ := loadTasks()

	return model{
		loading:        true,
		spinner:        sp,
		input:          ti,
		pickerKeyInput: keyInput,
		activeView:     viewPages,
		activeScreen:   screenMain,
		briefViewport:  viewport.New(80, 10),
		pagesViewport:  viewport.New(80, 10),
		taskViewport:   viewport.New(80, 10),
		doneViewport:   viewport.New(80, 10),
		selectedModel:  selectedModel,
		pickerCursor:   cfg.SelectedModel,
		cfg:            cfg,
		tasks:          tasks,
		log:            log,
	}
}

// -----------------------------------------------------------------------
// Init
// -----------------------------------------------------------------------

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, doFetchPages)
}

func doFetchPages() tea.Msg {
	_ = godotenv.Load()
	pages, err := fetchNotionPages()
	return fetchedPagesMsg{pages: pages, err: err}
}

func doSummarize(page NotionPage, tasks []Task, model AIModel) tea.Cmd {
	return func() tea.Msg {
		content, err := fetchPageContent(page.ID)
		if err != nil {
			return summaryMsg{err: err}
		}

		var pending, completed []string
		for _, t := range tasks {
			if t.Done {
				completed = append(completed, "- "+t.Text)
			} else {
				pending = append(pending, "- "+t.Text)
			}
		}
		taskContext := ""
		if len(pending) > 0 {
			taskContext += "\n\nPending tasks:\n" + strings.Join(pending, "\n")
		}
		if len(completed) > 0 {
			taskContext += "\n\nCompleted today:\n" + strings.Join(completed, "\n")
		}

		summary, err := summarizeWithAI(content+taskContext, model)
		return summaryMsg{summary: summary, err: err}
	}
}

func doSaveTasks(tasks []Task, log []LogEntry) tea.Cmd {
	return func() tea.Msg {
		err := saveTasks(tasks, log)
		return savedTasksMsg{err: err}
	}
}

// -----------------------------------------------------------------------
// Viewport rebuilders
// -----------------------------------------------------------------------

func (m *model) rebuildPagesViewport() {
	var lines []string
	for i, p := range m.pages {
		edited := ""
		if len(p.LastEditedTime) >= 10 {
			edited = p.LastEditedTime[:10]
		}
		row := fmt.Sprintf("%s  %s", edited, p.Title)
		if i == m.pageCursor {
			lines = append(lines, styleCursor.Render("> ")+styleSelected.Render(row))
		} else {
			lines = append(lines, "  "+styleHint.Render(row))
		}
	}
	if len(lines) == 0 {
		lines = []string{styleHint.Render("No pages found.")}
	}
	m.pagesViewport.SetContent(strings.Join(lines, "\n"))
}

func (m *model) rebuildTaskViewport() {
	var lines []string
	for i, t := range m.tasks {
		cursor := "  "
		if i == m.cursor {
			cursor = styleCursor.Render("> ")
		}
		if t.Done {
			lines = append(lines, cursor+styleDone.Render("[x] "+t.Text))
		} else {
			lines = append(lines, cursor+stylePending.Render("[ ] "+t.Text))
		}
	}
	if len(lines) == 0 {
		lines = []string{styleHint.Render("No tasks yet. Press n to add one.")}
	}
	m.taskViewport.SetContent(strings.Join(lines, "\n"))
}

func (m *model) rebuildDoneViewport() {
	var lines []string
	for _, e := range m.log {
		lines = append(lines,
			styleLogTime.Render(e.At.Format("15:04"))+" "+styleLogText.Render(e.Text),
		)
	}
	if len(lines) == 0 {
		lines = []string{styleHint.Render("Nothing done yet.")}
	}
	m.doneViewport.SetContent(strings.Join(lines, "\n"))
}

func (m *model) resizeViewports() {
	contentH := m.height - 6
	if contentH < 4 {
		contentH = 4
	}
	w := m.width - 4
	m.briefViewport.Width = w
	m.briefViewport.Height = contentH
	m.pagesViewport.Width = w
	m.pagesViewport.Height = contentH
	m.taskViewport.Width = w
	m.taskViewport.Height = contentH
	m.doneViewport.Width = w
	m.doneViewport.Height = contentH
	m.input.Width = w - 4
	m.pickerKeyInput.Width = w - 10
}

// -----------------------------------------------------------------------
// Update
// -----------------------------------------------------------------------

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewports()
		m.rebuildPagesViewport()
		m.rebuildTaskViewport()
		m.rebuildDoneViewport()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case fetchedPagesMsg:
		m.loading = false
		if msg.err != nil {
			m.fetchErr = msg.err.Error()
		} else {
			m.pages = msg.pages
			m.fetchErr = ""
		}
		m.rebuildPagesViewport()
		return m, nil

	case summaryMsg:
		m.summarizing = false
		if msg.err != nil {
			m.briefViewport.SetContent(styleErr.Render("error: " + msg.err.Error()))
		} else {
			m.summary = msg.summary
			m.briefViewport.SetContent(styleBrief.Render(m.summary))
		}
		return m, nil

	case savedTasksMsg:
		if msg.err != nil {
			m.saveMsg = styleErr.Render("save failed: " + msg.err.Error())
		} else {
			m.saveMsg = styleSuccess.Render("tasks saved")
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			// Auto-save on quit
			return m, tea.Sequence(doSaveTasks(m.tasks, m.log), tea.Quit)
		}

		// ---- Picker screen ----------------------------------------
		if m.activeScreen == screenPicker {
			return m.updatePicker(msg)
		}

		// ---- Main screen ------------------------------------------

		// Open picker with m key (not while typing)
		if msg.String() == "m" && !m.inputActive {
			m.activeScreen = screenPicker
			m.pickerKeyMode = false
			m.pickerKeyInput.Reset()
			m.pickerKeyInput.Blur()
			return m, nil
		}

		// View switching
		if !m.inputActive {
			switch msg.String() {
			case "1":
				m.activeView = viewBriefing
				return m, nil
			case "2":
				m.activeView = viewPages
				m.rebuildPagesViewport()
				return m, nil
			case "3":
				m.activeView = viewTasks
				m.rebuildTaskViewport()
				return m, nil
			case "4":
				m.activeView = viewDone
				m.rebuildDoneViewport()
				return m, nil
			case "tab":
				m.activeView = (m.activeView + 1) % 4
				m.rebuildPagesViewport()
				m.rebuildTaskViewport()
				m.rebuildDoneViewport()
				return m, nil
			case "s":
				// Manual save
				return m, doSaveTasks(m.tasks, m.log)
			case "q":
				return m, tea.Sequence(tea.Quit)
			}
		}

		// Task input
		if m.inputActive {
			switch msg.String() {
			case "enter":
				text := strings.TrimSpace(m.input.Value())
				if text != "" {
					m.tasks = append(m.tasks, Task{Text: text})
					m.input.Reset()
					m.rebuildTaskViewport()
				}
				m.inputActive = false
				m.input.Blur()
			case "esc":
				m.inputActive = false
				m.input.Blur()
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		// Per-view keys
		switch m.activeView {
		case viewBriefing:
			switch msg.String() {
			case "c":
				if m.summary != "" {
					_ = clipboard.WriteAll(m.summary)
				}
			default:
				var cmd tea.Cmd
				m.briefViewport, cmd = m.briefViewport.Update(msg)
				return m, cmd
			}

		case viewPages:
			switch msg.String() {
			case "j", "down":
				if m.pageCursor < len(m.pages)-1 {
					m.pageCursor++
					m.rebuildPagesViewport()
				}
			case "k", "up":
				if m.pageCursor > 0 {
					m.pageCursor--
					m.rebuildPagesViewport()
				}
			case "enter":
				if len(m.pages) > 0 {
					m.summarizing = true
					m.summary = ""
					m.briefViewport.SetContent(m.spinner.View() + " Summarizing...")
					m.activeView = viewBriefing
					return m, doSummarize(m.pages[m.pageCursor], m.tasks, m.selectedModel)
				}
			case "r":
				m.loading = true
				m.fetchErr = ""
				m.rebuildPagesViewport()
				return m, doFetchPages
			}

		case viewTasks:
			switch msg.String() {
			case "j", "down":
				if m.cursor < len(m.tasks)-1 {
					m.cursor++
					m.rebuildTaskViewport()
				}
			case "k", "up":
				if m.cursor > 0 {
					m.cursor--
					m.rebuildTaskViewport()
				}
			case " ", "enter":
				if len(m.tasks) > 0 {
					t := &m.tasks[m.cursor]
					t.Done = !t.Done
					if t.Done {
						m.log = append(m.log, LogEntry{Text: t.Text, At: time.Now()})
					} else {
						for i, e := range m.log {
							if e.Text == t.Text {
								m.log = append(m.log[:i], m.log[i+1:]...)
								break
							}
						}
					}
					m.rebuildTaskViewport()
					m.rebuildDoneViewport()
					if m.summary != "" && len(m.pages) > 0 {
						m.summarizing = true
						return m, doSummarize(m.pages[m.pageCursor], m.tasks, m.selectedModel)
					}
				}
			case "d":
				if len(m.tasks) > 0 {
					m.tasks = append(m.tasks[:m.cursor], m.tasks[m.cursor+1:]...)
					if m.cursor >= len(m.tasks) && m.cursor > 0 {
						m.cursor--
					}
					m.rebuildTaskViewport()
				}
			case "n":
				m.inputActive = true
				m.input.Focus()
			}

		case viewDone:
			var cmd tea.Cmd
			m.doneViewport, cmd = m.doneViewport.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// -----------------------------------------------------------------------
// Picker update
// -----------------------------------------------------------------------

func (m model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Key input mode — user is typing an API key
	if m.pickerKeyMode {
		switch msg.String() {
		case "enter":
			key := strings.TrimSpace(m.pickerKeyInput.Value())
			switch AvailableModels[m.pickerCursor].Provider {
			case ProviderClaude:
				m.cfg.AnthropicKey = key
			case ProviderOpenAI:
				m.cfg.OpenAIKey = key
			}
			m.pickerKeyInput.Reset()
			m.pickerKeyMode = false
			m.pickerKeyInput.Blur()
			_ = saveConfig(m.cfg)
		case "esc":
			m.pickerKeyMode = false
			m.pickerKeyInput.Blur()
		default:
			var cmd tea.Cmd
			m.pickerKeyInput, cmd = m.pickerKeyInput.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// Normal picker navigation
	switch msg.String() {
	case "esc", "m":
		m.activeScreen = screenMain
	case "j", "down":
		if m.pickerCursor < len(AvailableModels)-1 {
			m.pickerCursor++
		}
	case "k", "up":
		if m.pickerCursor > 0 {
			m.pickerCursor--
		}
	case "enter":
		chosen := AvailableModels[m.pickerCursor]
		// If provider needs a key and we don't have one, prompt for it
		needsKey := chosen.NeedsKey
		hasKey := false
		switch chosen.Provider {
		case ProviderClaude:
			hasKey = m.cfg.AnthropicKey != ""
		case ProviderOpenAI:
			hasKey = m.cfg.OpenAIKey != ""
		}
		if needsKey && !hasKey {
			m.pickerKeyMode = true
			m.pickerKeyInput.Focus()
			return m, nil
		}
		// Confirm selection
		m.selectedModel = chosen
		m.cfg.SelectedModel = m.pickerCursor
		_ = saveConfig(m.cfg)
		m.activeScreen = screenMain
		m.saveMsg = styleSuccess.Render("model set to " + chosen.Name)
	case "x":
		// Clear stored key for current selection
		switch AvailableModels[m.pickerCursor].Provider {
		case ProviderClaude:
			m.cfg.AnthropicKey = ""
		case ProviderOpenAI:
			m.cfg.OpenAIKey = ""
		}
		_ = saveConfig(m.cfg)
	}
	return m, nil
}

// -----------------------------------------------------------------------
// View
// -----------------------------------------------------------------------

func (m model) tabBar() string {
	var tabs []string
	for i, name := range viewNames {
		label := fmt.Sprintf("%d %s", i+1, name)
		if view(i) == m.activeView {
			tabs = append(tabs, styleTabActive.Render(label))
		} else {
			tabs = append(tabs, styleTabInactive.Render(label))
		}
	}
	bar := strings.Join(tabs, styleTabInactive.Render(" │ "))
	return styleTabBar.Width(m.width-4).Render(bar)
}

func (m model) hintLine() string {
	modelTag := stylePickerKey.Render("[" + m.selectedModel.Name + "]")
	saveFeedback := ""
	if m.saveMsg != "" {
		saveFeedback = "  " + m.saveMsg
	}

	switch m.activeView {
	case viewBriefing:
		return styleHint.Render("[j/k] scroll  •  [c] copy  •  1-4/tab switch  •  [m] model  •  [q] quit")+
			"  "+modelTag+saveFeedback
	case viewPages:
		return styleHint.Render("[j/k] navigate  •  [enter] summarize  •  [r] refresh  •  [m] model  •  [q] quit")+
			"  "+modelTag+saveFeedback
	case viewTasks:
		if m.inputActive {
			return styleHint.Render("enter confirm  •  esc cancel")
		}
		return styleHint.Render("[j/k] navigate  •  [space] toggle  •  [n] add  •  [d] delete  •  [s] save  •  [m] model  •  [q] quit")+
			"  "+modelTag+saveFeedback
	case viewDone:
		return styleHint.Render("[j/k] scroll  •  [s] save  •  [m] model  •  [q] quit")+"  "+modelTag+saveFeedback
	}
	return ""
}

func (m model) activeContent() string {
	switch m.activeView {
	case viewBriefing:
		if m.summarizing {
			return m.spinner.View() + " Summarizing with " + m.selectedModel.Name + "..."
		}
		if m.loading {
			return m.spinner.View() + " Fetching Notion..."
		}
		return m.briefViewport.View()

	case viewPages:
		if m.loading {
			return m.spinner.View() + " Fetching pages..."
		}
		content := m.pagesViewport.View()
		if m.fetchErr != "" {
			content += "\n" + styleErr.Render("error: "+m.fetchErr)
		}
		return content

	case viewTasks:
		content := m.taskViewport.View()
		if m.inputActive {
			content += "\n\n" + m.input.View()
		}
		return content

	case viewDone:
		return m.doneViewport.View()
	}
	return ""
}

func (m model) pickerView() string {
	var lines []string

	lines = append(lines, stylePickerTitle.Render("Select AI Model"))

	for i, am := range AvailableModels {
		label := am.Name
		// Show key status for providers that need one
		if am.NeedsKey {
			hasKey := false
			switch am.Provider {
			case ProviderClaude:
				hasKey = m.cfg.AnthropicKey != ""
			case ProviderOpenAI:
				hasKey = m.cfg.OpenAIKey != ""
			}
			if hasKey {
				label += styleSuccess.Render(" ✓ key set")
			} else {
				label += styleErr.Render(" no key")
			}
		}
		if i == m.pickerCursor {
			lines = append(lines, styleCursor.Render("> ")+stylePickerSel.Render(label))
		} else {
			lines = append(lines, stylePickerItem.Render(label))
		}
	}

	if m.pickerKeyMode {
		lines = append(lines, "")
		lines = append(lines, styleCursor.Render("> ")+m.pickerKeyInput.View())
		lines = append(lines, stylePickerHint.Render("paste key and press enter  •  esc cancel"))
	} else {
		lines = append(lines, "")
		lines = append(lines, stylePickerHint.Render("enter select  •  x clear key  •  esc close  •  j/k navigate"))
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(lines, "\n"))
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	if m.activeScreen == screenPicker {
		return m.pickerView()
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(
		m.tabBar() + "\n" +
			m.activeContent() + "\n\n" +
			m.hintLine(),
	)
}

// -----------------------------------------------------------------------
// Main
// -----------------------------------------------------------------------

func main() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}