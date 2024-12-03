package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type viewState int

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
	// item{title: "southcentralus", desc: "Texas"}, region got deprecated
	item{title: "canadacentral", desc: "Maine"},
	// item{title: "eastus", desc: "NYC"},
} //* If you want more go talk to Microsoft Support :)

const (
	initialView viewState = iota
	logView
)

type model struct {
	state viewState
	log   *log.Logger
	list  list.Model
}

func (m *model) switchToLogView() tea.Cmd {
	m.state = logView
	return nil
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
			// case "t":
			// 	log.Debug("t pressed")
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
		// clear the screen
		fmt.Print("\033[H\033[2J")
		return docStyle.Render() // This works, trust me
	default:
		return ""
	}
}

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	m := model{
		state: initialView,
		log:   log.Default(),
		list:  list.New(azureLocations, list.NewDefaultDelegate(), 0, 0),
	}
	m.list.Title = "Azure Locations"

	p := tea.NewProgram(&m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
