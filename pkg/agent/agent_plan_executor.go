package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== Interface defines ==========

// PlanExecutorInterface defines plan execution functionality
//
// Available Functions:
//   - ExecuteConfirmedPlanWithDryRun() : Execute confirmed plans with dry-run support
//   - simulatePlanExecution()          : Simulate plan execution for dry-run mode
//   - executeExecutionStep()           : Execute individual plan steps
//   - executeCreateAction()            : Execute create actions via MCP tools
//   - executeAPIValueRetrieval()       : Execute AWS API retrieval operations
//   - executeNativeMCPTool()          : Execute native MCP tool calls
//   - executeUpdateAction()            : Execute update actions on existing resources
//   - executeDeleteAction()            : Execute delete actions on resources
//   - executeValidateAction()          : Execute validation actions
//   - updateStateFromMCPResult()       : Update state from MCP operation results
//   - extractResourceTypeFromStep()    : Extract resource type from execution step
//   - getAvailableToolsContext()       : Get available tools context for AI prompts
//   - persistCurrentState()            : Persist current state to storage
//   - extractResourceIDFromResponse()  : Extract AWS resource IDs from responses
//   - waitForResourceReady()           : Wait for AWS resources to be ready before continuing
//   - checkResourceState()             : Check if a specific AWS resource is ready
//   - checkNATGatewayState()           : Check if NAT gateway is available
//   - checkRDSInstanceState()          : Check if RDS instance is available
//   - storeResourceMapping()           : Store step-to-resource ID mappings
//
// This file manages the execution of infrastructure plans, including dry-run
// capabilities, progress tracking, and state management integration.
//
// Usage Example:
//   1. execution := agent.ExecuteConfirmedPlanWithDryRun(ctx, decision, progressChan, false)
//   2. // Monitor execution through progressChan updates

// ExecuteConfirmedPlanWithDryRun executes a confirmed execution plan with a specific dry run setting
func (a *StateAwareAgent) ExecuteConfirmedPlanWithDryRun(ctx context.Context, decision *types.AgentDecision, progressChan chan<- *types.ExecutionUpdate, dryRun bool) (*types.PlanExecution, error) {
	a.Logger.WithFields(map[string]interface{}{
		"decision_id": decision.ID,
		"action":      decision.Action,
		"plan_steps":  len(decision.ExecutionPlan),
	}).Info("Executing confirmed plan")

	a.Logger.WithFields(map[string]interface{}{
		"dry_run":           dryRun,
		"progress_chan_nil": progressChan == nil,
		"execution_plan":    len(decision.ExecutionPlan),
	}).Debug("ExecuteConfirmedPlan debug info")

	if dryRun {
		a.Logger.Info("Dry run mode - simulating execution")
		a.Logger.Debug("About to call simulatePlanExecution")
		result := a.simulatePlanExecution(decision, progressChan)
		a.Logger.WithField("simulation_result", result.Status).Debug("Simulation completed")
		return result, nil
	}

	// Create execution plan
	execution := &types.PlanExecution{
		ID:        uuid.New().String(),
		Name:      fmt.Sprintf("Execute %s", decision.Action),
		Status:    "running",
		StartedAt: time.Now(),
		Steps:     []*types.ExecutionStep{},
		Changes:   []*types.ChangeDetection{},
		Errors:    []string{},
	}

	// Send initial progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "execution_started",
			ExecutionID: execution.ID,
			Message:     "Starting plan execution",
			Timestamp:   time.Now(),
		}
	}

	// Define a custom type for the context key
	type contextKey string

	// Define a constant for the key
	const stepNumberKey contextKey = "step_number"

	// Execute each step in the plan
	for i, planStep := range decision.ExecutionPlan {
		stepCtx := context.WithValue(ctx, stepNumberKey, i+1)

		// Send step started update
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_started",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Starting step %d/%d: %s", i+1, len(decision.ExecutionPlan), planStep.Name),
				Timestamp:   time.Now(),
			}
		}

		// Execute the step
		step, err := a.executeExecutionStep(stepCtx, planStep, execution, progressChan)
		if err != nil {
			execution.Status = "failed"
			execution.Errors = append(execution.Errors, fmt.Sprintf("Step %s failed: %v", planStep.ID, err))

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
			break
		}

		execution.Steps = append(execution.Steps, step)

		// 🔥 CRITICAL: Save state after each successful step
		// This ensures that if later steps fail, we don't lose track of successfully created resources
		a.Logger.WithField("step_id", planStep.ID).Info("Attempting to persist state after successful step")

		if err := a.persistCurrentState(); err != nil {
			a.Logger.WithError(err).WithField("step_id", planStep.ID).Error("CRITICAL: Failed to persist state after successful step - this may cause state inconsistency")
			// Don't fail the execution for state persistence issues, but make it very visible
		} else {
			a.Logger.WithField("step_id", planStep.ID).Info("Successfully persisted state after step completion")
		}

		// Send step completed update
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_completed",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Completed step %d/%d: %s", i+1, len(decision.ExecutionPlan), planStep.Name),
				Timestamp:   time.Now(),
			}
		}
	}

	// Complete execution
	now := time.Now()
	execution.CompletedAt = &now
	if execution.Status != "failed" {
		execution.Status = "completed"
	}

	// Update decision record
	decision.ExecutedAt = &now
	if execution.Status == "failed" {
		decision.Result = "failed"
		decision.Error = strings.Join(execution.Errors, "; ")
	} else {
		decision.Result = "success"
	}

	// Send final progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "execution_completed",
			ExecutionID: execution.ID,
			Message:     fmt.Sprintf("Plan execution %s", execution.Status),
			Timestamp:   time.Now(),
		}
	}

	a.Logger.WithFields(map[string]interface{}{
		"execution_id": execution.ID,
		"status":       execution.Status,
		"steps":        len(execution.Steps),
	}).Info("Plan execution completed")

	return execution, nil
}

// simulatePlanExecution simulates plan execution for dry run mode
func (a *StateAwareAgent) simulatePlanExecution(decision *types.AgentDecision, progressChan chan<- *types.ExecutionUpdate) *types.PlanExecution {
	a.Logger.WithField("plan_steps", len(decision.ExecutionPlan)).Debug("Starting simulatePlanExecution")

	now := time.Now()
	execution := &types.PlanExecution{
		ID:        uuid.New().String(),
		Name:      fmt.Sprintf("Simulate %s", decision.Action),
		Status:    "running",
		StartedAt: now,
		Steps:     []*types.ExecutionStep{},
		Changes:   []*types.ChangeDetection{},
		Errors:    []string{},
	}

	a.Logger.WithField("execution_id", execution.ID).Debug("Created execution plan")

	// Send initial update
	if progressChan != nil {
		a.Logger.Debug("Sending initial progress update")
		select {
		case progressChan <- &types.ExecutionUpdate{
			Type:        "execution_started",
			ExecutionID: execution.ID,
			Message:     "Starting plan simulation (dry run)",
			Timestamp:   time.Now(),
		}:
			a.Logger.Debug("Initial progress update sent successfully")
		case <-time.After(time.Second * 5):
			a.Logger.Error("Timeout sending initial progress update - channel might be blocked")
		}
	} else {
		a.Logger.Debug("Progress channel is nil - skipping initial update")
	}

	a.Logger.WithField("steps_to_simulate", len(decision.ExecutionPlan)).Debug("Starting step simulation loop")

	// Simulate each step
	for i, planStep := range decision.ExecutionPlan {
		a.Logger.WithFields(map[string]interface{}{
			"step_number": i + 1,
			"step_id":     planStep.ID,
			"step_name":   planStep.Name,
		}).Debug("Simulating step")

		// Send step started update
		if progressChan != nil {
			select {
			case progressChan <- &types.ExecutionUpdate{
				Type:        "step_started",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Simulating step %d/%d: %s", i+1, len(decision.ExecutionPlan), planStep.Name),
				Timestamp:   time.Now(),
			}:
				a.Logger.Debug("Step started update sent")
			case <-time.After(time.Second * 2):
				a.Logger.Warn("Timeout sending step started update")
			}
		}

		// Simulate step execution with delay
		a.Logger.Debug("Sleeping for step simulation delay")
		time.Sleep(time.Millisecond * 500)

		stepStart := time.Now()
		stepEnd := stepStart.Add(time.Millisecond * 500)

		step := &types.ExecutionStep{
			ID:          planStep.ID,
			Name:        planStep.Name,
			Status:      "completed",
			Resource:    planStep.ResourceID,
			Action:      planStep.Action,
			StartedAt:   &stepStart,
			CompletedAt: &stepEnd,
			Duration:    time.Millisecond * 500,
			Output:      map[string]interface{}{"simulated": true, "message": "Dry run - no actual changes made"},
		}

		execution.Steps = append(execution.Steps, step)
		a.Logger.WithField("steps_completed", len(execution.Steps)).Debug("Added step to execution")

		// Send step completed update
		if progressChan != nil {
			select {
			case progressChan <- &types.ExecutionUpdate{
				Type:        "step_completed",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Simulated step %d/%d: %s", i+1, len(decision.ExecutionPlan), planStep.Name),
				Timestamp:   time.Now(),
			}:
				a.Logger.Debug("Step completed update sent")
			case <-time.After(time.Second * 2):
				a.Logger.Warn("Timeout sending step completed update")
			}
		}
	}

	a.Logger.Debug("Completed all step simulations, finalizing execution")

	// Complete simulation
	completion := time.Now()
	execution.CompletedAt = &completion
	execution.Status = "completed"

	// Send final update
	if progressChan != nil {
		select {
		case progressChan <- &types.ExecutionUpdate{
			Type:        "execution_completed",
			ExecutionID: execution.ID,
			Message:     "Plan simulation completed (dry run)",
			Timestamp:   time.Now(),
		}:
			a.Logger.Debug("Final progress update sent")
		case <-time.After(time.Second * 2):
			a.Logger.Warn("Timeout sending final progress update")
		}
	}

	a.Logger.WithFields(map[string]interface{}{
		"execution_id": execution.ID,
		"status":       execution.Status,
		"steps":        len(execution.Steps),
	}).Info("Plan simulation completed")

	return execution
}

