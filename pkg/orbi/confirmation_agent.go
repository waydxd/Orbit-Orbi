package orbi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ConfirmationAgentImpl is the concrete implementation of the Confirmation Agent
type ConfirmationAgentImpl struct {
	name      string
	llm       LLMInterface
	validator ConfirmationValidator
}

// NewConfirmationAgent creates a new Confirmation Agent instance
func NewConfirmationAgent(llm LLMInterface, validator ConfirmationValidator) *ConfirmationAgentImpl {
	return &ConfirmationAgentImpl{
		name:      "confirmation-agent",
		llm:       llm,
		validator: validator,
	}
}

// Name returns the agent's identifier
func (a *ConfirmationAgentImpl) Name() string {
	return a.name
}

// ShouldHandle returns true if this agent should handle the given intent
func (a *ConfirmationAgentImpl) ShouldHandle(intent *Intent) bool {
	return intent.RequiresApproval
}

// Handle processes a confirmation request
func (a *ConfirmationAgentImpl) Handle(ctx context.Context, intent *Intent, context *ConversationContext) (string, error) {
	// Generate a confirmation prompt with human-readable summary
	approval, ok := context.PendingApprovals[intent.Parameters["approval_id"].(string)]
	if !ok {
		return "", fmt.Errorf("approval not found")
	}

	// Validate the proposed change
	valid, reason, err := a.validator.ValidateChange(ctx, approval.ChangeType, approval.OldState, approval.NewState)
	if err != nil {
		return fmt.Sprintf("❌ Validation error: %v", err), nil
	}

	if !valid {
		return fmt.Sprintf("❌ Change validation failed: %s", reason), nil
	}

	// Check for conflicts
	conflicts, err := a.validator.CheckConflicts(ctx, approval.NewState)
	if err != nil {
		return fmt.Sprintf("⚠️ Conflict check error: %v", err), nil
	}

	if len(conflicts) > 0 {
		conflictData, _ := json.MarshalIndent(conflicts, "", "  ")
		return fmt.Sprintf("⚠️ Potential conflicts detected:\n%s\n\nProceed with caution.", string(conflictData)), nil
	}

	// Generate a detailed confirmation prompt
	prompt, err := a.generateConfirmationPrompt(ctx, approval)
	if err != nil {
		// Fallback to basic prompt
		return fmt.Sprintf("Confirm this change:\n%s\n\nReply 'yes' to confirm or 'no' to cancel.", approval.Summary), nil
	}

	return prompt, nil
}

// ValidateChange checks if a proposed change is valid
func (a *ConfirmationAgentImpl) ValidateChange(
	ctx context.Context,
	changeType string,
	oldState map[string]interface{},
	newState map[string]interface{},
) (bool, string, error) {
	// Basic validation rules
	switch changeType {
	case "create":
		// Check required fields
		if newState == nil || len(newState) == 0 {
			return false, "event data is missing", nil
		}
		if _, ok := newState["title"]; !ok {
			return false, "event title is required", nil
		}
		if _, ok := newState["start_time"]; !ok {
			return false, "event start time is required", nil
		}
		if _, ok := newState["end_time"]; !ok {
			return false, "event end time is required", nil
		}

	case "update":
		if newState == nil || len(newState) == 0 {
			return false, "no changes specified", nil
		}

	case "delete":
		// Deletion is always allowed (but should be confirmed)
		break

	default:
		return false, fmt.Sprintf("unknown change type: %s", changeType), nil
	}

	return true, "", nil
}

// GenerateHumanSummary creates a user-friendly description of a change
func (a *ConfirmationAgentImpl) GenerateHumanSummary(
	ctx context.Context,
	intent *Intent,
	changes map[string]interface{},
) (string, error) {
	if a.llm == nil {
		return a.generateFallbackSummary(intent.Action, changes), nil
	}

	// Prepare data for LLM
	changesData, _ := json.MarshalIndent(changes, "", "  ")
	prompt := fmt.Sprintf(`You are a helpful assistant for a smart calendar system. 
A user is about to make a change to their calendar. 
Generate a brief, natural language summary of what will happen.
Be concise but clear. Respond in a way that helps the user understand the change.

Action: %s
Changes:
%s

Generate a 1-2 sentence confirmation message that starts with "我將會" (in Chinese) or "I will" (in English).
Make it friendly and clear.`,
		intent.Action, string(changesData))

	response, err := a.llm.Generate(ctx, prompt)
	if err != nil {
		return a.generateFallbackSummary(intent.Action, changes), nil
	}

	return response, nil
}

