package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== ReAct-Style Step Recovery Engine Implementation ==========

// DefaultStepRecoveryEngine implements the StepRecoveryEngine interface
type DefaultStepRecoveryEngine struct {
	agent *StateAwareAgent
}

// NewStepRecoveryEngine creates a new step recovery engine
func NewStepRecoveryEngine(agent *StateAwareAgent) StepRecoveryEngine {
	return &DefaultStepRecoveryEngine{
		agent: agent,
	}
}

// AttemptStepRecovery tries to recover from a failed step using AI consultation
func (r *DefaultStepRecoveryEngine) AttemptStepRecovery(ctx context.Context, failureContext *StepFailureContext) (*StepRecoveryResult, error) {
	r.agent.Logger.WithFields(map[string]interface{}{
		"step_id":        failureContext.OriginalStep.ID,
		"attempt_number": failureContext.AttemptNumber,
		"error":          failureContext.FailureError,
	}).Info("Attempting ReAct-style step recovery")

	// Step 1: Analyze the failure using AI
	analysis, err := r.AnalyzeFailure(ctx, failureContext)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze step failure: %w", err)
	}

	r.agent.Logger.WithFields(map[string]interface{}{
		"recommended_action": analysis.RecommendedAction,
		"confidence":         analysis.Confidence,
		"options_count":      len(analysis.RecoveryOptions),
	}).Info("AI failure analysis completed")

	// Step 2: Select the best recovery option
	selectedOption := r.selectBestRecoveryOption(analysis.RecoveryOptions)
	if selectedOption == nil {
		return &StepRecoveryResult{
			Success:        false,
			AttemptNumber:  failureContext.AttemptNumber,
			RecoveryAction: "fail_plan",
			Reasoning:      "No viable recovery options found",
		}, nil
	}

	// Step 3: Validate the recovery action
	recoveryResult := &StepRecoveryResult{
		Success:         false, // Will be set based on execution
		AlternativeTool: selectedOption.ToolName,
		ModifiedParams:  selectedOption.Parameters,
		Reasoning:       selectedOption.Reasoning,
		AttemptNumber:   failureContext.AttemptNumber,
		RecoveryAction:  selectedOption.Action,
	}

	if err := r.ValidateRecoveryAction(ctx, recoveryResult, failureContext); err != nil {
		r.agent.Logger.WithError(err).Warn("Recovery action validation failed")
		return &StepRecoveryResult{
			Success:        false,
			AttemptNumber:  failureContext.AttemptNumber,
			RecoveryAction: "fail_plan",
			Reasoning:      fmt.Sprintf("Recovery validation failed: %v", err),
		}, nil
	}

	// Step 4: Execute the recovery action
	return r.executeRecoveryAction(ctx, recoveryResult, failureContext)
}

// AnalyzeFailure analyzes the failure and suggests recovery options using AI
func (r *DefaultStepRecoveryEngine) AnalyzeFailure(ctx context.Context, failureContext *StepFailureContext) (*AIRecoveryAnalysis, error) {
	// Build comprehensive prompt for AI analysis
	prompt := r.buildRecoveryAnalysisPrompt(failureContext)

	r.agent.Logger.WithField("prompt_length", len(prompt)).Debug("Built recovery analysis prompt")

	// Call AI model for analysis using GenerateContent
	messages := []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{
				llms.TextContent{Text: "You are an expert infrastructure recovery agent. Analyze failures and provide structured recovery options in JSON format."},
			},
		},
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextContent{Text: prompt},
			},
		},
	}

	resp, err := r.agent.llm.GenerateContent(ctx, messages,
		llms.WithTemperature(0.1),
		llms.WithMaxTokens(r.agent.config.MaxTokens))
	if err != nil {
		return nil, fmt.Errorf("AI consultation failed: %w", err)
	}

	// Extract response content
	var content string
	if len(resp.Choices) > 0 && len(resp.Choices[0].Content) > 0 {
		content = resp.Choices[0].Content
	} else {
		return nil, fmt.Errorf("empty response from AI model")
	}

	// Log the raw AI response for debugging
	r.agent.Logger.WithFields(map[string]interface{}{
		"response_length": len(content),
		"raw_response":    content,
	}).Debug("Raw AI recovery analysis response")

	// Parse the AI response into structured analysis
	analysis, err := r.parseRecoveryAnalysis(content)
	if err != nil {
		r.agent.Logger.WithError(err).WithField("raw_response", content).Warn("Failed to parse AI recovery analysis, using fallback")
		// Fallback to basic analysis
		return r.createFallbackAnalysis(failureContext), nil
	}

	return analysis, nil
}

