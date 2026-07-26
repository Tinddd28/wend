package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tinddd28/wend/internal/tui"
)

// version подставляется при сборке релиза через -ldflags "-X main.version=...".
// Для сборок через `go install` он остаётся пустым, и версия читается из
// метаданных модуля, которые в этом случае проставляет сам Go.
var version = ""

func main() {
	showVersion := flag.Bool("version", false, "показать версию и выйти")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Printf("wend %s %s/%s\n", buildVersion(), runtime.GOOS, runtime.GOARCH)
		return
	}

	path := "."
	if flag.NArg() > 0 {
		path = flag.Arg(0)
	}

	if err := run(path); err != nil {
		fmt.Fprintln(os.Stderr, "wend:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Проверяем цель до запуска TUI: иначе на опечатке в пути открывался
	// полноэкранный интерфейс с пустым деревом и без внятной причины.
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s не является директорией", abs)
	}

	_, err = tea.NewProgram(tui.New(abs), tea.WithAltScreen()).Run()
	return err
}

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(dev)"
}

func usage() {
	fmt.Fprint(os.Stderr, `wend — интерактивный анализатор занятого места на диске.

Использование:
  wend [флаги] [путь]

Если путь не указан, анализируется текущая директория.

Флаги:
`)
	flag.PrintDefaults()
}
