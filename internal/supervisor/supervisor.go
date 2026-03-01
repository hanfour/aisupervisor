package supervisor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/audit"
	"github.com/hanfourmini/aisupervisor/internal/config"
	sessionctx "github.com/hanfourmini/aisupervisor/internal/context"
	"github.com/hanfourmini/aisupervisor/internal/detector"
	"github.com/hanfourmini/aisupervisor/internal/group"
	"github.com/hanfourmini/aisupervisor/internal/intervention"
	"github.com/hanfourmini/aisupervisor/internal/observer"
	"github.com/hanfourmini/aisupervisor/internal/role"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

type Event struct {
	SessionID    string
	SessionName  string
	Type         EventType
	Match        *detector.PromptMatch
	Decision     *ai.Decision
	Intervention *role.Intervention
	RoleID       string
	Error        error
	Timestamp    time.Time
}

type EventType string

const (
	EventDetected         EventType = "detected"
	EventDecision         EventType = "decision"
	EventAutoApproved     EventType = "auto_approved"
	EventSent             EventType = "sent"
	EventPaused           EventType = "paused"
	EventError            EventType = "error"
	EventRoleIntervention EventType = "role_intervention"
	EventRoleObservation  EventType = "role_observation"
)

type Supervisor struct {
	cfg          *config.Config
	client       tmux.TmuxClient
	sender       *tmux.Sender
	registry     *detector.Registry
	backend      ai.Backend
	auditor      *audit.Logger
	ctxStore     sessionctx.Store
	roleManager  *role.Manager
	groupManager *group.Manager
	roleResolver *role.SessionRoleResolver
	observers    []observer.Observer
	executor     *intervention.Executor
	dryRun       bool

	events chan Event

	mu              sync.Mutex
	debounce        map[string]time.Time
	lastActivity    map[string]time.Time
	lastContent     map[string]string
	sessionContexts map[string]*sessionctx.SessionContext
	pendingEvents   map[string]*PendingEvent // sessionID → latest paused event
	sessions        map[string]*session.MonitoredSession // sessionID → session
}

// PendingEvent holds a paused event awaiting human approval.
type PendingEvent struct {
	SessionID    string
	Match        *detector.PromptMatch
	Decision     *ai.Decision
	Intervention *role.Intervention
	Timestamp    time.Time
}

func New(
	cfg *config.Config,
	client tmux.TmuxClient,
	registry *detector.Registry,
	backend ai.Backend,
	auditor *audit.Logger,
	dryRun bool,
	ctxStore sessionctx.Store,
	roleManager *role.Manager,
	groupManager *group.Manager,
	roleResolver ...*role.SessionRoleResolver,
) *Supervisor {
	sender := tmux.NewSender(client)

	// Build default observers
	observers := []observer.Observer{
		observer.NewPromptObserver(registry),
		observer.NewContentObserver(30 * time.Second),
	}

	var resolver *role.SessionRoleResolver
	if len(roleResolver) > 0 && roleResolver[0] != nil {
		resolver = roleResolver[0]
	}

	return &Supervisor{
		cfg:             cfg,
		client:          client,
		sender:          sender,
		registry:        registry,
		backend:         backend,
		auditor:         auditor,
		ctxStore:        ctxStore,
		roleManager:     roleManager,
		groupManager:    groupManager,
		roleResolver:    resolver,
		observers:       observers,
		executor:        intervention.NewExecutor(sender, dryRun),
		dryRun:          dryRun,
		events:          make(chan Event, 100),
		debounce:        make(map[string]time.Time),
		lastActivity:    make(map[string]time.Time),
		lastContent:     make(map[string]string),
		sessionContexts: make(map[string]*sessionctx.SessionContext),
		pendingEvents:   make(map[string]*PendingEvent),
		sessions:        make(map[string]*session.MonitoredSession),
	}
}

// GroupManager returns the group manager (may be nil).
func (s *Supervisor) GroupManager() *group.Manager {
	return s.groupManager
}

// RoleResolver returns the session role resolver (may be nil).
func (s *Supervisor) RoleResolver() *role.SessionRoleResolver {
	return s.roleResolver
}

func (s *Supervisor) Events() <-chan Event {
	return s.events
}

func (s *Supervisor) RoleManager() *role.Manager {
	return s.roleManager
}

