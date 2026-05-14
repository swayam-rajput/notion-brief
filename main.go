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

var (
	accent = lipgloss.Color("#F5A623")
	muted  = lipgloss.Color("#555555")
	done   = lipgloss.Color("#5A9E6F")
	dim    = lipgloss.Color("#D0D0D0")
	border = lipgloss.Color("#2A2A2A")

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(accent)

	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1, 2).
			MarginBottom(1)

	styleLabel = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true).
			MarginBottom(1)

	styleBrief = lipgloss.NewStyle().
			Foreground(dim).
			Italic(true)

	styleHint = lipgloss.NewStyle().
			Foreground(muted).
			Italic(true)

	styleCursor = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)

	styleDone = lipgloss.NewStyle().
			Foreground(done).
			Strikethrough(true)

	stylePending = lipgloss.NewStyle().
			Foreground(dim)

	styleLogTime = lipgloss.NewStyle().
			Foreground(accent)

	styleLogText = lipgloss.NewStyle().
			Foreground(muted)

	styleErr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AA4444")).
			Italic(true)

	styleSelected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(accent).
			Bold(true)
)

type fetchedPagesMsg struct {
	pages []NotionPage
	err   error
}

type summaryMsg struct {
	summary string
	err     error
}

type Task struct {
	Text string
	Done bool
}

type LogEntry struct {
	Text string
	At   time.Time
}

type focusArea int

const (
	focusBriefing focusArea = iota
	focusPages
	focusTasks
	focusInput
)

type model struct {
	width  int
	height int

	loading     bool
	summarizing bool
	spinner     spinner.Model
	pages       []NotionPage
	summary     string
	fetchErr    string

	briefViewport viewport.Model
	pagesViewport viewport.Model
	taskViewport  viewport.Model

	tasks      []Task
	cursor     int
	pageCursor int
	focus      focusArea
	input      textinput.Model

	log []LogEntry
}

func newModel() model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(accent)

	ti := textinput.New()
	ti.Placeholder = "Type a task and press Enter..."
	ti.CharLimit = 120
	ti.Width = 50

	briefVP := viewport.New(80, 7)
	briefVP.SetContent("Select a page and press Enter to summarize.")

	pagesVP := viewport.New(80, 10)
	pagesVP.SetContent("Loading...")

	taskVP := viewport.New(80, 10)
	taskVP.SetContent("No tasks yet.")

	return model{
		loading:       true,
		spinner:       sp,
		input:         ti,
		focus:         focusPages,
		briefViewport: briefVP,
		pagesViewport: pagesVP,
		taskViewport:  taskVP,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, doFetchPages)
}

func doFetchPages() tea.Msg {
	_ = godotenv.Load()
	pages, err := fetchNotionPages()
	return fetchedPagesMsg{pages: pages, err: err}
}

func doSummarize(page NotionPage) tea.Cmd {
	return func() tea.Msg {
		content, err := fetchPageContent(page.ID)
		if err != nil {
			return summaryMsg{err: err}
		}

		summary, err := summarizeWithOllama(content)
		return summaryMsg{summary: summary, err: err}
	}
}

func (m *model) rebuildPagesViewport() {
	var lines []string

	for i, p := range m.pages {
		cursor := "  "
		edited := ""
		if len(p.LastEditedTime) >= 10 {
			edited = p.LastEditedTime[:10]
		}

		row := fmt.Sprintf("%s  %s", edited, p.Title)

		if i == m.pageCursor && m.focus == focusPages {
			cursor = styleCursor.Render("> ")
			lines = append(lines, cursor+styleSelected.Render(row))
		} else {
			lines = append(lines, cursor+row)
		}
	}

	if len(lines) == 0 {
		lines = append(lines, "No pages found.")
	}

	m.pagesViewport.SetContent(strings.Join(lines, "\n"))
}