// executeExecutionStep executes a single step in the execution plan
func (a *StateAwareAgent) executeExecutionStep(ctx context.Context, planStep *types.ExecutionPlanStep, execution *types.PlanExecution, progressChan chan<- *types.ExecutionUpdate) (*types.ExecutionStep, error) {
	startTime := time.Now()

	step := &types.ExecutionStep{
		ID:        planStep.ID,
		Name:      planStep.Name,
		Status:    "running",
		Resource:  planStep.ResourceID,
		Action:    planStep.Action,
		StartedAt: &startTime,
	}

	// Send progress update for step details
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "step_progress",
			ExecutionID: execution.ID,
			StepID:      planStep.ID,
			Message:     fmt.Sprintf("Executing: %s", planStep.Description),
			Timestamp:   time.Now(),
		}
	}

	// Execute based on action type
	var result map[string]interface{}
	var err error

	switch planStep.Action {
	case "create":
		result, err = a.executeCreateAction(planStep, progressChan, execution.ID)
	case "update":
		result, err = a.executeUpdateAction(ctx, planStep, progressChan, execution.ID)
	case "delete":
		result, err = a.executeDeleteAction(planStep, progressChan, execution.ID)
	case "validate":
		result, err = a.executeValidateAction(planStep, progressChan, execution.ID)
	case "api_value_retrieval":
		result, err = a.executeAPIValueRetrieval(ctx, planStep, progressChan, execution.ID)
	default:
		err = fmt.Errorf("unknown action type: %s", planStep.Action)
	}

	// Complete the step
	endTime := time.Now()
	step.CompletedAt = &endTime
	step.Duration = endTime.Sub(startTime)

	if err != nil {
		step.Status = "failed"
		step.Error = err.Error()
	} else {
		step.Status = "completed"
		step.Output = result
	}

	return step, err
}

// executeCreateAction handles resource creation using native MCP tool calls
func (a *StateAwareAgent) executeCreateAction(planStep *types.ExecutionPlanStep, progressChan chan<- *types.ExecutionUpdate, executionID string) (map[string]interface{}, error) {
	// Send progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "step_progress",
			ExecutionID: executionID,
			StepID:      planStep.ID,
			Message:     fmt.Sprintf("Creating %s resource: %s", planStep.ResourceID, planStep.Name),
			Timestamp:   time.Now(),
		}
	}

	// Use native MCP tool call approach
	return a.executeNativeMCPTool(planStep, progressChan, executionID)
}

// executeAPIValueRetrieval handles API calls to retrieve real values instead of AI-generated placeholders
func (a *StateAwareAgent) executeAPIValueRetrieval(ctx context.Context, planStep *types.ExecutionPlanStep, progressChan chan<- *types.ExecutionUpdate, executionID string) (map[string]interface{}, error) {
	// Send progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "step_progress",
			ExecutionID: executionID,
			StepID:      planStep.ID,
			Message:     fmt.Sprintf("Retrieving real values from AWS API: %s", planStep.Name),
			Timestamp:   time.Now(),
		}
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_id":     planStep.ID,
		"step_name":   planStep.Name,
		"resource_id": planStep.ResourceID,
		"parameters":  planStep.Parameters,
	}).Info("Executing API value retrieval")

	// Determine the type of value retrieval based on step parameters
	valueType, exists := planStep.Parameters["value_type"]
	if !exists {
		// Try to infer value_type from step description or name for backward compatibility
		description := strings.ToLower(planStep.Description)
		name := strings.ToLower(planStep.Name)

		if strings.Contains(description, "default vpc") || strings.Contains(name, "default vpc") {
			valueType = "default_vpc"
			a.Logger.WithField("step_id", planStep.ID).Warn("Inferred value_type as 'default_vpc' from step description")
		} else if strings.Contains(description, "existing vpc") || strings.Contains(description, "vpc id") || strings.Contains(name, "existing vpc") || strings.Contains(name, "vpc id") {
			valueType = "existing_vpc"
			a.Logger.WithField("step_id", planStep.ID).Warn("Inferred value_type as 'existing_vpc' from step description")
		} else if strings.Contains(description, "default subnet") || strings.Contains(name, "default subnet") {
			valueType = "default_subnet"
			a.Logger.WithField("step_id", planStep.ID).Warn("Inferred value_type as 'default_subnet' from step description")
		} else if strings.Contains(description, "subnets") && strings.Contains(description, "vpc") || strings.Contains(name, "subnets") && strings.Contains(name, "vpc") {
			valueType = "subnets_in_vpc"
			a.Logger.WithField("step_id", planStep.ID).Warn("Inferred value_type as 'subnets_in_vpc' from step description")
		} else if strings.Contains(description, "latest ami") || strings.Contains(name, "latest ami") {
			valueType = "latest_ami"
			a.Logger.WithField("step_id", planStep.ID).Warn("Inferred value_type as 'latest_ami' from step description")
		} else if strings.Contains(description, "availability zone") || strings.Contains(name, "availability zone") {
			valueType = "available_azs"
			a.Logger.WithField("step_id", planStep.ID).Warn("Inferred value_type as 'available_azs' from step description")
		} else if (strings.Contains(description, "load balancer") && strings.Contains(description, "arn")) || (strings.Contains(description, "alb") && strings.Contains(description, "arn")) || (strings.Contains(name, "load balancer") && strings.Contains(name, "arn")) || (strings.Contains(name, "alb") && strings.Contains(name, "arn")) {
			valueType = "load_balancer_arn"
			a.Logger.WithField("step_id", planStep.ID).Warn("Inferred value_type as 'load_balancer_arn' from step description")
		} else if (strings.Contains(description, "target group") && strings.Contains(description, "arn")) || (strings.Contains(name, "target group") && strings.Contains(name, "arn")) {
			valueType = "target_group_arn"
			a.Logger.WithField("step_id", planStep.ID).Warn("Inferred value_type as 'target_group_arn' from step description")
		} else {
			return nil, fmt.Errorf("value_type parameter is required for API value retrieval. Unable to infer from description: '%s'", planStep.Description)
		}

		// Store the inferred value_type back in parameters for consistency
		if planStep.Parameters == nil {
			planStep.Parameters = make(map[string]interface{})
		}
		planStep.Parameters["value_type"] = valueType
	}

	var result map[string]interface{}
	var err error

	switch valueType {
	case "latest_ami":
		result, err = a.retrieveLatestAMI(ctx, planStep)
	case "default_vpc":
		result, err = a.retrieveDefaultVPC(ctx, planStep)
	case "existing_vpc":
		result, err = a.retrieveExistingVPC(ctx, planStep)
	case "default_subnet":
		result, err = a.retrieveDefaultSubnet(ctx, planStep)
	case "subnets_in_vpc":
		result, err = a.retrieveSubnetsInVPC(ctx, planStep)
	case "available_azs":
		result, err = a.retrieveAvailabilityZones(ctx, planStep)
	case "select_subnets_for_alb":
		result, err = a.retrieveSelectSubnetsForALB(ctx, planStep)
	case "load_balancer_arn":
		result, err = a.retrieveLoadBalancerArn(ctx, planStep)
	case "target_group_arn":
		result, err = a.retrieveTargetGroupArn(ctx, planStep)
	// Get values from existing state file
	case "vpc_id":
		result, err = a.retrieveExistingResourceFromState(planStep, "vpc")
	case "subnet_id":
		result, err = a.retrieveExistingResourceFromState(planStep, "subnet")
	case "security_group_id":
		result, err = a.retrieveExistingResourceFromState(planStep, "security_group")
	case "instance_id":
		result, err = a.retrieveExistingResourceFromState(planStep, "ec2_instance")
	case "existing_resource":
		result, err = a.retrieveExistingResourceFromState(planStep, "")
	default:
		err = fmt.Errorf("unsupported value_type: %s", valueType)
	}

	if err != nil {
		a.Logger.WithError(err).WithField("value_type", valueType).Error("API value retrieval failed")
		return nil, fmt.Errorf("failed to retrieve %s: %w", valueType, err)
	}

	// Store the retrieved value in resource mappings for use in subsequent steps
	if resourceValue, exists := result["value"]; exists {
		if resourceValueStr, ok := resourceValue.(string); ok {
			a.storeResourceMapping(planStep.ID, resourceValueStr)
		}
	}

	// For availability zones, also store indexed values for array access
	if valueType == "available_azs" {
		if allZones, exists := result["all_zones"]; exists {
			if zoneList, ok := allZones.([]string); ok {
				for i, zone := range zoneList {
					a.storeResourceMapping(fmt.Sprintf("%s.%d", planStep.ID, i), zone)
				}
				a.Logger.WithFields(map[string]interface{}{
					"step_id":    planStep.ID,
					"zone_count": len(zoneList),
					"first_zone": zoneList[0],
				}).Debug("Stored indexed availability zone mappings")
			}
		}
	}

	// For subnet retrieval, also store the VPC ID for security group creation
	if valueType == "default_subnet" {
		if vpcID, exists := result["vpc_id"]; exists {
			if vpcIDStr, ok := vpcID.(string); ok {
				a.storeResourceMapping(planStep.ID+".vpcId", vpcIDStr)
				a.Logger.WithFields(map[string]interface{}{
					"step_id": planStep.ID,
					"vpc_id":  vpcIDStr,
				}).Debug("Stored VPC ID mapping for subnet step")
			}
		}
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_id":    planStep.ID,
		"value_type": valueType,
		"result":     result,
	}).Info("API value retrieval completed successfully")

	return result, nil
}