func (s *Supervisor) Monitor(ctx context.Context, sess *session.MonitoredSession) {
	// Track session for approval
	s.mu.Lock()
	s.sessions[sess.ID] = sess
	s.mu.Unlock()

	// Initialize session context
	if s.cfg.Context.Enabled && s.ctxStore != nil {
		sc, err := s.ctxStore.GetOrCreate(
			sess.ID,
			s.cfg.Context.MaxDecisions,
			s.cfg.Context.MaxActivities,
		)
		if err != nil {
			log.Printf("context init error for %s: %v", sess.ID, err)
		} else {
			if sess.ProjectDir != "" {
				proj := sessionctx.DetectProject(sess.ProjectDir)
				sc.SetProject(proj)
				if err := s.ctxStore.Save(sc); err != nil {
					log.Printf("context save error for %s: %v", sess.ID, err)
				}
			}
			s.mu.Lock()
			s.sessionContexts[sess.ID] = sc
			s.mu.Unlock()
		}
	}

	poller := tmux.NewPoller(s.client, s.cfg.Polling.IntervalMs, s.cfg.Polling.ContextLines)
	updates := make(chan tmux.PaneUpdate, 10)

	go poller.Poll(ctx, sess.TmuxSession, sess.Window, sess.Pane, updates)

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			s.handleUpdate(ctx, sess, update)
		}
	}
}

func (s *Supervisor) handleUpdate(ctx context.Context, sess *session.MonitoredSession, update tmux.PaneUpdate) {
	// Update context: parse cwd from pane content and record periodic activity
	sc := s.getSessionContext(sess.ID)
	if sc != nil {
		if cwd := sessionctx.ParseWorkingDirectory(update.Content); cwd != "" {
			if sc.Snapshot().Project.Directory == "" || sc.Snapshot().Project.Directory != cwd {
				proj := sessionctx.DetectProject(cwd)
				sc.SetProject(proj)
			}
		}

		interval := time.Duration(s.cfg.Context.ActivityIntervalSec) * time.Second
		if interval > 0 {
			s.mu.Lock()
			last := s.lastActivity[sess.ID]
			s.mu.Unlock()
			if time.Since(last) >= interval {
				summary := sessionctx.SummarizeActivity(update.Content, 500)
				if summary != "" {
					sc.AddActivity(sessionctx.ActivitySummary{
						Timestamp: time.Now(),
						Summary:   summary,
					})
				}
				s.mu.Lock()
				s.lastActivity[sess.ID] = time.Now()
				s.mu.Unlock()
			}
		}
	}

	// Run observer pipeline
	s.mu.Lock()
	prevContent := s.lastContent[sess.ID]
	s.lastContent[sess.ID] = update.Content
	s.mu.Unlock()

	var obsEvents []observer.ObservationEvent
	for _, obs := range s.observers {
		events := obs.Observe(update.Content, prevContent)
		for i := range events {
			events[i].SessionID = sess.ID
		}
		obsEvents = append(obsEvents, events...)
	}

	// Categorize events
	var promptEvent *observer.ObservationEvent
	var proactiveEvents []observer.ObservationEvent

	for i := range obsEvents {
		switch obsEvents[i].Type {
		case observer.ObservationPrompt:
			promptEvent = &obsEvents[i]
		case observer.ObservationError, observer.ObservationIdle, observer.ObservationInputReady, observer.ObservationContentChanged:
			proactiveEvents = append(proactiveEvents, obsEvents[i])
		}
	}

	// Reactive path: prompt detected
	if promptEvent != nil {
		s.handleReactive(ctx, sess, sc, promptEvent)
	}

	// Proactive path: non-prompt observations
	if len(proactiveEvents) > 0 {
		s.handleProactive(ctx, sess, sc, proactiveEvents, update.Content)
	}
}

