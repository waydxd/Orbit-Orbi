package orbi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UpdateAgentImpl is the concrete implementation of the Update Agent
type UpdateAgentImpl struct {
	name                 string
	tool                 UpdateTool
	llm                  LLMInterface
	validator            ConfirmationValidator
	pendingApprovals     map[string]*PendingApproval
	auditTrail           AuditTrail
	requiresConfirmation bool
}

// NewUpdateAgent creates a new Update Agent instance
func NewUpdateAgent(
	tool UpdateTool,
	llm LLMInterface,
	validator ConfirmationValidator,
	auditTrail AuditTrail,
) *UpdateAgentImpl {
	return &UpdateAgentImpl{
		name:                 "update-agent",
		tool:                 tool,
		llm:                  llm,
		validator:            validator,
		auditTrail:           auditTrail,
		pendingApprovals:     make(map[string]*PendingApproval),
		requiresConfirmation: true,
	}
}

// Name returns the agent's identifier
func (a *UpdateAgentImpl) Name() string {
	return a.name
}

// ShouldHandle returns true if this agent should handle the given intent
func (a *UpdateAgentImpl) ShouldHandle(intent *Intent) bool {
	return intent.Type == "write" || intent.Type == "modify" || intent.RequiresApproval
}

// Handle processes an update request
func (a *UpdateAgentImpl) Handle(ctx context.Context, intent *Intent, context *ConversationContext) (string, error) {
	action := intent.Action
	params := intent.Parameters

	switch action {
	case "create_event":
		return a.handleCreateEvent(ctx, params, context)
	case "update_event":
		return a.handleUpdateEvent(ctx, params, context)
	case "delete_event":
		return a.handleDeleteEvent(ctx, params, context)
	case "reschedule_event":
		return a.handleRescheduleEvent(ctx, params, context)
	case "apply_approval":
		return a.handleApplyApproval(ctx, params, context)
	default:
		return "", fmt.Errorf("unknown update action: %s", action)
	}
}

// handleCreateEvent creates a new calendar event
func (a *UpdateAgentImpl) handleCreateEvent(
	ctx context.Context,
	params map[string]interface{},
	context *ConversationContext,
) (string, error) {
	// Validate event data
	valid, reason, err := a.validator.ValidateChange(ctx, "create", nil, params)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}
	if !valid {
		return fmt.Sprintf("Cannot create event: %s", reason), nil
	}

	// Generate human-readable summary
	summary, err := a.validator.GenerateHumanSummary(ctx, &Intent{Action: "create_event"}, params)
	if err != nil {
		summary = "Create a new calendar event"
	}

	// Create pending approval
	approval := &PendingApproval{
		ID:                uuid.New().String(),
		ChangeType:        "create",
		ResourceType:      "event",
		OldState:          make(map[string]interface{}),
		NewState:          params,
		Summary:           summary,
		ExpiresAt:         time.Now().Add(5 * time.Minute),
		RequiredApprovals: 1,
		Approvals:         []string{},
	}

	// Store pending approval
	a.pendingApprovals[approval.ID] = approval
	context.PendingApprovals[approval.ID] = approval

	return fmt.Sprintf("🔒 Confirmation Required:\n%s\n\nApproval ID: %s\nExpires in 5 minutes",
		summary, approval.ID), nil
}

// handleUpdateEvent modifies an existing event
func (a *UpdateAgentImpl) handleUpdateEvent(
	ctx context.Context,
	params map[string]interface{},
	context *ConversationContext,
) (string, error) {
	eventID, ok := params["event_id"].(string)
	if !ok {
		return "", fmt.Errorf("missing event_id parameter")
	}

	updates := make(map[string]interface{})
	for k, v := range params {
		if k != "event_id" {
			updates[k] = v
		}
	}

	// Validate the update
	valid, reason, err := a.validator.ValidateChange(ctx, "update", nil, updates)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}
	if !valid {
		return fmt.Sprintf("Cannot update event: %s", reason), nil
	}

	// Generate summary
	summary, err := a.validator.GenerateHumanSummary(ctx,
		&Intent{Action: "update_event", Parameters: map[string]interface{}{"event_id": eventID}},
		updates)
	if err != nil {
		summary = fmt.Sprintf("Update event %s", eventID)
	}

	// Create pending approval
	approval := &PendingApproval{
		ID:                uuid.New().String(),
		ChangeType:        "update",
		ResourceType:      "event",
		ResourceID:        eventID,
		OldState:          make(map[string]interface{}), // In real impl, fetch current state
		NewState:          updates,
		Summary:           summary,
		ExpiresAt:         time.Now().Add(5 * time.Minute),
		RequiredApprovals: 1,
		Approvals:         []string{},
	}

	a.pendingApprovals[approval.ID] = approval
	context.PendingApprovals[approval.ID] = approval

	return fmt.Sprintf("🔒 Confirmation Required:\n%s\n\nApproval ID: %s\nExpires in 5 minutes",
		summary, approval.ID), nil
}

