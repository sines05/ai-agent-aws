package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== StateAwareAgent Extensions for ReAct Recovery ==========

// ExecuteStepWithRecovery executes a step with automatic ReAct-style recovery on failure
func (a *StateAwareAgent) ExecuteStepWithRecovery(
	ctx context.Context,
	planStep *types.ExecutionPlanStep,
	execution *types.PlanExecution,
	progressChan chan<- *types.ExecutionUpdate,
	strategy *RecoveryStrategy,
) (*types.ExecutionStep, error) {
	return a.ExecuteStepWithRecoveryAndCoordinator(ctx, planStep, execution, progressChan, strategy, nil)
}

// ExecuteStepWithRecoveryAndCoordinator executes a step with ReAct-style recovery and UI coordination
func (a *StateAwareAgent) ExecuteStepWithRecoveryAndCoordinator(
	ctx context.Context,
	planStep *types.ExecutionPlanStep,
	execution *types.PlanExecution,
	progressChan chan<- *types.ExecutionUpdate,
	strategy *RecoveryStrategy,
	coordinator RecoveryCoordinator,
) (*types.ExecutionStep, error) {

	// Initialize recovery components
	recoveryEngine := NewStepRecoveryEngine(a)
	contextBuilder := NewFailureContextBuilder(a)

	// Set default strategy if none provided
	if strategy == nil {
		strategy = &RecoveryStrategy{
			MaxAttempts:          3,
			EnableAIConsultation: true,
			AllowToolSwapping:    true,
			AllowParameterMod:    true,
			TimeoutPerAttempt:    5 * time.Minute,
		}
	}

	// First attempt - try normal execution
	a.Logger.WithFields(map[string]interface{}{
		"step_id": planStep.ID,
		"attempt": 1,
	}).Info("Attempting step execution")

	// Send progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "step_started",
			ExecutionID: execution.ID,
			StepID:      planStep.ID,
			Message:     fmt.Sprintf("Starting step: %s", planStep.Name),
			Timestamp:   time.Now(),
		}
	}

	startTime := time.Now()
	step, err := a.executeExecutionStep(ctx, planStep, execution, progressChan)

	if err == nil {
		// Success on first attempt
		return step, nil
	}

	// Step failed - if coordinator is available, request user input for recovery
	if coordinator != nil && strategy.EnableAIConsultation {
		a.Logger.WithField("step_id", planStep.ID).Info("Step failed, requesting recovery decision from user")

		// Send step failed message
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_failed",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Step failed: %v", err),
				Error:       err.Error(),
				Timestamp:   time.Now(),
			}
		}

		// Build failure context for AI consultation
		executionPlan := []*types.ExecutionPlanStep{planStep} // Simplified for now

		failureContext, contextErr := contextBuilder.BuildFailureContext(
			ctx,
			planStep,
			err,
			execution,
			executionPlan,
			1,
			[]*StepRecoveryAttempt{},
		)

		if contextErr != nil {
			a.Logger.WithError(contextErr).Warn("Failed to build failure context")
			return nil, err // Return original error
		}

		// Get AI recovery analysis
		aiAnalysis, aiErr := recoveryEngine.AnalyzeFailure(ctx, failureContext)
		if aiErr != nil {
			a.Logger.WithError(aiErr).Warn("Failed to get AI recovery analysis")
			return nil, err // Return original error
		}

		// Convert to UI format
		uiFailureContext := map[string]interface{}{
			"stepName":     planStep.Name,
			"toolName":     planStep.MCPTool,
			"action":       planStep.Action,
			"errorMessage": err.Error(),
			"aiAnalysis": map[string]interface{}{
				"rootCause":             aiAnalysis.FailureReason,
				"recommendation":        aiAnalysis.RecommendedAction,
				"alternativeApproaches": []string{aiAnalysis.AlternativeApproach},
			},
		}

		// Convert AI recovery options to UI format
		var uiRecoveryOptions []map[string]interface{}
		for _, option := range aiAnalysis.RecoveryOptions {
			uiRecoveryOptions = append(uiRecoveryOptions, map[string]interface{}{
				"action":             option.Action,
				"successProbability": option.SuccessProbability,
				"riskLevel":          option.RiskLevel,
				"reasoning":          option.Reasoning,
				"newTool":            option.ToolName,
				"modifiedParameters": option.Parameters,
			})
		}

		// Add default options if none provided
		if len(uiRecoveryOptions) == 0 {
			uiRecoveryOptions = []map[string]interface{}{
				{
					"action":             "Retry step",
					"successProbability": 0.6,
					"riskLevel":          "Low",
					"reasoning":          "Retry the same operation in case it was a temporary failure",
					"newTool":            nil,
					"modifiedParameters": nil,
				},
			}
		}

		// Request recovery decision from user
		userDecision, reqErr := coordinator.RequestRecoveryDecision(planStep.ID, uiFailureContext, uiRecoveryOptions)
		if reqErr != nil {
			a.Logger.WithError(reqErr).Error("Failed to get recovery decision from user")
			return nil, fmt.Errorf("recovery coordination failed: %w", reqErr)
		}

		// Check if user aborted
		if abort, exists := userDecision["abort"]; exists && abort.(bool) {
			return nil, fmt.Errorf("execution aborted by user during recovery")
		}

		// Apply user's recovery choice
		selectedIndex, exists := userDecision["selectedOptionIndex"]
		if !exists {
			return nil, fmt.Errorf("no recovery option selected")
		}

		indexStr := selectedIndex.(string)
		if indexStr == "skip" {
			a.Logger.WithField("step_id", planStep.ID).Info("User chose to skip step")

			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "step_skipped",
					ExecutionID: execution.ID,
					StepID:      planStep.ID,
					Message:     "Step skipped by user",
					Timestamp:   time.Now(),
				}
			}

			// Return a mock successful step for skipped steps
			now := time.Now()
			return &types.ExecutionStep{
				ID:          planStep.ID,
				Name:        planStep.Name,
				Status:      "skipped",
				Resource:    planStep.ResourceID,
				Action:      planStep.Action,
				StartedAt:   &startTime,
				CompletedAt: &now,
				Output:      map[string]interface{}{"status": "skipped", "reason": "user_request"},
			}, nil
		}

		// Apply the selected recovery option
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_recovery_started",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     "Applying recovery strategy...",
				Timestamp:   time.Now(),
			}
		}

		// Modify planStep based on user selection (simplified implementation)
		// In a full implementation, you would parse the selected option and apply appropriate changes

		// Retry with recovery modifications
		retryStep, retryErr := a.executeExecutionStep(ctx, planStep, execution, progressChan)
		if retryErr == nil {
			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "step_recovery_completed",
					ExecutionID: execution.ID,
					StepID:      planStep.ID,
					Message:     "Recovery successful",
					Timestamp:   time.Now(),
				}
			}
			return retryStep, nil
		} else {
			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "step_recovery_failed",
					ExecutionID: execution.ID,
					StepID:      planStep.ID,
					Message:     fmt.Sprintf("Recovery failed: %v", retryErr),
					Error:       retryErr.Error(),
					Timestamp:   time.Now(),
				}
			}
			return nil, retryErr
		}
	}

	// Fallback to original behavior if no coordinator
	return a.executeExecutionStep(ctx, planStep, execution, progressChan)
}

