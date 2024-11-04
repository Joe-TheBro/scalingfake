package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	appLogChan := make(chan string)

	go func() {
		err := runAppLogic(appLogChan)
		if err != nil {
			appLogChan <- fmt.Sprintf("Error: %v", err)
		}
		close(appLogChan)
	}()

	p := tea.NewProgram(initialChoiceModel(appLogChan))
	if err := p.Start(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