// GetSimilarTools finds alternative tools that might achieve the same goal
func (r *DefaultStepRecoveryEngine) GetSimilarTools(ctx context.Context, originalTool string, objective string) ([]MCPToolInfo, error) {
	r.agent.capabilityMutex.RLock()
	defer r.agent.capabilityMutex.RUnlock()

	var similarTools []MCPToolInfo

	// Extract resource type from original tool
	resourceType, _ := r.parseToolComponents(originalTool)

	// Find tools that work with the same resource type or achieve similar objectives
	for toolName, toolInfo := range r.agent.mcpTools {
		if toolName == originalTool {
			continue // Skip the original tool
		}

		// Check if tool works with same resource type
		if r.matchesResourceType(toolName, resourceType) {
			similarTools = append(similarTools, toolInfo)
			continue
		}

		// Check if tool description matches the objective
		if r.matchesObjective(toolInfo.Description, objective) {
			similarTools = append(similarTools, toolInfo)
		}
	}

	r.agent.Logger.WithFields(map[string]interface{}{
		"original_tool": originalTool,
		"objective":     objective,
		"similar_tools": len(similarTools),
	}).Debug("Found similar tools for recovery")

	return similarTools, nil
}

// ValidateRecoveryAction ensures the proposed recovery action is safe and feasible
func (r *DefaultStepRecoveryEngine) ValidateRecoveryAction(ctx context.Context, action *StepRecoveryResult, context *StepFailureContext) error {
	switch action.RecoveryAction {
	case "retry_same":
		// Validate that we haven't exceeded retry limits
		if context.AttemptNumber >= 3 {
			return fmt.Errorf("maximum retry attempts reached")
		}
		return nil

	case "try_alternative":
		// Validate that the alternative tool exists and is available
		if action.AlternativeTool == "" {
			return fmt.Errorf("no alternative tool specified")
		}

		r.agent.capabilityMutex.RLock()
		toolInfo, exists := r.agent.mcpTools[action.AlternativeTool]
		r.agent.capabilityMutex.RUnlock()

		if !exists {
			return fmt.Errorf("alternative tool %s not available", action.AlternativeTool)
		}

		// Validate parameters against tool schema
		if err := r.validateToolParameters(action.ModifiedParams, toolInfo.InputSchema); err != nil {
			return fmt.Errorf("invalid parameters for alternative tool: %w", err)
		}
		return nil

	case "modify_params":
		// Validate that parameter modifications are safe
		return r.validateParameterModifications(action.ModifiedParams, context)

	case "skip_step":
		// Validate that skipping this step won't break dependencies
		return r.validateStepSkip(context)

	case "fail_plan":
		// Always valid - represents controlled failure
		return nil

	default:
		return fmt.Errorf("unknown recovery action: %s", action.RecoveryAction)
	}
}

// ========== Private Helper Methods ==========

