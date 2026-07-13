package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"terminal-coding-agent/internal/agent"
	mylogger "terminal-coding-agent/internal/logger"
)

// ─────────────────────── Styles ───────────────────────

var (
	// Colour palette
	purple   = lipgloss.Color("#9B59B6")
	indigo   = lipgloss.Color("#5B5EA6")
	green    = lipgloss.Color("#2ECC71")
	cyan     = lipgloss.Color("#1ABC9C")
	yellow   = lipgloss.Color("#F1C40F")
	red      = lipgloss.Color("#E74C3C")
	white    = lipgloss.Color("#ECF0F1")
	grey     = lipgloss.Color("#7F8C8D")
	darkBg   = lipgloss.Color("#1A1A2E")
	panelBg  = lipgloss.Color("#16213E")

	// Header
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white).
			Background(indigo).
			Padding(0, 2).
			Width(100)

	// Chat viewport border
	chatBorderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Background(darkBg).
			Padding(0, 1)

	// Sidebar
	sidebarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(indigo).
			Background(panelBg).
			Padding(1, 1).
			Width(34)

	// Input box
	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(0, 1)

	// Footer bar
	footerStyle = lipgloss.NewStyle().
			Foreground(grey).
			Italic(true)

	// Message role labels
	roleAgent  = lipgloss.NewStyle().Bold(true).Foreground(green)
	roleTool   = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	roleUser   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#BDC3C7"))
	roleSystem = lipgloss.NewStyle().Foreground(yellow)
	roleError  = lipgloss.NewStyle().Bold(true).Foreground(red)
)

// ─────────────────────── Messages ───────────────────────

type agentEventMsg agent.AgentEvent
type agentDoneMsg struct{ err error }

// ─────────────────────── Model ───────────────────────

type tuiModel struct {
	ag        *agent.Agent
	ctx       context.Context
	viewport  viewport.Model
	input     textarea.Model
	eventChan chan agent.AgentEvent

	lines     []string // rendered history lines
	current   strings.Builder // current agent streaming text

	status    string
	activeTool string
	tokensIn  int64
	tokensOut int64
	steps     int

	width  int
	height int
	ready  bool
	busy   bool
}

func initialModel(ag *agent.Agent, ctx context.Context) tuiModel {
	ta := textarea.New()
	ta.Placeholder = "Type your message and press Enter…"
	ta.Focus()
	ta.SetHeight(3)
	ta.CharLimit = 2000
	ta.ShowLineNumbers = false

	return tuiModel{
		ag:     ag,
		ctx:    ctx,
		input:  ta,
		status: "Ready",
		lines:  []string{},
	}
}

// ─────────────────────── Init / Update / View ───────────────────────

func (m tuiModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		vpCmd    tea.Cmd
		inputCmd tea.Cmd
		cmds     []tea.Cmd
	)

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		sidebarWidth := 36
		chatWidth := m.width - sidebarWidth - 4

		inputHeight := 5
		headerHeight := 2
		footerHeight := 2
		chatHeight := m.height - inputHeight - headerHeight - footerHeight - 4

		if !m.ready {
			m.viewport = viewport.New(chatWidth, chatHeight)
			m.viewport.SetContent("")
			m.ready = true
		} else {
			m.viewport.Width = chatWidth
			m.viewport.Height = chatHeight
		}

		m.input.SetWidth(m.width - 4)
		headerStyle = headerStyle.Width(m.width - 2)
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.busy {
				return m, nil
			}
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			if text == "exit" || text == "quit" {
				return m, tea.Quit
			}

			m.input.Reset()
			m.addLine(roleUser.Render("You") + "\n" + text)
			m.busy = true
			m.steps = 0
			m.status = "Thinking..."
			m.current.Reset()

			// Open a fresh events channel and start the agent in a goroutine
			m.eventChan = make(chan agent.AgentEvent, 128)
			return m, tea.Batch(
				m.runAgent(text),
				m.waitForEvent(),
			)
		}

	case agentEventMsg:
		ev := agent.AgentEvent(msg)
		switch ev.Type {
		case agent.EventStatus:
			m.status = ev.Content
		case agent.EventTextChunk:
			m.current.WriteString(ev.Content)
			m.updateCurrentLine()
		case agent.EventToolCall:
			// Flush any current agent text
			if m.current.Len() > 0 {
				m.flushCurrentLine()
			}
			m.activeTool = ev.ToolName
			m.status = fmt.Sprintf("Calling %s…", ev.ToolName)
			m.addLine(roleTool.Render("→ "+ev.ToolName))
			m.steps++
		case agent.EventToolResult:
			m.activeTool = ""
			if ev.ToolError != nil {
				m.addLine(roleError.Render("  ✗ error: " + ev.ToolError.Error()))
			} else {
				m.addLine(roleTool.Render("  ✓ done"))
			}
		case agent.EventTokenUpdate:
			m.tokensIn += int64(ev.TokensIn)
			m.tokensOut += int64(ev.TokensOut)
		case agent.EventFinished:
			if m.current.Len() > 0 {
				m.flushCurrentLine()
			}
			m.addLine(roleSystem.Render("✓ " + ev.Content))
			m.status = "Ready"
			m.busy = false
		case agent.EventError:
			m.addLine(roleError.Render("✗ "+ev.Content))
			m.status = "Error"
			m.busy = false
		}

		// Keep listening unless we are done
		if m.busy {
			return m, m.waitForEvent()
		}
		return m, nil

	case agentDoneMsg:
		// Agent goroutine finished (channel closed)
		if m.current.Len() > 0 {
			m.flushCurrentLine()
		}
		if msg.err != nil {
			m.addLine(roleError.Render("Agent error: " + msg.err.Error()))
		}
		m.status = "Ready"
		m.busy = false
		return m, nil
	}

	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	m.input, inputCmd = m.input.Update(msg)
	cmds = append(cmds, inputCmd)

	return m, tea.Batch(cmds...)
}