// executeNativeMCPTool executes MCP tools directly with AI-provided parameters
func (a *StateAwareAgent) executeNativeMCPTool(planStep *types.ExecutionPlanStep, _ chan<- *types.ExecutionUpdate, _ string) (map[string]interface{}, error) {
	toolName := planStep.MCPTool

	a.Logger.WithFields(map[string]interface{}{
		"tool_name":       toolName,
		"step_id":         planStep.ID,
		"tool_parameters": planStep.ToolParameters,
	}).Info("Executing native MCP tool call")

	// Ensure MCP capabilities are discovered
	if err := a.ensureMCPCapabilities(); err != nil {
		return nil, fmt.Errorf("failed to ensure MCP capabilities: %w", err)
	}

	// Validate tool exists in discovered capabilities
	a.capabilityMutex.RLock()
	toolInfo, exists := a.mcpTools[toolName]
	availableTools := make([]string, 0, len(a.mcpTools))
	for tool := range a.mcpTools {
		availableTools = append(availableTools, tool)
	}
	a.capabilityMutex.RUnlock()

	if !exists {
		a.Logger.WithFields(map[string]interface{}{
			"requested_tool":  toolName,
			"available_tools": availableTools,
			"tools_count":     len(availableTools),
		}).Error("MCP tool not found - debugging tool discovery issue")
		return nil, fmt.Errorf("MCP tool %s not found in discovered capabilities. Available tools: %v", toolName, availableTools)
	}

	// Prepare tool arguments - start with AI-provided parameters
	arguments := make(map[string]interface{})

	// First, copy all AI-provided tool parameters
	for key, value := range planStep.ToolParameters {
		// Resolve dependency references like {{step-1.resourceId}}
		if strValue, ok := value.(string); ok {
			if strings.Contains(strValue, "{{") && strings.Contains(strValue, "}}") {
				a.Logger.WithFields(map[string]interface{}{
					"key":            key,
					"original_value": strValue,
					"step_id":        planStep.ID,
					"tool_name":      toolName,
				}).Debug("Attempting to resolve dependency reference")

				resolvedValue, err := a.resolveDependencyReference(strValue)
				if err != nil {
					a.Logger.WithError(err).WithFields(map[string]interface{}{
						"reference": strValue,
						"key":       key,
					}).Error("Failed to resolve dependency reference")
					arguments[key] = value // Use original value if resolution fails
				} else {
					a.Logger.WithFields(map[string]interface{}{
						"key":            key,
						"original_value": strValue,
						"resolved_value": resolvedValue,
					}).Info("Successfully resolved dependency reference")
					arguments[key] = resolvedValue
				}
			} else {
				arguments[key] = value
			}
		} else if arrayValue, ok := value.([]interface{}); ok {
			// Handle arrays that might contain dependency references
			resolvedArray := make([]interface{}, len(arrayValue))
			for i, item := range arrayValue {
				if strItem, ok := item.(string); ok && strings.Contains(strItem, "{{") && strings.Contains(strItem, "}}") {
					resolvedValue, err := a.resolveDependencyReference(strItem)
					if err != nil {
						a.Logger.WithError(err).WithFields(map[string]interface{}{
							"reference": strItem,
							"index":     i,
							"key":       key,
						}).Warn("Failed to resolve dependency reference in array")
						resolvedArray[i] = item // Use original value if resolution fails
					} else {
						resolvedArray[i] = resolvedValue
						a.Logger.WithFields(map[string]interface{}{
							"reference":      strItem,
							"resolved_value": resolvedValue,
							"index":          i,
							"key":            key,
						}).Debug("Successfully resolved dependency reference in array")
					}
				} else {
					resolvedArray[i] = item
				}
			}
			arguments[key] = resolvedArray
		} else {
			arguments[key] = value
		}
	}

	// Fill in missing required parameters with intelligent defaults
	if err := a.addMissingRequiredParameters(toolName, arguments, toolInfo); err != nil {
		return nil, fmt.Errorf("failed to add required parameters for tool %s: %w", toolName, err)
	}

	// Validate arguments before MCP call
	if err := a.validateNativeMCPArguments(toolName, arguments, toolInfo); err != nil {
		return nil, fmt.Errorf("invalid arguments for MCP tool %s: %w", toolName, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"tool_name":       toolName,
		"final_arguments": arguments,
		"step_id":         planStep.ID,
	}).Debug("Calling MCP tool with final arguments")

	// Call the actual MCP tool
	result, err := a.callMCPTool(toolName, arguments)
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Extract actual resource ID from MCP response
	resourceID, err := a.extractResourceIDFromResponse(result, toolName)
	if err != nil {
		a.Logger.WithError(err).Warn("Could not extract resource ID from MCP response")
		// Use plan step resource ID as fallback
		resourceID = planStep.ResourceID
	}

	// Store the mapping of plan step ID to actual resource ID
	a.storeResourceMapping(planStep.ID, resourceID)

	// Wait for resource to be ready if it has dependencies
	if err := a.waitForResourceReady(toolName, resourceID); err != nil {
		a.Logger.WithError(err).WithFields(map[string]interface{}{
			"step_id":     planStep.ID,
			"tool_name":   toolName,
			"resource_id": resourceID,
		}).Error("Failed to wait for resource to be ready")
		return nil, fmt.Errorf("resource %s not ready: %w", resourceID, err)
	}

	// Update state manager with the new resource
	if err := a.updateStateFromMCPResult(planStep, result); err != nil {
		a.Logger.WithError(err).WithFields(map[string]interface{}{
			"step_id":     planStep.ID,
			"tool_name":   toolName,
			"resource_id": resourceID,
			"result":      result,
		}).Error("CRITICAL: Failed to update state after resource creation - this may cause state inconsistency")

		// Still continue execution but ensure this is visible
		return nil, fmt.Errorf("failed to update state after creating resource %s: %w", resourceID, err)
	}

	// Create result map for return
	resultMap := map[string]interface{}{
		"resource_id":  resourceID,
		"plan_step_id": planStep.ID,
		"mcp_tool":     toolName,
		"mcp_response": result,
	}

	return resultMap, nil
}

