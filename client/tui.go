package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model int

const (
	listModel model = iota
	logModel
)

type azureLocation struct {
	name string
	city string
}

type appModel struct {
	model       model
	selectedIdx int
	logChannel  chan logMessage
}

type logMessage struct {
	message    string
	formatting lipgloss.Style
}

var azureLocations = []azureLocation{
	{"centralus", "Chicago"},
	{"westcentralus", "Wyoming"},
	{"westus2", "Oregon"},
	{"westus", "Los Angeles"},
	{"westus3", "Arizona"},
	{"southcentralus", "Texas"},
	{"canadacentral", "Maine"},
	{"eastus", "NYC"},
}

func (m appModel) Init() tea.Cmd {
	if m.model == listModel {
		return nil
	}
	return m.listenToLogs()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.model {
	case listModel:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "j", "down":
				if m.selectedIdx < len(azureLocations)-1 {
					m.selectedIdx++
				}
			case "k", "up":
				if m.selectedIdx > 0 {
					m.selectedIdx--
				}
			case "enter":
				m.model = logModel
				m.logChannel = make(chan logMessage)
				return m, m.listenToLogs()
			}
		}
	case logModel:
		switch msg := msg.(type) {
		case logMessage:
			style := msg.formatting
			fmt.Sprintf("%s", style)
		}
	}

	return m, nil
}

func (m appModel) View() string {
	switch m.model {
	case listModel:
		return m.listView()
	case logModel:
		return m.logView()
	default:
		return "Unknown model state"
	}
}

func (m appModel) listView() string {
	var output string
	output = "Select Azure Location (use up/down keys to navigate, 'enter' to select):\n\n"
	for i, loc := range azureLocations {
		if i == m.selectedIdx {
			cityString := fmt.Sprintf("%s (%s)", loc.name, loc.city)
			output += fmt.Sprintf("> %s\n", lipgloss.NewStyle().Bold(true).Render(cityString))
		} else {
			output += fmt.Sprintf("  %s (%s)\n", loc.name, loc.city)
		}
	}
	return output
}
