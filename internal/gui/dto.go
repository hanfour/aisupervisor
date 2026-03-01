package gui

import (
	"time"

	"github.com/hanfourmini/aisupervisor/internal/group"
	"github.com/hanfourmini/aisupervisor/internal/role"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
)

// EventDTO is a JSON-friendly representation of a supervisor event.
type EventDTO struct {
	SessionID   string `json:"sessionId"`
	SessionName string `json:"sessionName"`
	Type        string `json:"type"`
	Summary     string `json:"summary,omitempty"`
	ChosenKey   string `json:"chosenKey,omitempty"`
	Reasoning   string `json:"reasoning,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
	RoleID      string `json:"roleId,omitempty"`
	Error       string `json:"error,omitempty"`
	Timestamp   string `json:"timestamp"`
}

// EventToDTO converts a supervisor.Event to EventDTO.
func EventToDTO(e supervisor.Event) EventDTO {
	dto := EventDTO{
		SessionID:   e.SessionID,
		SessionName: e.SessionName,
		Type:        string(e.Type),
		Timestamp:   e.Timestamp.Format(time.RFC3339),
		RoleID:      e.RoleID,
	}

	if e.Match != nil {
		dto.Summary = e.Match.Summary
	}
	if e.Decision != nil {
		dto.ChosenKey = e.Decision.ChosenOption.Key
		dto.Reasoning = e.Decision.Reasoning
		dto.Confidence = e.Decision.Confidence
	}
	if e.Intervention != nil {
		dto.Confidence = e.Intervention.Confidence
		dto.Reasoning = e.Intervention.Reasoning
	}
	if e.Error != nil {
		dto.Error = e.Error.Error()
	}

	return dto
}

// DiscussionEventDTO is a JSON-friendly representation of a discussion event.
type DiscussionEventDTO struct {
	DiscussionID string  `json:"discussionId"`
	SessionID    string  `json:"sessionId"`
	GroupID      string  `json:"groupId"`
	Phase        string  `json:"phase"`
	RoleID       string  `json:"roleId"`
	RoleName     string  `json:"roleName"`
	Message      string  `json:"message"`
	Action       string  `json:"action"`
	Confidence   float64 `json:"confidence"`
	Timestamp    string  `json:"timestamp"`
}

// DiscussionEventToDTO converts a group.DiscussionEvent to DTO.
func DiscussionEventToDTO(e group.DiscussionEvent) DiscussionEventDTO {
	return DiscussionEventDTO{
		DiscussionID: e.DiscussionID,
		SessionID:    e.SessionID,
		GroupID:      e.GroupID,
		Phase:        string(e.Phase),
		RoleID:       e.RoleID,
		RoleName:     e.RoleName,
		Message:      e.Message,
		Action:       e.Action,
		Confidence:   e.Confidence,
		Timestamp:    e.Timestamp.Format(time.RFC3339),
	}
}

// RoleDTO is a JSON-friendly role representation.
type RoleDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Mode     string `json:"mode"`
	Priority int    `json:"priority"`
	Avatar   string `json:"avatar,omitempty"`
}

// RoleToDTO converts a role.Role to RoleDTO.
func RoleToDTO(r role.Role) RoleDTO {
	return RoleDTO{
		ID:       r.ID(),
		Name:     r.Name(),
		Mode:     string(r.Mode()),
		Priority: r.Priority(),
		Avatar:   role.GetAvatar(r),
	}
}

// SessionDTO is a JSON-friendly session representation.
type SessionDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	TmuxSession string `json:"tmuxSession"`
	Window      int    `json:"window"`
	Pane        int    `json:"pane"`
	ToolType    string `json:"toolType"`
	TaskGoal    string `json:"taskGoal"`
	ProjectDir  string `json:"projectDir"`
	Status      string `json:"status"`
}

// SessionToDTO converts a session.MonitoredSession to SessionDTO.
func SessionToDTO(s *session.MonitoredSession) SessionDTO {
	return SessionDTO{
		ID:          s.ID,
		Name:        s.Name,
		TmuxSession: s.TmuxSession,
		Window:      s.Window,
		Pane:        s.Pane,
		ToolType:    s.ToolType,
		TaskGoal:    s.TaskGoal,
		ProjectDir:  s.ProjectDir,
		Status:      string(s.Status),
	}
}

// GroupDTO is a JSON-friendly group representation.
type GroupDTO struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	LeaderID            string   `json:"leaderId"`
	RoleIDs             []string `json:"roleIds"`
	DivergenceThreshold float64  `json:"divergenceThreshold"`
}

// GroupToDTO converts a group.Group to GroupDTO.
func GroupToDTO(g *group.Group) GroupDTO {
	return GroupDTO{
		ID:                  g.ID,
		Name:                g.Name,
		LeaderID:            g.LeaderID,
		RoleIDs:             g.RoleIDs,
		DivergenceThreshold: g.DivergenceThreshold,
	}
}

// DiscussionDTO is a JSON-friendly discussion representation.
type DiscussionDTO struct {
	ID        string `json:"id"`
	GroupID   string `json:"groupId"`
	SessionID string `json:"sessionId"`
	Phase     string `json:"phase"`
	CreatedAt string `json:"createdAt"`
}

// DiscussionToDTO converts a group.Discussion to DiscussionDTO.
func DiscussionToDTO(d *group.Discussion) DiscussionDTO {
	return DiscussionDTO{
		ID:        d.ID,
		GroupID:   d.GroupID,
		SessionID: d.SessionID,
		Phase:     string(d.Phase),
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
	}
}