func (m tuiModel) View() string {
	if !m.ready {
		return "\n  Initialising…"
	}

	// ── Header ──
	providerLabel := fmt.Sprintf("◈ Terminal Agent  [%s]", strings.ToUpper(m.ag.Provider))
	header := headerStyle.Render(providerLabel)

	// ── Chat viewport ──
	chatPanel := chatBorderStyle.
		Width(m.viewport.Width + 2).
		Height(m.viewport.Height + 2).
		Render(m.viewport.View())

	// ── Sidebar ──
	statusIcon := "●"
	statusColor := lipgloss.NewStyle().Foreground(green)
	if m.busy {
		statusColor = lipgloss.NewStyle().Foreground(yellow)
	}
	if m.status == "Error" {
		statusColor = lipgloss.NewStyle().Foreground(red)
	}

	activeTool := m.activeTool
	if activeTool == "" {
		activeTool = "—"
	}

	sidebar := sidebarStyle.Render(strings.Join([]string{
		lipgloss.NewStyle().Bold(true).Foreground(white).Render("◈ Status"),
		"",
		statusColor.Render(statusIcon + " " + m.status),
		"",
		lipgloss.NewStyle().Bold(true).Foreground(white).Render("◈ Current Tool"),
		roleTool.Render(activeTool),
		"",
		lipgloss.NewStyle().Bold(true).Foreground(white).Render("◈ Steps"),
		fmt.Sprintf("%d", m.steps),
		"",
		lipgloss.NewStyle().Bold(true).Foreground(white).Render("◈ Session Tokens"),
		roleSystem.Render(fmt.Sprintf("In  : %s", fmtNum(m.tokensIn))),
		roleSystem.Render(fmt.Sprintf("Out : %s", fmtNum(m.tokensOut))),
		roleSystem.Render(fmt.Sprintf("Tot : %s", fmtNum(m.tokensIn+m.tokensOut))),
	}, "\n"))

	// ── Main body (chat + sidebar side by side) ──
	body := lipgloss.JoinHorizontal(lipgloss.Top, chatPanel, "  ", sidebar)

	// ── Input ──
	inputBox := inputStyle.Width(m.width - 4).Render(m.input.View())

	// ── Footer ──
	footer := footerStyle.Render("  ctrl+c / exit  quit   ↑↓  scroll")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		body,
		inputBox,
		footer,
	)
}

// ─────────────────────── Helpers ───────────────────────

func (m *tuiModel) addLine(line string) {
	m.lines = append(m.lines, line)
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
	m.viewport.GotoBottom()
}

func (m *tuiModel) updateCurrentLine() {
	// Replace the last "current" entry if it's already there, else append a new one
	rendered := roleAgent.Render("Agent") + "\n" + m.current.String()
	if len(m.lines) > 0 && strings.HasPrefix(m.lines[len(m.lines)-1], roleAgent.Render("Agent")) {
		m.lines[len(m.lines)-1] = rendered
	} else {
		m.lines = append(m.lines, rendered)
	}
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
	m.viewport.GotoBottom()
}

func (m *tuiModel) flushCurrentLine() {
	text := m.current.String()
	m.current.Reset()
	if text == "" {
		return
	}
	rendered := roleAgent.Render("Agent") + "\n" + text
	// Replace streaming placeholder or add
	if len(m.lines) > 0 && strings.HasPrefix(m.lines[len(m.lines)-1], roleAgent.Render("Agent")) {
		m.lines[len(m.lines)-1] = rendered
	} else {
		m.lines = append(m.lines, rendered)
	}
	m.lines = append(m.lines, "")
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
	m.viewport.GotoBottom()
}

// runAgent launches the agent in a goroutine and drains the event channel.
func (m tuiModel) runAgent(input string) tea.Cmd {
	return func() tea.Msg {
		err := m.ag.HandleUserRequest(m.ctx, input, m.eventChan)
		close(m.eventChan)
		return agentDoneMsg{err: err}
	}
}

// waitForEvent reads the next event from the channel.
func (m tuiModel) waitForEvent() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-m.eventChan
		if !ok {
			return agentDoneMsg{}
		}
		return agentEventMsg(ev)
	}
}

func fmtNum(n int64) string {
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(ch))
	}
	return string(out)
}

// ─────────────────────── Entry Point ───────────────────────

// Run starts the Bubble Tea TUI.
func Run(ctx context.Context, ag *agent.Agent) {
	mylogger.Muted = true

	m := initialModel(ag, ctx)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		mylogger.Muted = false
		mylogger.Error("TUI error: %v", err)
	}

	mylogger.Muted = false

	// Print final token summary to the normal terminal after TUI exits
	ag.PrintSessionSummary()
}