// handleDeleteEvent removes an event
func (a *UpdateAgentImpl) handleDeleteEvent(
	ctx context.Context,
	params map[string]interface{},
	context *ConversationContext,
) (string, error) {
	eventID, ok := params["event_id"].(string)
	if !ok {
		return "", fmt.Errorf("missing event_id parameter")
	}

	// Validate deletion
	valid, reason, err := a.validator.ValidateChange(ctx, "delete", nil, nil)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}
	if !valid {
		return fmt.Sprintf("Cannot delete event: %s", reason), nil
	}

	summary := fmt.Sprintf("Delete event with ID: %s\nThis action cannot be undone.", eventID)

	// Create pending approval
	approval := &PendingApproval{
		ID:                uuid.New().String(),
		ChangeType:        "delete",
		ResourceType:      "event",
		ResourceID:        eventID,
		OldState:          make(map[string]interface{}),
		NewState:          make(map[string]interface{}),
		Summary:           summary,
		ExpiresAt:         time.Now().Add(5 * time.Minute),
		RequiredApprovals: 1,
		Approvals:         []string{},
	}

	a.pendingApprovals[approval.ID] = approval
	context.PendingApprovals[approval.ID] = approval

	return fmt.Sprintf("⚠️ DESTRUCTIVE OPERATION:\n%s\n\nApproval ID: %s\nExpires in 5 minutes",
		summary, approval.ID), nil
}

// handleRescheduleEvent moves an event to a new time
func (a *UpdateAgentImpl) handleRescheduleEvent(
	ctx context.Context,
	params map[string]interface{},
	context *ConversationContext,
) (string, error) {
	eventID, ok := params["event_id"].(string)
	if !ok {
		return "", fmt.Errorf("missing event_id parameter")
	}

	newStartTime, ok := params["new_start_time"].(time.Time)
	if !ok {
		if s, ok := params["new_start_time"].(int64); ok {
			newStartTime = time.Unix(s, 0)
		} else {
			return "", fmt.Errorf("invalid new_start_time parameter")
		}
	}

	var newEndTime time.Time
	if e, ok := params["new_end_time"].(time.Time); ok {
		newEndTime = e
	} else if e, ok := params["new_end_time"].(int64); ok {
		newEndTime = time.Unix(e, 0)
	} else if d, ok := params["duration"].(time.Duration); ok {
		newEndTime = newStartTime.Add(d)
	} else {
		newEndTime = newStartTime.Add(1 * time.Hour)
	}

	// Check for conflicts
	conflicts, err := a.validator.CheckConflicts(ctx, map[string]interface{}{
		"id":         eventID,
		"start_time": newStartTime,
		"end_time":   newEndTime,
	})
	if err != nil {
		return "", fmt.Errorf("failed to check conflicts: %w", err)
	}

	if len(conflicts) > 0 {
		conflictInfo, _ := json.MarshalIndent(conflicts, "", "  ")
		return fmt.Sprintf("⚠️ Scheduling conflict detected:\n%s\n\nPlease choose a different time slot.",
			string(conflictInfo)), nil
	}

	// Generate summary
	summary, err := a.validator.GenerateHumanSummary(ctx,
		&Intent{Action: "reschedule_event"},
		map[string]interface{}{
			"event_id":       eventID,
			"new_start_time": newStartTime,
			"new_end_time":   newEndTime,
		})
	if err != nil {
		summary = fmt.Sprintf("Reschedule event to %s - %s",
			newStartTime.Format(time.RFC1123), newEndTime.Format(time.RFC1123))
	}

	// Create pending approval
	approval := &PendingApproval{
		ID:           uuid.New().String(),
		ChangeType:   "update",
		ResourceType: "event",
		ResourceID:   eventID,
		OldState:     make(map[string]interface{}),
		NewState: map[string]interface{}{
			"start_time": newStartTime,
			"end_time":   newEndTime,
		},
		Summary:           summary,
		ExpiresAt:         time.Now().Add(5 * time.Minute),
		RequiredApprovals: 1,
		Approvals:         []string{},
	}

	a.pendingApprovals[approval.ID] = approval
	context.PendingApprovals[approval.ID] = approval

	return fmt.Sprintf("🔒 Confirmation Required:\n%s\n\nApproval ID: %s\nExpires in 5 minutes",
		summary, approval.ID), nil
}