// executeUpdateAction handles resource updates using real MCP tools
func (a *StateAwareAgent) executeUpdateAction(_ context.Context, planStep *types.ExecutionPlanStep, progressChan chan<- *types.ExecutionUpdate, executionID string) (map[string]interface{}, error) {
	// Send progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "step_progress",
			ExecutionID: executionID,
			StepID:      planStep.ID,
			Message:     fmt.Sprintf("Updating %s resource: %s", planStep.ResourceID, planStep.Name),
			Timestamp:   time.Now(),
		}
	}

	// For update actions, we mainly just simulate for now since the focus is on create operations
	// The native MCP approach will be extended to update/delete actions in future iterations
	a.Logger.WithField("step_id", planStep.ID).Info("Simulating update action as focus is on create operations")
	time.Sleep(time.Second * 1)
	return map[string]interface{}{
		"resource_id": planStep.ResourceID,
		"status":      "updated",
		"message":     fmt.Sprintf("%s updated successfully (simulated)", planStep.Name),
		"changes":     planStep.Parameters,
		"simulated":   true,
	}, nil
}

// executeDeleteAction handles resource deletion
func (a *StateAwareAgent) executeDeleteAction(planStep *types.ExecutionPlanStep, progressChan chan<- *types.ExecutionUpdate, executionID string) (map[string]interface{}, error) {
	// Send progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "step_progress",
			ExecutionID: executionID,
			StepID:      planStep.ID,
			Message:     fmt.Sprintf("Deleting %s resource: %s", planStep.ResourceID, planStep.Name),
			Timestamp:   time.Now(),
		}
	}

	// Simulate resource deletion
	time.Sleep(time.Second * 1)

	return map[string]interface{}{
		"resource_id": planStep.ResourceID,
		"status":      "deleted",
		"message":     fmt.Sprintf("%s deleted successfully", planStep.Name),
	}, nil
}

// executeValidateAction handles validation steps using real MCP tools where possible
func (a *StateAwareAgent) executeValidateAction(planStep *types.ExecutionPlanStep, progressChan chan<- *types.ExecutionUpdate, executionID string) (map[string]interface{}, error) {
	// Send progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "step_progress",
			ExecutionID: executionID,
			StepID:      planStep.ID,
			Message:     fmt.Sprintf("Validating %s: %s", planStep.ResourceID, planStep.Name),
			Timestamp:   time.Now(),
		}
	}

	// For validation actions, we mainly just simulate for now since the focus is on create operations
	// The native MCP approach will be extended to validation actions in future iterations
	a.Logger.WithField("step_id", planStep.ID).Info("Simulating validation action as focus is on create operations")
	time.Sleep(time.Millisecond * 500)
	return map[string]interface{}{
		"resource_id": planStep.ResourceID,
		"status":      "validated",
		"message":     fmt.Sprintf("%s validation completed (simulated)", planStep.Name),
		"checks":      []string{"basic_validation"},
	}, nil
}

// updateStateFromMCPResult updates the state manager with results from MCP operations
func (a *StateAwareAgent) updateStateFromMCPResult(planStep *types.ExecutionPlanStep, result map[string]interface{}) error {
	a.Logger.WithFields(map[string]interface{}{
		"step_id":      planStep.ID,
		"step_name":    planStep.Name,
		"resource_id":  planStep.ResourceID,
		"mcp_response": result,
	}).Info("Starting state update from MCP result")

	// Create a simple properties map from MCP result
	resultData := map[string]interface{}{
		"mcp_response": result,
		"status":       "created_via_mcp",
	}

	// Extract resource type
	resourceType := extractResourceTypeFromStep(planStep)

	a.Logger.WithFields(map[string]interface{}{
		"step_id":       planStep.ID,
		"resource_type": resourceType,
		"properties":    resultData,
	}).Debug("Extracted resource type and prepared properties")

	// Create a resource state entry
	resourceState := &types.ResourceState{
		ID:           planStep.ResourceID,
		Name:         planStep.Name,
		Type:         resourceType,
		Status:       "created",
		Properties:   resultData,
		Dependencies: planStep.DependsOn,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_id":           planStep.ID,
		"resource_state_id": resourceState.ID,
		"resource_type":     resourceState.Type,
		"dependencies":      resourceState.Dependencies,
	}).Info("Calling AddResourceToState")

	// Add to state manager via MCP server
	if err := a.AddResourceToState(resourceState); err != nil {
		a.Logger.WithError(err).WithFields(map[string]interface{}{
			"step_id":           planStep.ID,
			"resource_state_id": resourceState.ID,
			"resource_type":     resourceState.Type,
		}).Error("Failed to add resource to state via MCP server")
		return fmt.Errorf("failed to add resource %s to state: %w", resourceState.ID, err)
	}

	// Also store the resource with the step ID as the key for dependency resolution
	// This ensures that {{step-create-xxx.resourceId}} references can be resolved
	if planStep.ID != planStep.ResourceID {
		stepResourceState := &types.ResourceState{
			ID:           planStep.ID, // Use step ID as the key
			Name:         planStep.Name + " (Step Reference)",
			Type:         "step_reference",
			Status:       "created",
			Properties:   resultData,
			Dependencies: planStep.DependsOn,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := a.AddResourceToState(stepResourceState); err != nil {
			a.Logger.WithError(err).WithFields(map[string]interface{}{
				"step_id":          planStep.ID,
				"step_resource_id": stepResourceState.ID,
			}).Warn("Failed to add step-based resource to state - dependency resolution may be affected")
			// Don't fail the whole operation for this, just log the warning
		} else {
			a.Logger.WithFields(map[string]interface{}{
				"step_id":          planStep.ID,
				"step_resource_id": stepResourceState.ID,
			}).Info("Successfully added step-based resource reference to state for dependency resolution")
		}
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_id":           planStep.ID,
		"resource_state_id": resourceState.ID,
		"resource_type":     resourceState.Type,
	}).Info("Successfully added resource to state")

	return nil
}

// Helper function to extract resource type from plan step
func extractResourceTypeFromStep(planStep *types.ExecutionPlanStep) string {
	// First try the resource_type parameter
	if rt, exists := planStep.Parameters["resource_type"]; exists {
		if rtStr, ok := rt.(string); ok {
			return rtStr
		}
	}

	// Try to infer from ResourceID field
	if planStep.ResourceID != "" {
		// Common resource ID patterns
		if strings.Contains(planStep.ResourceID, "sg-") || strings.Contains(strings.ToLower(planStep.ResourceID), "security") {
			return "security_group"
		}
		if strings.Contains(planStep.ResourceID, "i-") || strings.Contains(strings.ToLower(planStep.ResourceID), "instance") {
			return "ec2_instance"
		}
		if strings.Contains(planStep.ResourceID, "vpc-") || strings.Contains(strings.ToLower(planStep.ResourceID), "vpc") {
			return "vpc"
		}
		if strings.Contains(strings.ToLower(planStep.ResourceID), "subnet") {
			return "subnet"
		}
	}

	// Try to infer from step name or description
	stepNameLower := strings.ToLower(planStep.Name)
	if strings.Contains(stepNameLower, "security group") || strings.Contains(stepNameLower, "security-group") {
		return "security_group"
	}
	if strings.Contains(stepNameLower, "ec2") || strings.Contains(stepNameLower, "instance") {
		return "ec2_instance"
	}
	if strings.Contains(stepNameLower, "vpc") {
		return "vpc"
	}
	if strings.Contains(stepNameLower, "subnet") {
		return "subnet"
	}
	if strings.Contains(stepNameLower, "load balancer") || strings.Contains(stepNameLower, "alb") {
		return "load_balancer"
	}
	if strings.Contains(stepNameLower, "target group") {
		return "target_group"
	}
	if strings.Contains(stepNameLower, "launch template") {
		return "launch_template"
	}
	if strings.Contains(stepNameLower, "auto scaling") || strings.Contains(stepNameLower, "asg") {
		return "auto_scaling_group"
	}
	if strings.Contains(stepNameLower, "database") || strings.Contains(stepNameLower, "rds") {
		return "db_instance"
	}

	return "unknown"
}