// CheckConflicts detects scheduling conflicts
func (a *ConfirmationAgentImpl) CheckConflicts(
	ctx context.Context,
	eventData map[string]interface{},
) ([]map[string]interface{}, error) {
	// This would typically call a retrieval tool to check for conflicts
	// For now, return empty slice (no conflicts)
	return []map[string]interface{}{}, nil
}

// generateConfirmationPrompt creates a detailed confirmation message
func (a *ConfirmationAgentImpl) generateConfirmationPrompt(
	ctx context.Context,
	approval *PendingApproval,
) (string, error) {
	if a.llm == nil {
		return fmt.Sprintf("🔒 Please confirm:\n%s\n\nReply with 'yes' to confirm or 'no' to cancel.", approval.Summary), nil
	}

	// Prepare data for LLM
	newData, _ := json.MarshalIndent(approval.NewState, "", "  ")
	prompt := fmt.Sprintf(`You are a helpful assistant for a smart calendar system.
Generate a friendly confirmation prompt for the user in their language.

Change Type: %s
Resource Type: %s
Summary: %s
New State:
%s

Create a 2-3 sentence confirmation prompt that:
1. Clearly explains what will happen
2. Highlights important details
3. Asks for confirmation
Start with a relevant emoji based on the change type.
If the change is about moving/updating a meeting, respond in Chinese.`,
		approval.ChangeType, approval.ResourceType, approval.Summary, string(newData))

	response, err := a.llm.Generate(ctx, prompt)
	if err != nil {
		return fmt.Sprintf("🔒 Please confirm:\n%s\n\nReply with 'yes' to confirm or 'no' to cancel.", approval.Summary), nil
	}

	return response, nil
}

// generateFallbackSummary creates a basic text summary when LLM is not available
func (a *ConfirmationAgentImpl) generateFallbackSummary(action string, changes map[string]interface{}) string {
	switch action {
	case "create_event":
		if title, ok := changes["title"]; ok {
			if start, ok := changes["start_time"]; ok {
				return fmt.Sprintf("Create a new event '%v' starting at %v", title, start)
			}
		}
		return "Create a new calendar event"

	case "update_event":
		summary := "Update event"
		if eventID, ok := changes["event_id"]; ok {
			summary = fmt.Sprintf("Update event %v", eventID)
		}
		if title, ok := changes["title"]; ok {
			summary += fmt.Sprintf(" (new title: '%v')", title)
		}
		return summary

	case "delete_event":
		if eventID, ok := changes["event_id"]; ok {
			return fmt.Sprintf("Delete event %v (this cannot be undone)", eventID)
		}
		return "Delete event (this cannot be undone)"

	case "reschedule_event":
		if start, ok := changes["new_start_time"]; ok {
			if end, ok := changes["new_end_time"]; ok {
				return fmt.Sprintf("Move event to %v - %v", start, end)
			}
		}
		return "Reschedule event"

	default:
		data, _ := json.MarshalIndent(changes, "", "  ")
		return string(data)
	}
}

// IsDestructive checks if a change is destructive (delete operation)
func (a *ConfirmationAgentImpl) IsDestructive(changeType string) bool {
	return changeType == "delete"
}

// GetRiskLevel returns the risk level of a change
func (a *ConfirmationAgentImpl) GetRiskLevel(approval *PendingApproval) string {
	switch approval.ChangeType {
	case "delete":
		return "HIGH"
	case "update":
		// Check if it's a significant time change
		if oldStart, ok := approval.OldState["start_time"]; ok {
			if newStart, ok := approval.NewState["start_time"]; ok {
				if oldStart != newStart {
					return "MEDIUM"
				}
			}
		}
		return "LOW"
	case "create":
		return "LOW"
	default:
		return "MEDIUM"
	}
}

// RequestApproval generates a message requesting user confirmation
func (a *ConfirmationAgentImpl) RequestApproval(
	ctx context.Context,
	approval *PendingApproval,
) (string, error) {
	riskLevel := a.GetRiskLevel(approval)
	isDestructive := a.IsDestructive(approval.ChangeType)

	// Build the confirmation message
	var message string

	if isDestructive {
		message = "⚠️ DESTRUCTIVE OPERATION\n"
	} else if riskLevel == "MEDIUM" {
		message = "🔒 CONFIRMATION REQUIRED\n"
	} else {
		message = "✓ PLEASE CONFIRM\n"
	}

	message += fmt.Sprintf("%s\n\n", approval.Summary)
	message += fmt.Sprintf("Expires in: %v\n", time.Until(approval.ExpiresAt).String())
	message += fmt.Sprintf("Approval ID: %s\n\n", approval.ID)
	message += "Reply 'yes' to confirm or 'no' to cancel."

	return message, nil
}
