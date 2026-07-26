package tui

import "github.com/Tinddd28/wend/internal/scan"

// ScanID — correlation ID для отличия сообщений актуального скана от
// «устаревших», присланных отменённым/предыдущим сканом (например, после
// того как пользователь перешёл в другую директорию до завершения обхода).
type ScanID int

// scanProgressMsg — партия частичных результатов, инкрементально мержится в DirTree.
type scanProgressMsg struct {
	ID      ScanID
	Entries []scan.Entry
}

// scanDoneMsg — скан для ID завершён (успешно или с ошибкой).
type scanDoneMsg struct {
	ID  ScanID
	Err error
}

type paneID int

const (
	paneSidebar paneID = iota
	paneMain
)