func (m *model) rebuildTaskViewport() {
	var lines []string

	for i, t := range m.tasks {
		cursor := "  "

		if i == m.cursor && m.focus == focusTasks {
			cursor = styleCursor.Render("> ")
		}

		if t.Done {
			lines = append(lines, cursor+styleDone.Render("[x] "+t.Text))
		} else {
			lines = append(lines, cursor+stylePending.Render("[ ] "+t.Text))
		}
	}

	if len(lines) == 0 {
		lines = append(lines, "No tasks yet.")
	}

	m.taskViewport.SetContent(strings.Join(lines, "\n"))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = m.width - 14

		panelWidth := m.width - 10

		m.briefViewport.Width = panelWidth
		m.pagesViewport.Width = panelWidth
		m.taskViewport.Width = panelWidth

		m.rebuildPagesViewport()
		m.rebuildTaskViewport()

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
			m.fetchErr = msg.err.Error()
			m.briefViewport.SetContent(styleErr.Render("error: " + m.fetchErr))
		} else {
			m.summary = msg.summary
			m.fetchErr = ""
			m.briefViewport.SetContent(styleBrief.Render(m.summary))
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if msg.String() == "tab" {
			switch m.focus {
			case focusBriefing:
				m.focus = focusPages
			case focusPages:
				m.focus = focusTasks
			case focusTasks:
				m.focus = focusInput
				m.input.Focus()
			case focusInput:
				m.focus = focusBriefing
				m.input.Blur()
			}

			m.rebuildPagesViewport()
			m.rebuildTaskViewport()
			return m, nil
		}

		if m.focus == focusInput {
			switch msg.String() {
			case "enter":
				text := strings.TrimSpace(m.input.Value())
				if text != "" {
					m.tasks = append(m.tasks, Task{Text: text})
					m.input.Reset()
					m.focus = focusTasks
					m.input.Blur()
					m.rebuildTaskViewport()
				}
			case "esc":
				m.focus = focusTasks
				m.input.Blur()
				m.rebuildTaskViewport()
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		if m.focus == focusBriefing {
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
			return m, nil
		}

		if m.focus == focusPages {
			switch msg.String() {
			case "j", "down":
				if m.pageCursor < len(m.pages)-1 {
					m.pageCursor++
					if m.pageCursor > m.pagesViewport.YOffset+m.pagesViewport.Height-1 {
						m.pagesViewport.LineDown(1)
					}
					m.rebuildPagesViewport()
				}
			case "k", "up":
				if m.pageCursor > 0 {
					m.pageCursor--
					if m.pageCursor < m.pagesViewport.YOffset {
						m.pagesViewport.LineUp(1)
					}
					m.rebuildPagesViewport()
				}
			case "enter":
				if len(m.pages) > 0 {
					m.summarizing = true
					m.summary = ""
					m.briefViewport.SetContent("Summarizing...")
					return m, doSummarize(m.pages[m.pageCursor])
				}
			case "r":
				m.loading = true
				m.fetchErr = ""
				return m, doFetchPages
			}
			return m, nil
		}

		if m.focus == focusTasks {
			switch msg.String() {
			case "j", "down":
				if m.cursor < len(m.tasks)-1 {
					m.cursor++
					if m.cursor > m.taskViewport.YOffset+m.taskViewport.Height-1 {
						m.taskViewport.LineDown(1)
					}
					m.rebuildTaskViewport()
				}
			case "k", "up":
				if m.cursor > 0 {
					m.cursor--
					if m.cursor < m.taskViewport.YOffset {
						m.taskViewport.LineUp(1)
					}
					m.rebuildTaskViewport()
				}
			case " ", "enter":
				if len(m.tasks) > 0 {
					t := &m.tasks[m.cursor]
					t.Done = !t.Done

					if t.Done {
						m.log = append(m.log, LogEntry{
							Text: t.Text,
							At:   time.Now(),
						})
					}
					m.rebuildTaskViewport()
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
				m.focus = focusInput
				m.input.Focus()
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	w := m.width - 4

	var out []string

	out = append(out,
		styleHeader.Render(
			fmt.Sprintf("NOTION BRIEF  //  %s",
				time.Now().Format("Monday, January 2")),
		),
	)

	var briefContent string
	if m.summarizing {
		briefContent = m.spinner.View() + " Summarizing with Ollama..."
	} else if m.loading {
		briefContent = m.spinner.View() + " Fetching Notion..."
	} else {
		briefContent = m.briefViewport.View()
	}

	briefStyle := styleBox.Width(w)
	if m.focus == focusBriefing {
		briefStyle = briefStyle.BorderForeground(accent)
	}

	out = append(out,
		briefStyle.Render(
			styleLabel.Render("BRIEFING")+"\n"+briefContent+
				"\n"+styleHint.Render("j/k to scroll • [c] copy"),
		),
	)

	pagesStyle := styleBox.Width(w)
	if m.focus == focusPages {
		pagesStyle = pagesStyle.BorderForeground(accent)
	}

	out = append(out,
		pagesStyle.Render(
			styleLabel.Render("NOTION PAGES")+"\n"+
				m.pagesViewport.View()+"\n"+
				styleHint.Render("[enter] summarize • [tab] switch • j/k navigate • r refresh"),
		),
	)

	var logLines []string
	for _, e := range m.log {
		logLines = append(logLines,
			styleLogTime.Render(e.At.Format("15:04"))+
				" "+
				styleLogText.Render(e.Text),
		)
	}

	if len(logLines) == 0 {
		logLines = append(logLines, "Completed tasks show up here.")
	}

	out = append(out,
		styleBox.Width(w).Render(
			styleLabel.Render("DONE TODAY")+"\n"+
				strings.Join(logLines, "\n"),
		),
	)

	return lipgloss.NewStyle().
		Padding(1, 2).
		Render(strings.Join(out, "\n"))
}

func main() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}