// buildRecoveryAnalysisPrompt creates a comprehensive prompt for AI analysis
func (r *DefaultStepRecoveryEngine) buildRecoveryAnalysisPrompt(failureContext *StepFailureContext) string {
	var prompt strings.Builder

	prompt.WriteString("# ReAct-Style Step Recovery Analysis\n\n")
	prompt.WriteString("You are an intelligent infrastructure agent tasked with recovering from a failed execution step. ")
	prompt.WriteString("Analyze the failure and provide structured recovery options.\n\n")

	// Step failure information
	prompt.WriteString("## Failed Step Information\n")
	prompt.WriteString(fmt.Sprintf("- **Step ID**: %s\n", failureContext.OriginalStep.ID))
	prompt.WriteString(fmt.Sprintf("- **Step Name**: %s\n", failureContext.OriginalStep.Name))
	prompt.WriteString(fmt.Sprintf("- **Action**: %s\n", failureContext.OriginalStep.Action))
	prompt.WriteString(fmt.Sprintf("- **Tool Used**: %s\n", failureContext.OriginalStep.MCPTool))
	prompt.WriteString(fmt.Sprintf("- **Error**: %s\n", failureContext.FailureError))
	prompt.WriteString(fmt.Sprintf("- **Attempt Number**: %d\n\n", failureContext.AttemptNumber))

	// Parameters used
	if len(failureContext.OriginalStep.Parameters) > 0 {
		prompt.WriteString("## Parameters Used\n")
		for key, value := range failureContext.OriginalStep.Parameters {
			prompt.WriteString(fmt.Sprintf("- **%s**: %v\n", key, value))
		}
		prompt.WriteString("\n")
	}

	// Previous attempts
	if len(failureContext.PreviousAttempts) > 0 {
		prompt.WriteString("## Previous Recovery Attempts\n")
		for _, attempt := range failureContext.PreviousAttempts {
			prompt.WriteString(fmt.Sprintf("- **Attempt %d**: Used %s, Result: %s\n",
				attempt.AttemptNumber, attempt.ToolUsed, attempt.Error))
		}
		prompt.WriteString("\n")
	}

	// Available tools
	prompt.WriteString("## Available Tools\n")
	for _, tool := range failureContext.AvailableTools {
		prompt.WriteString(fmt.Sprintf("- **%s**: %s\n", tool.Name, tool.Description))
	}
	prompt.WriteString("\n")

	// Current execution context
	prompt.WriteString("## Execution Context\n")
	prompt.WriteString(fmt.Sprintf("- **Completed Steps**: %d\n", len(failureContext.CompletedSteps)))
	prompt.WriteString(fmt.Sprintf("- **Remaining Steps**: %d\n", len(failureContext.RemainingSteps)))
	prompt.WriteString(fmt.Sprintf("- **AWS Region**: %s\n\n", failureContext.AWSRegion))

	// Request structured response
	prompt.WriteString("## Required Response Format\n")
	prompt.WriteString("Provide your analysis in the following JSON format. ")
	prompt.WriteString("**IMPORTANT**: Use only valid JSON syntax without comments or explanatory text within the JSON structure.\n\n")
	prompt.WriteString("```json\n")
	prompt.WriteString(`{
  "failure_reason": "Root cause analysis of why the step failed",
  "recovery_options": [
    {
      "action": "retry_same|try_alternative|modify_params|skip_step",
      "tool_name": "alternative-tool-name-if-applicable",
      "parameters": {"param1": "value1"},
      "reasoning": "Why this approach should work",
      "success_probability": 0.8,
      "risk_level": "low|medium|high",
      "dependencies": ["step-ids-that-must-complete-first"]
    }
  ],
  "recommended_action": "retry_same|try_alternative|modify_params|skip_step|fail_plan",
  "confidence": 0.85,
  "requires_user_input": false,
  "risk_assessment": "Overall risk assessment",
  "alternative_approach": "If complete replanning is needed"
}`)
	prompt.WriteString("\n```\n\n")

	prompt.WriteString("Focus on practical, safe recovery options. Consider AWS service limitations, ")
	prompt.WriteString("resource dependencies, and infrastructure consistency. Prioritize solutions with ")
	prompt.WriteString("high success probability and low risk.\n\n")
	prompt.WriteString("**CRITICAL**: Return ONLY valid JSON without any comments (//), explanations, or additional text. ")
	prompt.WriteString("Do not include inline comments like '// comment here' within the JSON structure.")

	return prompt.String()
}

// parseRecoveryAnalysis parses the AI response into structured analysis
func (r *DefaultStepRecoveryEngine) parseRecoveryAnalysis(response string) (*AIRecoveryAnalysis, error) {
	// Use existing agent JSON processing methods instead of custom implementation
	var jsonStr string

	// Try primary extraction method
	jsonStr = r.agent.extractJSON(response)

	// If primary method fails, try alternative methods
	if jsonStr == "" {
		jsonStr = r.agent.extractJSONAlternative(response)
	}

	// If still no JSON found, try truncated JSON parsing
	if jsonStr == "" {
		jsonStr = r.agent.attemptTruncatedJSONParse(response)
	}

	if jsonStr == "" {
		return nil, fmt.Errorf("no valid JSON found in AI response")
	}

	// Clean up common AI JSON issues (like inline comments)
	jsonStr = r.cleanJSONComments(jsonStr)

	if r.agent.config.EnableDebug {
		r.agent.Logger.WithFields(map[string]interface{}{
			"extracted_json": jsonStr,
			"json_length":    len(jsonStr),
		}).Debug("Extracted JSON from AI response using agent JSON processing")
	}

	var analysis AIRecoveryAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Validate the parsed analysis
	if len(analysis.RecoveryOptions) == 0 {
		return nil, fmt.Errorf("no recovery options provided by AI")
	}

	return &analysis, nil
}

// cleanJSONComments removes JavaScript-style comments from JSON that AI models sometimes include
func (r *DefaultStepRecoveryEngine) cleanJSONComments(jsonStr string) string {
	lines := strings.Split(jsonStr, "\n")
	var cleanedLines []string

	for _, line := range lines {
		// Find the position of // comment (but not inside strings)
		inString := false
		escaped := false
		commentPos := -1

		for i, char := range line {
			if escaped {
				escaped = false
				continue
			}

			if char == '\\' {
				escaped = true
				continue
			}

			if char == '"' {
				inString = !inString
				continue
			}

			// If we're not inside a string and find //, mark it as comment start
			if !inString && char == '/' && i+1 < len(line) && line[i+1] == '/' {
				commentPos = i
				break
			}
		}

		// Remove the comment part if found
		if commentPos != -1 {
			line = strings.TrimSpace(line[:commentPos])
		}

		// Skip empty lines that resulted from comment removal
		if strings.TrimSpace(line) != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, "\n")
}