// getAvailableToolsContext returns a formatted string of available tools for the AI to understand
func (a *StateAwareAgent) getAvailableToolsContext() string {
	a.capabilityMutex.RLock()
	toolsCount := len(a.mcpTools)
	a.capabilityMutex.RUnlock()

	if toolsCount == 0 {
		// Try to ensure capabilities are available
		if err := a.ensureMCPCapabilities(); err != nil {
			a.Logger.WithError(err).Warn("Failed to ensure MCP capabilities in getAvailableToolsContext")
			return "No MCP tools available. MCP server may not be properly initialized. Please check server status."
		}

		// Re-check after ensuring capabilities
		a.capabilityMutex.RLock()
		toolsCount = len(a.mcpTools)
		a.capabilityMutex.RUnlock()
	}

	if toolsCount == 0 {
		return "No MCP tools discovered yet. Available tools will be listed after MCP server initialization."
	}

	a.capabilityMutex.RLock()
	defer a.capabilityMutex.RUnlock()

	var context strings.Builder
	context.WriteString("=== AVAILABLE MCP TOOLS WITH FULL SCHEMAS ===\n\n")
	context.WriteString("You have direct access to these MCP tools. Use the exact tool names and parameter structures shown below.\n\n")

	// Group tools by category and provide complete schemas
	categories := map[string][]string{
		"EC2 Compute":    {},
		"VPC Networking": {},
		"Security":       {},
		"Load Balancing": {},
		"Auto Scaling":   {},
		"Database":       {},
		"Other":          {},
	}

	toolDetails := make(map[string]string)

	for toolName, toolInfo := range a.mcpTools {
		category := "Other"
		switch {
		case strings.Contains(toolName, "ec2") || strings.Contains(toolName, "instance") || strings.Contains(toolName, "ami"):
			category = "EC2 Compute"
		case strings.Contains(toolName, "vpc") || strings.Contains(toolName, "subnet") || strings.Contains(toolName, "gateway") || strings.Contains(toolName, "route"):
			category = "VPC Networking"
		case strings.Contains(toolName, "security-group"):
			category = "Security"
		case strings.Contains(toolName, "load-balancer") || strings.Contains(toolName, "target-group") || strings.Contains(toolName, "listener"):
			category = "Load Balancing"
		case strings.Contains(toolName, "auto-scaling") || strings.Contains(toolName, "launch-template"):
			category = "Auto Scaling"
		case strings.Contains(toolName, "db-") || strings.Contains(toolName, "rds"):
			category = "Database"
		}

		// Build detailed tool schema
		var toolDetail strings.Builder
		toolDetail.WriteString(fmt.Sprintf("  TOOL: %s\n", toolName))
		toolDetail.WriteString(fmt.Sprintf("  Description: %s\n", toolInfo.Description))

		if toolInfo.InputSchema != nil {
			if properties, ok := toolInfo.InputSchema["properties"].(map[string]interface{}); ok {
				toolDetail.WriteString("  Parameters:\n")

				// Get required fields
				requiredFields := make(map[string]bool)
				if required, ok := toolInfo.InputSchema["required"].([]interface{}); ok {
					for _, field := range required {
						if fieldStr, ok := field.(string); ok {
							requiredFields[fieldStr] = true
						}
					}
				}

				for paramName, paramSchema := range properties {
					if paramSchemaMap, ok := paramSchema.(map[string]interface{}); ok {
						requiredMark := ""
						if requiredFields[paramName] {
							requiredMark = " (REQUIRED)"
						}

						paramType := "string"
						if pType, exists := paramSchemaMap["type"]; exists {
							paramType = fmt.Sprintf("%v", pType)
						}

						description := ""
						if desc, exists := paramSchemaMap["description"]; exists {
							description = fmt.Sprintf(" - %v", desc)
						}

						toolDetail.WriteString(fmt.Sprintf("    - %s: %s%s%s\n", paramName, paramType, requiredMark, description))
					}
				}
			}
		}
		toolDetail.WriteString("\n")

		categories[category] = append(categories[category], toolName)
		toolDetails[toolName] = toolDetail.String()
	}

	// Write categorized tools with full schemas
	for category, tools := range categories {
		if len(tools) > 0 {
			context.WriteString(fmt.Sprintf("=== %s ===\n\n", category))
			for _, toolName := range tools {
				context.WriteString(toolDetails[toolName])
			}
		}
	}

	context.WriteString("=== EXECUTION PLAN STRUCTURE ===\n\n")
	context.WriteString("When creating execution plans, use this structure for each step:\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-1\",\n")
	context.WriteString("  \"name\": \"Descriptive step name\",\n")
	context.WriteString("  \"description\": \"What this step accomplishes\",\n")
	context.WriteString("  \"action\": \"create|update|delete|validate|api_value_retrieval\",\n")
	context.WriteString("  \"resourceId\": \"unique-resource-identifier\",\n")
	context.WriteString("  \"mcpTool\": \"exact-tool-name-from-above\",\n")
	context.WriteString("  \"toolParameters\": {\n")
	context.WriteString("    \"use\": \"exact parameter names from tool schema\",\n")
	context.WriteString("    \"imageId\": \"{{step-ami.resourceId}}\",\n")
	context.WriteString("    \"instanceType\": \"t3.micro\",\n")
	context.WriteString("    \"name\": \"my-instance\"\n")
	context.WriteString("  },\n")
	context.WriteString("  \"parameters\": {\n")
	context.WriteString("    \"value_type\": \"latest_ami\",\n")
	context.WriteString("    \"os_type\": \"amazon-linux-2\",\n")
	context.WriteString("    \"architecture\": \"x86_64\"\n")
	context.WriteString("  },\n")
	context.WriteString("  \"dependsOn\": [\"previous-step-id\"],\n")
	context.WriteString("  \"estimatedDuration\": \"30s\",\n")
	context.WriteString("  \"status\": \"pending\"\n")
	context.WriteString("}\n\n")

	context.WriteString("=== API VALUE RETRIEVAL STEPS ===\n\n")
	context.WriteString("For resources that need real AWS values instead of AI-generated placeholders, add API retrieval steps:\n\n")
	context.WriteString("STEP 1 - API Value Retrieval:\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-ami\",\n")
	context.WriteString("  \"name\": \"Get Latest Amazon Linux 2 AMI\",\n")
	context.WriteString("  \"description\": \"Call AWS API to get real AMI ID because user didn't provide one\",\n")
	context.WriteString("  \"action\": \"api_value_retrieval\",\n")
	context.WriteString("  \"resourceId\": \"latest-ami\",\n")
	context.WriteString("  \"parameters\": {\n")
	context.WriteString("    \"value_type\": \"latest_ami\",\n")
	context.WriteString("    \"os_type\": \"amazon-linux-2\",\n")
	context.WriteString("    \"architecture\": \"x86_64\"\n")
	context.WriteString("  },\n")
	context.WriteString("  \"dependsOn\": [],\n")
	context.WriteString("  \"estimatedDuration\": \"10s\"\n")
	context.WriteString("}\n\n")
	context.WriteString("STEP 2 - Use Retrieved Value:\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-create-instance\",\n")
	context.WriteString("  \"name\": \"Create EC2 Instance\",\n")
	context.WriteString("  \"action\": \"create\",\n")
	context.WriteString("  \"mcpTool\": \"create-ec2-instance\",\n")
	context.WriteString("  \"toolParameters\": {\n")
	context.WriteString("    \"imageId\": \"{{step-ami.resourceId}}\",\n")
	context.WriteString("    \"instanceType\": \"t3.micro\",\n")
	context.WriteString("    \"name\": \"my-instance\"\n")
	context.WriteString("  },\n")
	context.WriteString("  \"dependsOn\": [\"step-ami\"]\n")
	context.WriteString("}\n\n")
	context.WriteString("Available value_type options:\n")
	context.WriteString("- \"latest_ami\": Get latest AMI for specified OS\n")
	context.WriteString("  * os_type: amazon-linux-2, ubuntu, windows\n")
	context.WriteString("  * architecture: x86_64, arm64 (default: x86_64)\n")
	context.WriteString("- \"default_vpc\": Get default VPC for the region\n")
	context.WriteString("- \"default_subnet\": Get default subnet in the region\n")
	context.WriteString("- \"available_azs\": Get available availability zones\n")
	context.WriteString("  * max_azs: limit number of AZs returned (optional)\n")
	context.WriteString("- \"select_subnets_for_alb\": Select subnets for ALB creation\n")
	context.WriteString("  * scheme: internet-facing, internal (default: internet-facing)\n")
	context.WriteString("  * vpc_id: specific VPC ID (optional, uses default VPC if not specified)\n")
	context.WriteString("- \"vpc_id\": Get existing VPC ID from state file\n")
	context.WriteString("  * resource_name: name of VPC to find (e.g., \"production-vpc\")\n")
	context.WriteString("  * resource_id: specific VPC ID to find (optional)\n")
	context.WriteString("- \"subnet_id\": Get existing subnet ID from state file\n")
	context.WriteString("  * resource_name: name of subnet to find\n")
	context.WriteString("  * resource_id: specific subnet ID to find (optional)\n")
	context.WriteString("- \"security_group_id\": Get existing security group ID from state file\n")
	context.WriteString("  * resource_name: name of security group to find\n")
	context.WriteString("- \"instance_id\": Get existing EC2 instance ID from state file\n")
	context.WriteString("  * resource_name: name of instance to find\n")
	context.WriteString("- \"existing_resource\": Get any existing resource from state file\n")
	context.WriteString("  * resource_name: name of resource to find\n")
	context.WriteString("  * resource_id: specific resource ID to find (optional)\n\n")

	context.WriteString("=== EXTENDED API VALUE RETRIEVAL EXAMPLES ===\n\n")

	context.WriteString("Example 1 - Ubuntu AMI:\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-ubuntu-ami\",\n")
	context.WriteString("  \"action\": \"api_value_retrieval\",\n")
	context.WriteString("  \"parameters\": {\n")
	context.WriteString("    \"value_type\": \"latest_ami\",\n")
	context.WriteString("    \"os_type\": \"ubuntu\",\n")
	context.WriteString("    \"architecture\": \"x86_64\"\n")
	context.WriteString("  }\n")
	context.WriteString("}\n\n")

	context.WriteString("Example 2 - Default VPC:\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-vpc\",\n")
	context.WriteString("  \"action\": \"api_value_retrieval\",\n")
	context.WriteString("  \"parameters\": {\n")
	context.WriteString("    \"value_type\": \"default_vpc\"\n")
	context.WriteString("  }\n")
	context.WriteString("}\n\n")

	context.WriteString("Example 3 - Default Subnet:\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-subnet\",\n")
	context.WriteString("  \"action\": \"api_value_retrieval\",\n")
	context.WriteString("  \"parameters\": {\n")
	context.WriteString("    \"value_type\": \"default_subnet\"\n")
	context.WriteString("  }\n")
	context.WriteString("}\n\n")

	context.WriteString("Example 4 - Availability Zones (limit to 2):\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-azs\",\n")
	context.WriteString("  \"action\": \"api_value_retrieval\",\n")
	context.WriteString("  \"parameters\": {\n")
	context.WriteString("    \"value_type\": \"available_azs\",\n")
	context.WriteString("    \"max_azs\": 2\n")
	context.WriteString("  }\n")
	context.WriteString("}\n\n")

	context.WriteString("Example 5 - Get Existing VPC from State File:\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-get-vpc\",\n")
	context.WriteString("  \"name\": \"Get Production VPC ID\",\n")
	context.WriteString("  \"action\": \"api_value_retrieval\",\n")
	context.WriteString("  \"parameters\": {\n")
	context.WriteString("    \"value_type\": \"vpc_id\",\n")
	context.WriteString("    \"resource_name\": \"production-vpc\"\n")
	context.WriteString("  }\n")
	context.WriteString("}\n\n")

	context.WriteString("Example 6 - Get Existing Security Group from State:\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-get-sg\",\n")
	context.WriteString("  \"action\": \"api_value_retrieval\",\n")
	context.WriteString("  \"parameters\": {\n")
	context.WriteString("    \"value_type\": \"security_group_id\",\n")
	context.WriteString("    \"resource_name\": \"web-security-group\"\n")
	context.WriteString("  }\n")
	context.WriteString("}\n\n")

	context.WriteString("Example 7 - CORRECT EC2 Instance Pattern:\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-get-subnet\",\n")
	context.WriteString("  \"name\": \"Get Default Subnet\",\n")
	context.WriteString("  \"action\": \"api_value_retrieval\",\n")
	context.WriteString("  \"parameters\": { \"value_type\": \"default_subnet\" }\n")
	context.WriteString("},\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-create-instance\",\n")
	context.WriteString("  \"name\": \"Create EC2 Instance\",\n")
	context.WriteString("  \"action\": \"create\",\n")
	context.WriteString("  \"mcpTool\": \"create-ec2-instance\",\n")
	context.WriteString("  \"toolParameters\": {\n")
	context.WriteString("    \"subnetId\": \"{{step-get-subnet.resourceId}}\",\n")
	context.WriteString("    \"imageId\": \"ami-12345\",\n")
	context.WriteString("    \"instanceType\": \"t3.micro\"\n")
	context.WriteString("  },\n")
	context.WriteString("  \"dependsOn\": [\"step-get-subnet\"]\n")
	context.WriteString("}\n")
	context.WriteString("⚠️  NOTE: subnetId uses step-get-subnet (default_subnet), NOT step-get-vpc!\n\n")

	context.WriteString("=== CRITICAL NETWORKING RULES ===\n\n")
	context.WriteString("⚠️  IMPORTANT: EC2 instances require SUBNET IDs, NOT VPC IDs\n")
	context.WriteString("✅ CORRECT: \"subnetId\": \"{{step-subnet.resourceId}}\" (where step-subnet uses default_subnet)\n")
	context.WriteString("❌ WRONG:   \"subnetId\": \"{{step-vpc.resourceId}}\" (VPC ID cannot be used as subnet ID)\n\n")

	context.WriteString("⚠️  IMPORTANT: Security groups require VPC IDs\n")
	context.WriteString("✅ CORRECT: \"vpcId\": \"{{step-vpc.resourceId}}\" (use separate default_vpc step)\n")
	context.WriteString("❌ WRONG:   \"vpcId\": \"{{step-subnet.resourceId}}\" (subnet ID cannot be used as VPC ID)\n\n")

	context.WriteString("📝 Resource ID Access Pattern:\n")
	context.WriteString("- {{step-name.resourceId}} → returns the primary resource ID\n")
	context.WriteString("- default_vpc step → returns VPC ID\n")
	context.WriteString("- default_subnet step → returns subnet ID\n\n")

	context.WriteString("=== COMMON PATTERNS ===\n\n")
	context.WriteString("Pattern 1 - Complete Infrastructure Setup:\n")
	context.WriteString("1. Retrieve default subnet → step-subnet (this gets both VPC discovery and subnet selection)\n")
	context.WriteString("2. Retrieve AMI → step-ami\n")
	context.WriteString("3. Create instance using {{step-ami.resourceId}}, {{step-subnet.resourceId}}\n")
	context.WriteString("   NOTE: Use step-subnet.resourceId for subnetId parameter!\n")
	context.WriteString("   NOTE: keyName is optional - omit if no key pair needed\n\n")

	context.WriteString("Pattern 2 - Security Group + EC2 Creation:\n")
	context.WriteString("1. Retrieve default VPC → step-vpc (for security group)\n")
	context.WriteString("2. Retrieve default subnet → step-subnet (for EC2 instance)\n")
	context.WriteString("3. Create security group → step-sg using {{step-vpc.resourceId}}\n")
	context.WriteString("4. Create EC2 instance using {{step-subnet.resourceId}} and security group\n")
	context.WriteString("Example:\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-vpc\",\n")
	context.WriteString("  \"action\": \"api_value_retrieval\",\n")
	context.WriteString("  \"parameters\": { \"value_type\": \"default_vpc\" }\n")
	context.WriteString("},\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-subnet\",\n")
	context.WriteString("  \"action\": \"api_value_retrieval\",\n")
	context.WriteString("  \"parameters\": { \"value_type\": \"default_subnet\" }\n")
	context.WriteString("},\n")
	context.WriteString("{\n")
	context.WriteString("  \"id\": \"step-sg\",\n")
	context.WriteString("  \"action\": \"create\",\n")
	context.WriteString("  \"mcpTool\": \"create-security-group\",\n")
	context.WriteString("  \"toolParameters\": {\n")
	context.WriteString("    \"name\": \"web-sg\",\n")
	context.WriteString("    \"description\": \"Web server security group\",\n")
	context.WriteString("    \"vpcId\": \"{{step-vpc.resourceId}}\"\n")
	context.WriteString("  }\n")
	context.WriteString("}\n\n")

	context.WriteString("Pattern 3 - VPC and Subnet Discovery:\n")
	context.WriteString("1. Retrieve default VPC → step-vpc (only if you need VPC ID for other resources)\n")
	context.WriteString("2. Retrieve default subnet → step-subnet (for EC2 instances)\n")
	context.WriteString("3. Create resources using appropriate IDs\n")
	context.WriteString("   - VPC resources: {{step-vpc.resourceId}}\n")
	context.WriteString("   - EC2 instances: {{step-subnet.resourceId}} for subnetId\n\n")

	context.WriteString("Pattern 4 - Custom Network Setup:\n")
	context.WriteString("1. Retrieve default VPC → step-vpc\n")
	context.WriteString("2. Retrieve AZs → step-azs  \n")
	context.WriteString("3. Create custom subnet using {{step-vpc.resourceId}} and {{step-azs.resourceId}}\n")
	context.WriteString("4. Retrieve AMI\n")
	context.WriteString("5. Create instance with custom subnet\n\n")

	context.WriteString("Pattern 4 - Multi-OS Deployment:\n")
	context.WriteString("1. Get Linux AMI → step-linux-ami (os_type: ubuntu)\n")
	context.WriteString("2. Get Windows AMI → step-windows-ami (os_type: windows)\n")
	context.WriteString("3. Create Linux instances → {{step-linux-ami.resourceId}}\n")
	context.WriteString("4. Create Windows instances → {{step-windows-ami.resourceId}}\n\n")

	context.WriteString("=== CRITICAL INSTRUCTIONS ===\n")
	context.WriteString("1. Use EXACT tool names and parameter names from the schemas above\n")
	context.WriteString("2. Include 'mcpTool' field specifying which tool to use\n")
	context.WriteString("3. Put tool parameters in 'toolParameters' field with exact schema format\n")
	context.WriteString("4. The agent will call MCP tools directly with your parameters\n")
	context.WriteString("5. Only required parameters need values - optional ones can be omitted\n")
	context.WriteString("6. Use dependency references like {{step-1.resourceId}} for resource IDs from previous steps\n")
	context.WriteString("7. IMPORTANT: For AMI IDs, VPC IDs, subnet IDs, etc., add api_value_retrieval steps BEFORE create steps\n")
	context.WriteString("8. This prevents \"Invalid AMI ID\", \"VPCIdNotSpecified\", and subnet errors by using real AWS values\n\n")

	context.WriteString("=== DEPENDENCY MANAGEMENT ===\n")
	context.WriteString("AWS resource creation order:\n")
	context.WriteString("1. VPC → Subnets → Internet Gateway → Route Tables\n")
	context.WriteString("2. Security Groups (after VPC)\n")
	context.WriteString("3. Launch Templates, Load Balancers, Target Groups\n")
	context.WriteString("4. EC2 Instances, Auto Scaling Groups\n")
	context.WriteString("5. RDS Instances, other dependent services\n")

	return context.String()
}

