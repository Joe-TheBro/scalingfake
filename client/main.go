package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

var logChan chan logMessage

type viewState int

var logStyles = map[string]lipgloss.Style{
	"background": lipgloss.NewStyle().Background(lipgloss.Color("235")),
	"debug":      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	"info":       lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
	"error":      lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
	"warning":    lipgloss.NewStyle().Foreground(lipgloss.Color("220")),
	"critical":   lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
}

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

var azureLocations = []list.Item{
	item{title: "centralus", desc: "Chicago"},
	item{title: "westcentralus", desc: "Wyoming"},
	item{title: "westus2", desc: "Oregon"},
	item{title: "westus", desc: "Los Angeles"},
	item{title: "westus3", desc: "Arizona"},
	item{title: "southcentralus", desc: "Texas"},
	item{title: "canadacentral", desc: "Maine"},
	item{title: "eastus", desc: "NYC"},
}

const (
	initialView viewState = iota
	logView
)

type model struct {
	state     viewState
	logChan   chan logMessage
	logBuffer string
	list      list.Model
}

func (m *model) switchToLogView() tea.Cmd {
	m.state = logView
	return waitForLog(m.logChan)
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.state == initialView {
				location = azureLocations[m.list.Cursor()].(item).title
				go background_main()
				return m, m.switchToLogView()
			}
		case "t":
			sendLogMessage("This is a test message", lipgloss.NewStyle().Foreground(lipgloss.Color("205")))
		}

	case logMessage:
		if m.state == logView {
			formattedMsg := msg.format.Render(msg.msg)
			m.logBuffer += fmt.Sprintf("%s\n", formattedMsg)
			return m, waitForLog(m.logChan)
		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	if m.state == initialView {
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m *model) View() string {
	switch m.state {
	case initialView:
		return docStyle.Render(m.list.View())
	case logView:
		return docStyle.Render(m.logBuffer)
	default:
		return ""
	}
}

type logMessage struct {
	msg    string
	format lipgloss.Style
}

func waitForLog(logChan chan logMessage) tea.Cmd {
	return func() tea.Msg {
		return logMessage(<-logChan)
	}
}

func sendLogMessage(msg string, format lipgloss.Style) {
	logChan <- logMessage{msg: msg, format: format}
}

func main() {
	logChan = make(chan logMessage)
	logBuffer := ""

	m := model{
		state:     initialView,
		logChan:   logChan,
		logBuffer: logBuffer,
		list:      list.New(azureLocations, list.NewDefaultDelegate(), 0, 0),
	}
	m.list.Title = "Azure Locations"

	p := tea.NewProgram(&m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
