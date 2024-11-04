package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type azureLocation struct {
	name string
	city string
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

type choiceModel struct {
	cursor    int
	locations []azureLocation
	choice    string
}

func (m choiceModel) Init() tea.Cmd {
	return nil
}

func (m choiceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = true
			return m, func() tea.Msg {
				return selectionMsg{choice: m.choices[m.cursor]}
			}
		}
	}
	return m, nil
}

func (m choiceModel) View() string {
	s := "Choose an option:\n\n"
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}
	s += "\nPress q to quit."
	return s
}

type logModel struct {
	logs    []string
	logChan chan string
}

func (m logModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	case selectionMsg:
		return logsModel{logs: []string{fmt.Sprintf("Selected: %s", msg.choice)}}, nil
	case logMsg:
		m.logs = append(m.logs, msg.content)
		return m, waitForLogMessage(m.logChan)
	}
	return m, nil
}

func (m logModel) View() string {
	s := "Logs:\n\n"
	for _, log := range m.logs {
		s += fmt.Sprintf("%s\n", log)
	}
	s += "\nPress q to quit."
	return s
}

type logMsg struct {
	content string
}

type selectionMsg struct {
	choice string
}

func initialChoiceModel() choiceModel {
	return choiceModel{
		locations: azureLocations,
	}
}

func (m logModel) Init() tea.Cmd {
	m.logChan = make(chan string)
	go runBackgroundTasks(m.logChan)
	return waitForLogMessage(m.logChan)
}

func waitForLogMessage(logChan <-chan string) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-logChan
		if !ok {
			return nil
		}
		return logMsg{content: msg}
	}
}