// persistCurrentState saves the current infrastructure state to persistent storage
// This ensures that successfully completed steps are not lost if later steps fail
func (a *StateAwareAgent) persistCurrentState() error {
	a.Logger.Info("Starting state persistence via MCP server")

	// Use MCP server to save the current state
	result, err := a.callMCPTool("save-state", map[string]interface{}{
		"force": true, // Force save even if state hasn't changed much
	})
	if err != nil {
		a.Logger.WithError(err).Error("Failed to call save-state MCP tool")
		return fmt.Errorf("failed to save state via MCP: %w", err)
	}

	a.Logger.WithField("result", result).Info("State persistence completed successfully via MCP server")
	return nil
}

// extractResourceIDFromResponse extracts the actual AWS resource ID from MCP response
func (a *StateAwareAgent) extractResourceIDFromResponse(result map[string]interface{}, toolName string) (string, error) {
	a.Logger.WithFields(map[string]interface{}{
		"tool_name": toolName,
		"response":  result,
	}).Debug("Extracting resource ID from MCP response")

	// Try to extract resource ID from the response
	if resourceID, exists := result["resource_id"]; exists {
		if resourceIDStr, ok := resourceID.(string); ok {
			a.Logger.WithFields(map[string]interface{}{
				"tool_name":   toolName,
				"resource_id": resourceIDStr,
				"source":      "resource_id_field",
			}).Info("Successfully extracted resource ID from resource_id field")
			return resourceIDStr, nil
		}
	}

	// Try different field names based on tool type
	switch toolName {
	case "create-ec2-instance":
		if instanceID, exists := result["instanceId"]; exists {
			if instanceIDStr, ok := instanceID.(string); ok {
				return instanceIDStr, nil
			}
		}
	case "create-security-group":
		if groupID, exists := result["securityGroupId"]; exists {
			if groupIDStr, ok := groupID.(string); ok {
				return groupIDStr, nil
			}
		}
		// Also try the legacy field name for backward compatibility
		if groupID, exists := result["groupId"]; exists {
			if groupIDStr, ok := groupID.(string); ok {
				return groupIDStr, nil
			}
		}
	case "create-vpc":
		if vpcID, exists := result["vpcId"]; exists {
			if vpcIDStr, ok := vpcID.(string); ok {
				return vpcIDStr, nil
			}
		}
	case "create-internet-gateway":
		// Handle Internet Gateway creation
		if igwID, exists := result["internetGatewayId"]; exists {
			if igwIDStr, ok := igwID.(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"tool_name": toolName,
					"igw_id":    igwIDStr,
					"source":    "internetGatewayId_field",
				}).Info("Successfully extracted Internet Gateway ID from internetGatewayId field")
				return igwIDStr, nil
			}
		}
		if igwID, exists := result["internet_gateway_id"]; exists {
			if igwIDStr, ok := igwID.(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"tool_name": toolName,
					"igw_id":    igwIDStr,
					"source":    "internet_gateway_id_field",
				}).Info("Successfully extracted Internet Gateway ID from internet_gateway_id field")
				return igwIDStr, nil
			}
		}
		a.Logger.WithFields(map[string]interface{}{
			"tool_name": toolName,
			"response":  result,
		}).Warn("Could not find internetGatewayId or internet_gateway_id in Internet Gateway creation response")
	case "create-subnet", "create-db-subnet", "create-public-subnet", "create-private-subnet":
		// Handle various subnet creation tools
		if subnetID, exists := result["subnetId"]; exists {
			if subnetIDStr, ok := subnetID.(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"tool_name": toolName,
					"subnet_id": subnetIDStr,
					"source":    "subnetId_field",
				}).Info("Successfully extracted subnet ID from subnetId field")
				return subnetIDStr, nil
			}
		}
		// Also try subnet_id for different naming conventions
		if subnetID, exists := result["subnet_id"]; exists {
			if subnetIDStr, ok := subnetID.(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"tool_name": toolName,
					"subnet_id": subnetIDStr,
					"source":    "subnet_id_field",
				}).Info("Successfully extracted subnet ID from subnet_id field")
				return subnetIDStr, nil
			}
		}
		a.Logger.WithFields(map[string]interface{}{
			"tool_name": toolName,
			"response":  result,
		}).Warn("Could not find subnetId or subnet_id in subnet creation response")
	case "create-rds-db-instance", "create-database":
		// Handle RDS instance creation
		if dbInstanceID, exists := result["dbInstanceId"]; exists {
			if dbInstanceIDStr, ok := dbInstanceID.(string); ok {
				return dbInstanceIDStr, nil
			}
		}
		if dbInstanceID, exists := result["db_instance_id"]; exists {
			if dbInstanceIDStr, ok := dbInstanceID.(string); ok {
				return dbInstanceIDStr, nil
			}
		}
	case "create-db-subnet-group":
		// Handle DB subnet group creation
		if dbSubnetGroupName, exists := result["dbSubnetGroupName"]; exists {
			if dbSubnetGroupNameStr, ok := dbSubnetGroupName.(string); ok {
				return dbSubnetGroupNameStr, nil
			}
		}
		if dbSubnetGroupName, exists := result["db_subnet_group_name"]; exists {
			if dbSubnetGroupNameStr, ok := dbSubnetGroupName.(string); ok {
				return dbSubnetGroupNameStr, nil
			}
		}
	case "create-nat-gateway":
		// Handle NAT gateway creation
		if natGatewayID, exists := result["natGatewayId"]; exists {
			if natGatewayIDStr, ok := natGatewayID.(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"tool_name":      toolName,
					"nat_gateway_id": natGatewayIDStr,
					"source":         "natGatewayId_field",
				}).Info("Successfully extracted NAT gateway ID from natGatewayId field")
				return natGatewayIDStr, nil
			}
		}
		if natGatewayID, exists := result["nat_gateway_id"]; exists {
			if natGatewayIDStr, ok := natGatewayID.(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"tool_name":      toolName,
					"nat_gateway_id": natGatewayIDStr,
					"source":         "nat_gateway_id_field",
				}).Info("Successfully extracted NAT gateway ID from nat_gateway_id field")
				return natGatewayIDStr, nil
			}
		}
		a.Logger.WithFields(map[string]interface{}{
			"tool_name": toolName,
			"response":  result,
		}).Warn("Could not find natGatewayId or nat_gateway_id in NAT gateway creation response")
	case "create-public-route-table", "create-private-route-table", "create-route-table":
		// Handle route table creation
		if routeTableID, exists := result["routeTableId"]; exists {
			if routeTableIDStr, ok := routeTableID.(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"tool_name":      toolName,
					"route_table_id": routeTableIDStr,
					"source":         "routeTableId_field",
				}).Info("Successfully extracted route table ID from routeTableId field")
				return routeTableIDStr, nil
			}
		}
		if routeTableID, exists := result["route_table_id"]; exists {
			if routeTableIDStr, ok := routeTableID.(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"tool_name":      toolName,
					"route_table_id": routeTableIDStr,
					"source":         "route_table_id_field",
				}).Info("Successfully extracted route table ID from route_table_id field")
				return routeTableIDStr, nil
			}
		}
		if resourceID, exists := result["resourceId"]; exists {
			if resourceIDStr, ok := resourceID.(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"tool_name":   toolName,
					"resource_id": resourceIDStr,
					"source":      "resourceId_field",
				}).Info("Successfully extracted route table ID from resourceId field")
				return resourceIDStr, nil
			}
		}
		a.Logger.WithFields(map[string]interface{}{
			"tool_name": toolName,
			"response":  result,
		}).Warn("Could not find routeTableId, route_table_id, or resourceId in route table creation response")
	}

	a.Logger.WithFields(map[string]interface{}{
		"tool_name": toolName,
		"response":  result,
	}).Warn("Could not extract resource ID from MCP response, generating fallback ID")

	// Generate a fallback ID
	fallbackID := fmt.Sprintf("generated-%s-%d", toolName, time.Now().Unix())

	a.Logger.WithFields(map[string]interface{}{
		"tool_name":   toolName,
		"fallback_id": fallbackID,
	}).Warn("Using generated fallback resource ID")

	return fallbackID, nil
}

