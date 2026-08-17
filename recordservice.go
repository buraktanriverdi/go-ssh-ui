package main

import (
	"fmt"
	"sync"

	"go-ssh-ui/internal/record"
	"go-ssh-ui/internal/sshengine"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// RecordEvent carries a live "Kayıt" update for one session - fired while
// recording is active so the frontend can show something more useful than a
// silent toggle (see PLAN.md Faz 5 risk #7: recording must never be
// invisible). Kind is a record.EventKind ("prompt-detected"/"captured").
type RecordEvent struct {
	SessionID string `json:"sessionId"`
	Kind      string `json:"kind"`
	Label     string `json:"label"`
}

func init() {
	application.RegisterEvent[RecordEvent]("record:event")
}

// RecordService is the Wails-bound entry point for the "Kayıt" feature: it
// attaches a record.Recorder to a live sshengine session for the duration of
// a recording, and hands back the accumulated Snapshot when it stops. It
// never persists anything itself - the frontend's review dialog is what
// turns a Snapshot into an actual host + password entries, and only after
// the user confirms it (see RecordReviewDialog.svelte).
type RecordService struct {
	mu        sync.Mutex
	mgr       *sshengine.Manager
	recorders map[string]*record.Recorder
}

// NewRecordService wires RecordService to the same sshengine.Manager that
// backs TerminalService/FileService, since recording taps an already-open
// session's Write/output stream rather than owning a connection of its own.
func NewRecordService(mgr *sshengine.Manager) *RecordService {
	return &RecordService{mgr: mgr, recorders: make(map[string]*record.Recorder)}
}

// Start begins recording sessionID - only meaningful for a local shell tab
// (see PLAN.md Faz 5 scope note), but nothing here enforces that; a
// recording on a session where the user never types a bare `ssh ...` line
// just ends up an empty Snapshot.
func (s *RecordService) Start(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.recorders[sessionID]; exists {
		return fmt.Errorf("bu oturum zaten kaydediliyor")
	}
	rec := record.New(func(e record.Event) {
		application.Get().Event.Emit("record:event", RecordEvent{
			SessionID: sessionID,
			Kind:      string(e.Kind),
			Label:     e.Label,
		})
	})
	s.recorders[sessionID] = rec
	s.mgr.SetRecorder(sessionID, rec)
	return nil
}

// Stop ends recording sessionID and returns everything captured.
func (s *RecordService) Stop(sessionID string) (record.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.recorders[sessionID]
	if !ok {
		return record.Snapshot{}, fmt.Errorf("bu oturum kaydedilmiyor")
	}
	delete(s.recorders, sessionID)
	s.mgr.ClearRecorder(sessionID)
	return rec.Snapshot(), nil
}