// createFallbackAnalysis creates a basic analysis when AI parsing fails
func (r *DefaultStepRecoveryEngine) createFallbackAnalysis(failureContext *StepFailureContext) *AIRecoveryAnalysis {
	options := []*RecoveryOption{}

	// Basic retry option
	if failureContext.AttemptNumber < 3 {
		options = append(options, &RecoveryOption{
			Action:             "retry_same",
			Reasoning:          "Retry the same operation - may be a transient failure",
			SuccessProbability: 0.5,
			RiskLevel:          "low",
		})
	}

	// Look for similar tools
	similarTools, _ := r.GetSimilarTools(context.Background(), failureContext.OriginalStep.MCPTool, failureContext.OriginalStep.Description)
	if len(similarTools) > 0 {
		options = append(options, &RecoveryOption{
			Action:             "try_alternative",
			ToolName:           similarTools[0].Name,
			Parameters:         failureContext.OriginalStep.Parameters,
			Reasoning:          fmt.Sprintf("Try alternative tool: %s", similarTools[0].Name),
			SuccessProbability: 0.6,
			RiskLevel:          "medium",
		})
	}

	return &AIRecoveryAnalysis{
		FailureReason:     "Step execution failed",
		RecoveryOptions:   options,
		RecommendedAction: "retry_same",
		Confidence:        0.5,
		RequiresUserInput: false,
		RiskAssessment:    "Low risk fallback analysis",
	}
}

// selectBestRecoveryOption selects the best recovery option from AI suggestions
func (r *DefaultStepRecoveryEngine) selectBestRecoveryOption(options []*RecoveryOption) *RecoveryOption {
	if len(options) == 0 {
		return nil
	}

	// Score each option based on success probability, risk level, and action type
	bestOption := options[0]
	bestScore := r.calculateOptionScore(bestOption)

	for _, option := range options[1:] {
		score := r.calculateOptionScore(option)
		if score > bestScore {
			bestScore = score
			bestOption = option
		}
	}

	return bestOption
}

// calculateOptionScore scores a recovery option for selection
func (r *DefaultStepRecoveryEngine) calculateOptionScore(option *RecoveryOption) float64 {
	score := option.SuccessProbability

	// Adjust for risk level
	switch option.RiskLevel {
	case "low":
		score += 0.1
	case "medium":
		score += 0.0
	case "high":
		score -= 0.2
	}

	// Prefer certain action types
	switch option.Action {
	case "retry_same":
		score += 0.05
	case "try_alternative":
		score += 0.1
	case "modify_params":
		score += 0.0
	case "skip_step":
		score -= 0.1
	}

	return score
}

// executeRecoveryAction executes the selected recovery action
func (r *DefaultStepRecoveryEngine) executeRecoveryAction(ctx context.Context, action *StepRecoveryResult, failureContext *StepFailureContext) (*StepRecoveryResult, error) {
	startTime := time.Now()

	switch action.RecoveryAction {
	case "retry_same":
		// Retry the original step with same parameters
		result, err := r.executeStepAction(ctx, failureContext.OriginalStep)
		action.Success = err == nil
		if err != nil {
			action.Reasoning = fmt.Sprintf("Retry failed: %v", err)
		} else {
			action.Output = result
		}

	case "try_alternative":
		// Create modified step with alternative tool
		modifiedStep := *failureContext.OriginalStep
		modifiedStep.MCPTool = action.AlternativeTool
		if action.ModifiedParams != nil {
			modifiedStep.Parameters = action.ModifiedParams
		}

		result, err := r.executeStepAction(ctx, &modifiedStep)
		action.Success = err == nil
		if err != nil {
			action.Reasoning = fmt.Sprintf("Alternative tool failed: %v", err)
		} else {
			action.Output = result
		}

	case "modify_params":
		// Retry with modified parameters
		modifiedStep := *failureContext.OriginalStep
		modifiedStep.Parameters = action.ModifiedParams

		result, err := r.executeStepAction(ctx, &modifiedStep)
		action.Success = err == nil
		if err != nil {
			action.Reasoning = fmt.Sprintf("Modified parameters failed: %v", err)
		} else {
			action.Output = result
		}

	case "skip_step":
		// Mark as successful skip
		action.Success = true
		action.Output = map[string]interface{}{"skipped": true}

	case "fail_plan":
		// Controlled failure
		action.Success = false
		action.Reasoning = "Recovery determined plan should fail"

	default:
		return nil, fmt.Errorf("unknown recovery action: %s", action.RecoveryAction)
	}

	duration := time.Since(startTime)
	r.agent.Logger.WithFields(map[string]interface{}{
		"recovery_action": action.RecoveryAction,
		"success":         action.Success,
		"duration":        duration,
	}).Info("Recovery action completed")

	return action, nil
}

