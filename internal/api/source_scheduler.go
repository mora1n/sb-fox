package api

import (
	"context"
	"log"
	"time"

	"github.com/mora1n/sb-fox/internal/models"
)

const sourceSchedulePollInterval = 30 * time.Second

// StartSourceScheduler starts the in-process scheduler for subscription sources.
// The scheduler only persists scheduling state; a restart resumes from SQLite.
func (s *Server) StartSourceScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(sourceSchedulePollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				s.refreshDueSources(ctx, now)
			}
		}
	}()
}

func (s *Server) refreshDueSources(ctx context.Context, now time.Time) {
	sources, err := s.Store.DueSources(now)
	if err != nil {
		log.Printf("subscription scheduler: list due sources: %v", err)
		return
	}
	for _, source := range sources {
		if !s.beginSourceRefresh(source.ID) {
			continue
		}
		func() {
			defer s.endSourceRefresh(source.ID)
			if err := s.refreshSourceAndSchedule(ctx, source); err != nil {
				log.Printf("subscription scheduler: source %d: %v", source.ID, err)
			}
		}()
	}
}

func (s *Server) beginSourceRefresh(id int64) bool {
	s.sourceRefreshMu.Lock()
	defer s.sourceRefreshMu.Unlock()
	if s.sourceRefreshRunning == nil {
		s.sourceRefreshRunning = make(map[int64]bool)
	}
	if s.sourceRefreshRunning[id] {
		return false
	}
	s.sourceRefreshRunning[id] = true
	return true
}

func (s *Server) endSourceRefresh(id int64) {
	s.sourceRefreshMu.Lock()
	defer s.sourceRefreshMu.Unlock()
	delete(s.sourceRefreshRunning, id)
}

func (s *Server) refreshSourceAndSchedule(ctx context.Context, source *models.SubscriptionSource) error {
	owner, err := s.Store.GetUser(source.OwnerUserID)
	if err != nil {
		return err
	}
	_, _, _, _, err = s.refreshSourceNodesContext(ctx, owner, source)
	next := time.Now().UTC().Add(time.Duration(source.RefreshIntervalMinutes) * time.Minute)
	if err != nil {
		_ = s.Store.UpdateSourceFailure(source.ID, "error: "+err.Error())
	}
	if scheduleErr := s.Store.SetSourceNextRefresh(source.ID, &next); scheduleErr != nil {
		return scheduleErr
	}
	return err
}