func (s *Supervisor) handleReactive(ctx context.Context, sess *session.MonitoredSession, sc *sessionctx.SessionContext, promptEvent *observer.ObservationEvent) {
	match := promptEvent.Prompt
	if match == nil {
		return
	}

	// Debounce
	debounceKey := fmt.Sprintf("%s:%s", sess.ID, match.Summary)
	s.mu.Lock()
	if last, exists := s.debounce[debounceKey]; exists && time.Since(last) < 2*time.Second {
		s.mu.Unlock()
		return
	}
	s.debounce[debounceKey] = time.Now()
	s.mu.Unlock()

	s.emit(Event{
		SessionID:   sess.ID,
		SessionName: sess.Name,
		Type:        EventDetected,
		Match:       match,
		Timestamp:   time.Now(),
	})

	obs := role.Observation{
		PaneContent:    promptEvent.PaneContent,
		Prompt:         match,
		SessionContext: sc,
		SessionName:    sess.Name,
		TaskGoal:       sess.TaskGoal,
	}

	var iv *role.Intervention
	var err error
	if s.groupManager != nil {
		iv, err = s.groupManager.EvaluateWithGroups(ctx, obs, sess.ID)
	} else if s.roleResolver != nil {
		sessionRoles := s.roleResolver.RolesForSession(sess.ID)
		iv, err = s.roleManager.EvaluateReactiveFiltered(ctx, obs, sessionRoles)
	} else {
		iv, err = s.roleManager.EvaluateReactive(ctx, obs)
	}
	if err != nil {
		s.emit(Event{
			SessionID:   sess.ID,
			SessionName: sess.Name,
			Type:        EventError,
			Match:       match,
			Error:       err,
			Timestamp:   time.Now(),
		})
		return
	}

	if iv == nil {
		return
	}

	decision := interventionToDecision(iv, match)
	autoApproved := role.IsAutoApproved(iv)

	if autoApproved {
		s.emit(Event{
			SessionID:    sess.ID,
			SessionName:  sess.Name,
			Type:         EventAutoApproved,
			Match:        match,
			Decision:     decision,
			Intervention: iv,
			RoleID:       iv.RoleID,
			Timestamp:    time.Now(),
		})
		s.recordDecision(sc, match, decision, true)
		s.respond(sess, match, decision, iv, true)
		return
	}

	if decision.Confidence < s.cfg.Decision.ConfidenceThreshold {
		s.emit(Event{
			SessionID:    sess.ID,
			SessionName:  sess.Name,
			Type:         EventPaused,
			Match:        match,
			Decision:     decision,
			Intervention: iv,
			RoleID:       iv.RoleID,
			Timestamp:    time.Now(),
		})
		s.recordDecision(sc, match, decision, false)

		// Store for manual approval
		s.mu.Lock()
		s.pendingEvents[sess.ID] = &PendingEvent{
			SessionID:    sess.ID,
			Match:        match,
			Decision:     decision,
			Intervention: iv,
			Timestamp:    time.Now(),
		}
		s.mu.Unlock()
		return
	}

	s.emit(Event{
		SessionID:    sess.ID,
		SessionName:  sess.Name,
		Type:         EventDecision,
		Match:        match,
		Decision:     decision,
		Intervention: iv,
		RoleID:       iv.RoleID,
		Timestamp:    time.Now(),
	})

	s.recordDecision(sc, match, decision, false)
	s.respond(sess, match, decision, iv, false)
}

func (s *Supervisor) handleProactive(ctx context.Context, sess *session.MonitoredSession, sc *sessionctx.SessionContext, events []observer.ObservationEvent, paneContent string) {
	// Emit observation events
	for _, e := range events {
		s.emit(Event{
			SessionID:   sess.ID,
			SessionName: sess.Name,
			Type:        EventRoleObservation,
			Timestamp:   time.Now(),
		})
		_ = e // used for event emission
	}

	obs := role.Observation{
		PaneContent:    paneContent,
		SessionContext: sc,
		SessionName:    sess.Name,
		TaskGoal:       sess.TaskGoal,
	}

	interventions, err := s.roleManager.EvaluateProactive(ctx, obs)
	if err != nil {
		s.emit(Event{
			SessionID:   sess.ID,
			SessionName: sess.Name,
			Type:        EventError,
			Error:       err,
			Timestamp:   time.Now(),
		})
		return
	}

	for _, iv := range interventions {
		s.emit(Event{
			SessionID:    sess.ID,
			SessionName:  sess.Name,
			Type:         EventRoleIntervention,
			Intervention: iv,
			RoleID:       iv.RoleID,
			Timestamp:    time.Now(),
		})

		// Execute via executor
		if err := s.executor.Execute(sess.TmuxSession, sess.Window, sess.Pane, iv); err != nil {
			s.emit(Event{
				SessionID:   sess.ID,
				SessionName: sess.Name,
				Type:        EventError,
				Error:       fmt.Errorf("proactive intervention failed: %w", err),
				Timestamp:   time.Now(),
			})
		}
	}
}