// executeStepAction executes a step action (helper method)
func (r *DefaultStepRecoveryEngine) executeStepAction(ctx context.Context, step *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Resolve any template parameters first
	resolvedParams := make(map[string]interface{})
	for key, value := range step.Parameters {
		if strValue, ok := value.(string); ok && strings.Contains(strValue, "{{") && strings.Contains(strValue, "}}") {
			if resolvedValue, err := r.agent.resolveDependencyReference(strValue); err == nil {
				resolvedParams[key] = resolvedValue
			} else {
				resolvedParams[key] = value // Fallback to original
			}
		} else {
			resolvedParams[key] = value
		}
	}

	// Execute the MCP tool call
	result, err := r.agent.callMCPTool(step.MCPTool, resolvedParams)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ========== Validation Helper Methods ==========

func (r *DefaultStepRecoveryEngine) parseToolComponents(toolName string) (resourceType, action string) {
	// Parse tool name like "create-vpc" -> resource="vpc", action="create"
	parts := strings.Split(toolName, "-")
	if len(parts) >= 2 {
		action = parts[0]
		resourceType = strings.Join(parts[1:], "-")
	}
	return
}

func (r *DefaultStepRecoveryEngine) matchesResourceType(toolName, resourceType string) bool {
	return strings.Contains(toolName, resourceType)
}

func (r *DefaultStepRecoveryEngine) matchesObjective(description, objective string) bool {
	// Simple keyword matching - could be enhanced with semantic similarity
	descLower := strings.ToLower(description)
	objLower := strings.ToLower(objective)

	words := strings.Fields(objLower)
	matches := 0

	for _, word := range words {
		if strings.Contains(descLower, word) {
			matches++
		}
	}

	// Consider a match if at least 30% of words match
	return float64(matches)/float64(len(words)) >= 0.3
}

func (r *DefaultStepRecoveryEngine) validateToolParameters(params map[string]interface{}, schema map[string]interface{}) error {
	// Basic parameter validation - could be enhanced with JSON schema validation
	// For now, just check required parameters exist

	_, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return nil // No validation rules
	}

	required, ok := schema["required"].([]interface{})
	if !ok {
		return nil // No required parameters
	}

	for _, reqParam := range required {
		paramName, ok := reqParam.(string)
		if !ok {
			continue
		}

		if _, exists := params[paramName]; !exists {
			return fmt.Errorf("required parameter %s is missing", paramName)
		}
	}

	return nil
}

func (r *DefaultStepRecoveryEngine) validateParameterModifications(params map[string]interface{}, context *StepFailureContext) error {
	// Validate that parameter modifications are safe
	// Check for dangerous modifications that could affect other resources

	originalParams := context.OriginalStep.Parameters

	// Check for critical parameter changes
	criticalParams := []string{"vpcId", "subnetId", "securityGroupId", "keyName"}

	for _, param := range criticalParams {
		if originalVal, origExists := originalParams[param]; origExists {
			if newVal, newExists := params[param]; newExists && originalVal != newVal {
				r.agent.Logger.WithFields(map[string]interface{}{
					"parameter":      param,
					"original_value": originalVal,
					"new_value":      newVal,
				}).Warn("Critical parameter modification detected")
			}
		}
	}

	return nil
}

func (r *DefaultStepRecoveryEngine) validateStepSkip(context *StepFailureContext) error {
	// Check if any remaining steps depend on this step's output
	skipStepID := context.OriginalStep.ID

	for _, remainingStep := range context.RemainingSteps {
		for _, param := range remainingStep.Parameters {
			if paramStr, ok := param.(string); ok {
				// Check for dependency references like {{step-create-vpc.resourceId}}
				if strings.Contains(paramStr, fmt.Sprintf("{{%s.", skipStepID)) {
					return fmt.Errorf("cannot skip step %s: step %s depends on its output", skipStepID, remainingStep.ID)
				}
			}
		}
	}

	r.agent.Logger.WithField("step_id", skipStepID).Info("Step can be safely skipped - no dependencies found")
	return nil
}