// handleApplyApproval executes a pre-approved change
func (a *UpdateAgentImpl) handleApplyApproval(
	ctx context.Context,
	params map[string]interface{},
	context *ConversationContext,
) (string, error) {
	approvalID, ok := params["approval_id"].(string)
	if !ok {
		return "", fmt.Errorf("missing approval_id parameter")
	}

	approval, exists := a.pendingApprovals[approvalID]
	if !exists {
		return "", fmt.Errorf("approval not found: %s", approvalID)
	}

	// Check if approval has expired
	if time.Now().After(approval.ExpiresAt) {
		delete(a.pendingApprovals, approvalID)
		return "⏰ Approval has expired. Please request again.", nil
	}

	// Mark as approved by user
	userID := context.UserID
	approval.Approvals = append(approval.Approvals, userID)

	// Check if all required approvals have been obtained
	if len(approval.Approvals) < approval.RequiredApprovals {
		return fmt.Sprintf("✓ Approval recorded (%d/%d required approvals obtained)",
			len(approval.Approvals), approval.RequiredApprovals), nil
	}

	// Execute the change
	err := a.tool.ApplyPendingApproval(ctx, approval)
	if err != nil {
		// Log the failure
		if a.auditTrail != nil {
			a.auditTrail.LogAction(ctx, AuditLog{
				ID:           uuid.New().String(),
				SessionID:    context.SessionID,
				UserID:       context.UserID,
				Action:       approval.ChangeType,
				ResourceType: approval.ResourceType,
				ResourceID:   approval.ResourceID,
				Status:       "failed",
				Error:        err.Error(),
				Timestamp:    time.Now(),
			})
		}
		return fmt.Sprintf("❌ Failed to execute change: %v", err), nil
	}

	// Log the success
	if a.auditTrail != nil {
		a.auditTrail.LogAction(ctx, AuditLog{
			ID:           uuid.New().String(),
			SessionID:    context.SessionID,
			UserID:       context.UserID,
			Action:       approval.ChangeType,
			ResourceType: approval.ResourceType,
			ResourceID:   approval.ResourceID,
			NewValue:     fmt.Sprintf("%v", approval.NewState),
			Status:       "executed",
			Timestamp:    time.Now(),
		})
	}

	// Clean up
	delete(a.pendingApprovals, approvalID)
	delete(context.PendingApprovals, approvalID)

	return fmt.Sprintf("✅ Change executed successfully!\n%s", approval.Summary), nil
}

// SetRequiresConfirmation sets whether confirmation is required for all operations
func (a *UpdateAgentImpl) SetRequiresConfirmation(required bool) {
	a.requiresConfirmation = required
}

// GetPendingApprovals returns all pending approvals
func (a *UpdateAgentImpl) GetPendingApprovals() map[string]*PendingApproval {
	return a.pendingApprovals
}

// RejectApproval rejects a pending approval
func (a *UpdateAgentImpl) RejectApproval(ctx context.Context, approvalID string, context *ConversationContext) error {
	approval, exists := a.pendingApprovals[approvalID]
	if !exists {
		return fmt.Errorf("approval not found: %s", approvalID)
	}

	// Log the rejection
	if a.auditTrail != nil {
		a.auditTrail.LogAction(ctx, AuditLog{
			ID:           uuid.New().String(),
			SessionID:    context.SessionID,
			UserID:       context.UserID,
			Action:       approval.ChangeType,
			ResourceType: approval.ResourceType,
			ResourceID:   approval.ResourceID,
			Status:       "rejected",
			Timestamp:    time.Now(),
		})
	}

	delete(a.pendingApprovals, approvalID)
	delete(context.PendingApprovals, approvalID)

	return nil
}