func interventionToDecision(iv *role.Intervention, match *detector.PromptMatch) *ai.Decision {
	d := &ai.Decision{
		Reasoning:  iv.Reasoning,
		Confidence: iv.Confidence,
	}

	for _, opt := range match.Options {
		if opt.Key == iv.OptionKey {
			d.ChosenOption = opt
			return d
		}
	}

	d.ChosenOption = detector.ResponseOption{Key: iv.OptionKey, Label: "auto"}
	return d
}

func (s *Supervisor) recordDecision(sc *sessionctx.SessionContext, match *detector.PromptMatch, decision *ai.Decision, auto bool) {
	if sc == nil {
		return
	}
	sc.AddDecision(sessionctx.DecisionRecord{
		Timestamp:  time.Now(),
		Summary:    match.Summary,
		ChosenKey:  decision.ChosenOption.Key,
		Reasoning:  decision.Reasoning,
		Confidence: decision.Confidence,
		Auto:       auto,
	})
	if s.ctxStore != nil {
		go func() {
			if err := s.ctxStore.Save(sc); err != nil {
				log.Printf("context save error: %v", err)
			}
		}()
	}
}

func (s *Supervisor) getSessionContext(sessionID string) *sessionctx.SessionContext {
	if !s.cfg.Context.Enabled {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionContexts[sessionID]
}

func (s *Supervisor) respond(sess *session.MonitoredSession, match *detector.PromptMatch, decision *ai.Decision, iv *role.Intervention, autoApprove bool) {
	entry := audit.Entry{
		SessionID:        sess.ID,
		SessionName:      sess.Name,
		PromptType:       match.Type,
		Summary:          match.Summary,
		ChosenKey:        decision.ChosenOption.Key,
		ChosenLabel:      decision.ChosenOption.Label,
		Reasoning:        decision.Reasoning,
		Confidence:       decision.Confidence,
		AutoApprove:      autoApprove,
		DryRun:           s.dryRun,
		Backend:          s.backend.Name(),
		RoleID:           iv.RoleID,
		InterventionType: string(iv.Type),
	}

	if err := s.auditor.Log(entry); err != nil {
		log.Printf("audit log error: %v", err)
	}

	if err := s.executor.Execute(sess.TmuxSession, sess.Window, sess.Pane, iv); err != nil {
		s.emit(Event{
			SessionID:   sess.ID,
			SessionName: sess.Name,
			Type:        EventError,
			Error:       fmt.Errorf("send failed: %w", err),
			Timestamp:   time.Now(),
		})
	} else {
		s.emit(Event{
			SessionID:    sess.ID,
			SessionName:  sess.Name,
			Type:         EventSent,
			Match:        match,
			Decision:     decision,
			Intervention: iv,
			RoleID:       iv.RoleID,
			Timestamp:    time.Now(),
		})
	}
}

// ApprovePaused approves a paused event for a given session, optionally overriding the option key.
func (s *Supervisor) ApprovePaused(sessionID string, optionKey string) error {
	s.mu.Lock()
	pending, ok := s.pendingEvents[sessionID]
	sess := s.sessions[sessionID]
	delete(s.pendingEvents, sessionID)
	s.mu.Unlock()

	if !ok || pending == nil {
		return fmt.Errorf("no pending event for session %s", sessionID)
	}
	if sess == nil {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Override option key if provided
	iv := pending.Intervention
	if optionKey != "" {
		iv.OptionKey = optionKey
	}

	decision := interventionToDecision(iv, pending.Match)

	s.emit(Event{
		SessionID:    sessionID,
		SessionName:  sess.Name,
		Type:         EventDecision,
		Match:        pending.Match,
		Decision:     decision,
		Intervention: iv,
		RoleID:       iv.RoleID,
		Timestamp:    time.Now(),
	})

	sc := s.getSessionContext(sessionID)
	s.recordDecision(sc, pending.Match, decision, false)
	s.respond(sess, pending.Match, decision, iv, false)
	return nil
}

// GetPendingEvents returns all currently pending (paused) events.
func (s *Supervisor) GetPendingEvents() []*PendingEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*PendingEvent, 0, len(s.pendingEvents))
	for _, pe := range s.pendingEvents {
		result = append(result, pe)
	}
	return result
}

func (s *Supervisor) emit(e Event) {
	select {
	case s.events <- e:
	default:
	}
}