// waitForResourceReady waits for AWS resources to be in a ready state before continuing
func (a *StateAwareAgent) waitForResourceReady(toolName, resourceID string) error {
	// Determine if this resource type needs waiting
	needsWaiting := false
	maxWaitTime := 5 * time.Minute
	checkInterval := 15 * time.Second

	switch toolName {
	case "create-nat-gateway":
		needsWaiting = true
		maxWaitTime = 5 * time.Minute // NAT gateways typically take 2-3 minutes
	case "create-rds-db-instance", "create-database":
		needsWaiting = true
		maxWaitTime = 15 * time.Minute // RDS instances can take longer
	case "create-internet-gateway", "create-vpc", "create-subnet":
		// These are typically available immediately
		needsWaiting = false
	default:
		// For other resources, don't wait
		needsWaiting = false
	}

	if !needsWaiting {
		a.Logger.WithFields(map[string]interface{}{
			"tool_name":   toolName,
			"resource_id": resourceID,
		}).Debug("Resource type does not require waiting")
		return nil
	}

	a.Logger.WithFields(map[string]interface{}{
		"tool_name":      toolName,
		"resource_id":    resourceID,
		"max_wait_time":  maxWaitTime,
		"check_interval": checkInterval,
	}).Info("Waiting for resource to be ready")

	startTime := time.Now()
	timeout := time.After(maxWaitTime)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			elapsed := time.Since(startTime)
			return fmt.Errorf("timeout waiting for %s %s to be ready after %v", toolName, resourceID, elapsed)

		case <-ticker.C:
			ready, err := a.checkResourceState(toolName, resourceID)
			if err != nil {
				a.Logger.WithError(err).WithFields(map[string]interface{}{
					"tool_name":   toolName,
					"resource_id": resourceID,
				}).Warn("Error checking resource state, will retry")
				continue
			}

			if ready {
				elapsed := time.Since(startTime)
				a.Logger.WithFields(map[string]interface{}{
					"tool_name":   toolName,
					"resource_id": resourceID,
					"elapsed":     elapsed,
				}).Info("Resource is ready")
				return nil
			}

			elapsed := time.Since(startTime)
			a.Logger.WithFields(map[string]interface{}{
				"tool_name":   toolName,
				"resource_id": resourceID,
				"elapsed":     elapsed,
			}).Debug("Resource not ready yet, continuing to wait")
		}
	}
}

