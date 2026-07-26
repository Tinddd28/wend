package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tinddd28/wend/internal/tui"
)

func main() {
	path := "."
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wend:", err)
		os.Exit(1)
	}

	// Проверяем цель до запуска TUI: иначе на опечатке в пути открывался
	// полноэкранный интерфейс с пустым деревом и без внятной причины.
	info, err := os.Stat(abs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wend:", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "wend: %s не является директорией\n", abs)
		os.Exit(1)
	}

	if _, err := tea.NewProgram(tui.New(abs), tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "wend:", err)
		os.Exit(1)
	}
}
