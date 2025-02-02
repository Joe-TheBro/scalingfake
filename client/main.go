package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var UIIPAddress string

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type viewState int

// type item struct {
// 	title, desc string
// }

// func (i item) Title() string       { return i.title }
// func (i item) Description() string { return i.desc }
// func (i item) FilterValue() string { return i.title }

// var azureLocations = []list.Item{
// 	item{title: "centralus", desc: "Chicago"},
// 	item{title: "westcentralus", desc: "Wyoming"},
// 	item{title: "westus2", desc: "Oregon"},
// 	item{title: "westus", desc: "Los Angeles"},
// 	item{title: "westus3", desc: "Arizona"},
// 	// item{title: "southcentralus", desc: "Texas"}, // region got deprecated
// 	item{title: "canadacentral", desc: "Maine"},
// 	// item{title: "eastus", desc: "NYC"},
// }

const (
	initialView viewState = iota
	logView
)

type model struct {
	state     viewState
	log       *log.Logger
	list      list.Model
	textinput textinput.Model
}

func (m *model) switchToLogView() tea.Cmd {
	m.state = logView
	return nil
}

func (m *model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.state == initialView {
				// Alternatively, you could use the list selection:
				// location := azureLocations[m.list.Cursor()].(item).title
				UIIPAddress = m.textinput.Value()
				UIIPAddress = strings.TrimSpace(UIIPAddress)
				fmt.Print(UIIPAddress)
				if UIIPAddress == "" {
					return m, tea.Quit
				}
				go background_main()
				return m, m.switchToLogView()
			}
		}
	case tea.WindowSizeMsg:
		// h, v := docStyle.GetFrameSize()
		// m.list.SetSize(msg.Width-h, msg.Height-v)
	} // <-- Closing the switch statement here

	var cmd tea.Cmd
	if m.state == initialView {
		m.textinput, cmd = m.textinput.Update(msg)
	}
	return m, cmd
}

func (m *model) View() string {
	switch m.state {
	case initialView:
		// You can switch to rendering the list by replacing the next line:
		// return docStyle.Render(m.list.View())
		return docStyle.Render(m.textinput.View())
	case logView:
		// Clear the screen and render a log view
		tea.ClearScreen()
		// return docStyle.Render(m.List.View())
		return docStyle.Render(m.textinput.View())
	default:
		return ""
	}
}

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	// Initialize and configure the text input.
	ti := textinput.New()
	ti.Placeholder = "Enter IP Address"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20

	// Initialize the list (even though it isn’t used in the current view).
	// l := list.New(azureLocations, list.NewDefaultDelegate(), 0, 0)
	// l.Title = "Azure Locations"

	m := model{
		state:     initialView,
		log:       log.Default(),
		// list:      l,
		textinput: ti,
	}

	p := tea.NewProgram(&m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