// ConsultAIForRecovery asks the AI model for recovery advice
func (a *StateAwareAgent) ConsultAIForRecovery(ctx context.Context, failureContext *StepFailureContext) (*AIRecoveryAnalysis, error) {
	recoveryEngine := NewStepRecoveryEngine(a)
	return recoveryEngine.AnalyzeFailure(ctx, failureContext)
}

// ApplyRecoveryAction executes the chosen recovery action
func (a *StateAwareAgent) ApplyRecoveryAction(ctx context.Context, action *StepRecoveryResult, failureContext *StepFailureContext) (*types.ExecutionStep, error) {
	recoveryEngine := NewStepRecoveryEngine(a)

	// Execute the recovery action and get the result
	result, err := recoveryEngine.(*DefaultStepRecoveryEngine).executeRecoveryAction(ctx, action, failureContext)
	if err != nil {
		return nil, fmt.Errorf("recovery action execution failed: %w", err)
	}

	// Convert the recovery result to an execution step
	if result.Success {
		now := time.Now()
		step := &types.ExecutionStep{
			ID:          failureContext.OriginalStep.ID,
			Name:        failureContext.OriginalStep.Name,
			Status:      "completed",
			Resource:    failureContext.OriginalStep.ResourceID,
			Action:      failureContext.OriginalStep.Action,
			StartedAt:   &now,
			CompletedAt: &now,
			Duration:    0, // Recovery timing would need to be tracked separately
			Output:      result.Output,
		}
		return step, nil
	} else {
		return nil, fmt.Errorf("recovery action failed: %s", result.Reasoning)
	}
}

// GetAvailableToolsForRecovery gets all available tools that could be used for recovery
func (a *StateAwareAgent) GetAvailableToolsForRecovery() []MCPToolInfo {
	a.capabilityMutex.RLock()
	defer a.capabilityMutex.RUnlock()

	var tools []MCPToolInfo
	for _, tool := range a.mcpTools {
		tools = append(tools, tool)
	}

	return tools
}

// UpdateResourceMapping updates the resource mapping after a successful recovery
func (a *StateAwareAgent) UpdateResourceMapping(stepID string, resourceID string) {
	a.mappingsMutex.Lock()
	defer a.mappingsMutex.Unlock()

	a.resourceMappings[stepID] = resourceID

	a.Logger.WithFields(map[string]interface{}{
		"step_id":     stepID,
		"resource_id": resourceID,
	}).Debug("Updated resource mapping after recovery")
}
