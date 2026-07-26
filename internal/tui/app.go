package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Tinddd28/wend/internal/humanize"
	"github.com/Tinddd28/wend/internal/scan"
	"github.com/Tinddd28/wend/internal/tui/mainview"
	"github.com/Tinddd28/wend/internal/tui/sidebar"
	"github.com/Tinddd28/wend/internal/tui/statusbar"
	"github.com/Tinddd28/wend/internal/tui/style"
)

// Геометрия внешнего каркаса: заголовок и статусбар по одной строке,
// у каждой панели рамка сверху и снизу.
const (
	headerHeight = 1
	footerHeight = statusbar.Height
	borderSize   = 2

	// Таблица слева хочет ~56 колонок под все свои колонки; диаграмме справа
	// нужно не меньше 30, иначе кольца вырождаются в пару пикселей.
	minSidebarWidth = 24
	maxSidebarWidth = 62
	minMainWidth    = 30
)

// RootModel — единственная точка входа bubbletea. Тяжёлые данные (дерево)
// хранятся по указателю, поэтому копирование Model в Update остаётся дешёвым.
type RootModel struct {
	tree *scan.DirTree

	sidebar sidebar.Model
	main    mainview.Model
	status  statusbar.Model

	focus paneID

	activeScanID ScanID
	scanResults  <-chan scan.Result
	cancelScan   context.CancelFunc

	width, height int
}

func New(rootPath string) RootModel {
	tree := scan.NewDirTree(rootPath)
	m := RootModel{
		tree:    tree,
		sidebar: sidebar.New(tree),
		main:    mainview.New(tree),
		status:  statusbar.New(),
		focus:   paneSidebar,
	}
	m.status.SetHelp(keys.HelpText())
	m.sidebar.SetFocused(true)
	m.main.SetFocused(false)
	return m
}

// startScanMsg запускает скан внутри Update, а не Init: Init не может
// мутировать модель (его возврат — только tea.Cmd, а не новая Model),
// поэтому activeScanID/cancelScan/scanResults, выставленные внутри Init,
// терялись бы вместе с отброшенной копией m.
type startScanMsg struct{ path string }

func (m RootModel) Init() tea.Cmd {
	path := m.tree.Root().Path
	return func() tea.Msg { return startScanMsg{path: path} }
}

// startScan запускает новый скан, инкрементируя activeScanID — сообщения от
// предыдущих (отменённых) сканов будут отброшены в Update по несовпадению ID.
func (m *RootModel) startScan(path string) tea.Cmd {
	if m.cancelScan != nil {
		m.cancelScan()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelScan = cancel
	m.activeScanID++
	id := m.activeScanID
	m.scanResults = scan.Scan(ctx, path)
	m.status.SetScanning(true)
	m.status.SetError(nil)
	return waitForScan(id, m.scanResults)
}

// waitForScan читает одно сообщение из канала скана и оборачивает его в
// tea.Msg. Update переиздаёт этот Cmd после каждого прогресса, чтобы
// продолжать слушать канал вплоть до scanDoneMsg.
func waitForScan(id ScanID, ch <-chan scan.Result) tea.Cmd {
	return func() tea.Msg {
		res, ok := <-ch
		if !ok {
			return nil
		}
		if res.Done {
			return scanDoneMsg{ID: id, Err: res.Err}
		}
		return scanProgressMsg{ID: id, Entries: res.Entries}
	}
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startScanMsg:
		return m, m.startScan(msg.path)

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case scanProgressMsg:
		if msg.ID != m.activeScanID {
			return m, nil // устаревшее сообщение от предыдущего скана
		}
		for _, e := range msg.Entries {
			m.tree.Upsert(e)
		}
		m.sidebar.Rebuild()
		m.syncSelection()
		return m, waitForScan(msg.ID, m.scanResults)

	case scanDoneMsg:
		if msg.ID != m.activeScanID {
			return m, nil
		}
		m.sidebar.Rebuild()
		m.status.SetScanning(false)
		// Ошибка обхода раньше молча терялась, и пустое дерево выглядело
		// как «просканировано, тут ничего нет».
		m.status.SetError(msg.Err)
		m.syncSelection()
		return m, nil

	case mainview.OpenMsg:
		m.sidebar.Reveal(msg.Path)
		m.main.SetPath(msg.Path)
		m.syncSelection()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m RootModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		if m.cancelScan != nil {
			m.cancelScan()
		}
		return m, tea.Quit

	case key.Matches(msg, keys.NextPane):
		if m.focus == paneSidebar {
			m.focus = paneMain
		} else {
			m.focus = paneSidebar
		}
		m.sidebar.SetFocused(m.focus == paneSidebar)
		m.main.SetFocused(m.focus == paneMain)
		return m, nil

	// Toggle и Rescan — глобальные: статусбар обещает их безусловно, а раньше
	// "v" работал только при фокусе на главной панели.
	case key.Matches(msg, keys.Toggle):
		m.main.ToggleMode()
		return m, nil

	case key.Matches(msg, keys.Rescan):
		// Reset обязателен: без него повторный обход просуммировал бы
		// размеры поверх уже накопленных.
		m.tree.Reset()
		m.sidebar.Reset()
		m.main.SetPath(m.tree.Root().Path)
		m.syncSelection()
		return m, m.startScan(m.tree.Root().Path)
	}

	var cmd tea.Cmd
	if m.focus == paneSidebar {
		m.sidebar, cmd = m.sidebar.Update(msg)
		if dir := m.sidebar.SelectedDir(); dir != nil {
			m.main.SetPath(dir.Path)
		}
	} else {
		m.main, cmd = m.main.Update(msg)
	}
	m.syncSelection()
	return m, cmd
}

// syncSelection протягивает выбранный в дереве узел в статусбар (детали) и в
// диаграмму (подсветка ветки), чтобы обе панели показывали одно и то же.
func (m *RootModel) syncSelection() {
	sel := m.sidebar.Selected()
	m.status.SetSelected(sel)
	m.status.SetCounts(m.tree.Stats())
	if sel != nil {
		m.main.SetHighlight(sel.Path)
	}
}

// layout раздаёт размеры панелям. Все величины проходят через max(...,1):
// в узком терминале width/3-2 уходит в минус, и lipgloss рисует мусор.
func (m *RootModel) layout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	sidebarOuter := min(max(m.width/2, minSidebarWidth), maxSidebarWidth)
	if sidebarOuter > m.width-minMainWidth {
		sidebarOuter = m.width / 2
	}
	mainOuter := m.width - sidebarOuter

	paneHeight := max(m.height-headerHeight-footerHeight-borderSize, 1)

	m.sidebar.SetSize(max(sidebarOuter-borderSize, 1), paneHeight)
	m.main.SetSize(max(mainOuter-borderSize, 1), paneHeight)
	m.status.SetSize(m.width, footerHeight)
}

func (m RootModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	root := m.tree.Root()
	title := fmt.Sprintf("%s  ·  %s  ·  %s элементов",
		root.Path, humanize.Bytes(root.Size), humanize.Count(root.Items))
	// Header имеет Padding(0,1); без обрезки длинный путь переносится на
	// вторую строку и сдвигает вниз обе панели вместе со статусбаром.
	header := style.Header.Width(m.width).Render(style.Clip(title, max(m.width-2, 0)))

	body := lipgloss.JoinHorizontal(lipgloss.Top, m.sidebar.View(), m.main.View())

	return lipgloss.JoinVertical(lipgloss.Left, header, body, m.status.View())
}