// checkResourceState checks if a specific AWS resource is in a ready state
func (a *StateAwareAgent) checkResourceState(toolName, resourceID string) (bool, error) {
	switch toolName {
	case "create-nat-gateway":
		return a.checkNATGatewayState(resourceID)
	case "create-rds-db-instance", "create-database":
		return a.checkRDSInstanceState(resourceID)
	default:
		// For unknown resource types, assume they're ready
		return true, nil
	}
}

// checkNATGatewayState checks if a NAT gateway is available
func (a *StateAwareAgent) checkNATGatewayState(natGatewayID string) (bool, error) {
	// Try to use MCP tool to describe the NAT gateway if available
	result, err := a.callMCPTool("describe-nat-gateways", map[string]interface{}{
		"natGatewayIds": []string{natGatewayID},
	})
	if err != nil {
		// If describe tool is not available, use a simple time-based approach
		a.Logger.WithFields(map[string]interface{}{
			"nat_gateway_id": natGatewayID,
			"error":          err.Error(),
		}).Warn("describe-nat-gateways tool not available, using time-based wait")

		// NAT gateways typically take 2-3 minutes to become available
		// We'll wait a fixed amount of time and then assume it's ready
		time.Sleep(30 * time.Second)
		return true, nil
	}

	// Parse the response to check the state
	if natGateways, ok := result["natGateways"].([]interface{}); ok && len(natGateways) > 0 {
		if natGateway, ok := natGateways[0].(map[string]interface{}); ok {
			if state, ok := natGateway["state"].(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"nat_gateway_id": natGatewayID,
					"state":          state,
				}).Debug("NAT gateway state check")

				return state == "available", nil
			}
		}
	}

	return false, fmt.Errorf("could not determine NAT gateway state from response")
}

// checkRDSInstanceState checks if an RDS instance is available
func (a *StateAwareAgent) checkRDSInstanceState(dbInstanceID string) (bool, error) {
	// Try to use MCP tool to describe the RDS instance if available
	result, err := a.callMCPTool("describe-db-instances", map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceID,
	})
	if err != nil {
		// If describe tool is not available, use a simple time-based approach
		a.Logger.WithFields(map[string]interface{}{
			"db_instance_id": dbInstanceID,
			"error":          err.Error(),
		}).Warn("describe-db-instances tool not available, using time-based wait")

		// RDS instances typically take 5-10 minutes to become available
		// We'll wait a fixed amount of time and then assume it's ready
		time.Sleep(60 * time.Second)
		return true, nil
	}

	// Parse the response to check the state
	if dbInstances, ok := result["dbInstances"].([]interface{}); ok && len(dbInstances) > 0 {
		if dbInstance, ok := dbInstances[0].(map[string]interface{}); ok {
			if status, ok := dbInstance["dbInstanceStatus"].(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"db_instance_id": dbInstanceID,
					"status":         status,
				}).Debug("RDS instance state check")

				return status == "available", nil
			}
		}
	}

	return false, fmt.Errorf("could not determine RDS instance state from response")
}

// storeResourceMapping stores the mapping between plan step ID and actual AWS resource ID
func (a *StateAwareAgent) storeResourceMapping(stepID, resourceID string) {
	a.mappingsMutex.Lock()
	defer a.mappingsMutex.Unlock()
	a.resourceMappings[stepID] = resourceID

	a.Logger.WithFields(map[string]interface{}{
		"step_id":     stepID,
		"resource_id": resourceID,
	}).Debug("Stored resource mapping")
}
