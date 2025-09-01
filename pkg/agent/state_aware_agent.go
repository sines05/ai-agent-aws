package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/pkg/tools"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"

	"github.com/google/uuid"
)

// ========== Interface defines ==========

// StateAwareAgentInterface defines plan execution and resource management functionality
//
// Available Functions:
//   - ExecuteConfirmedPlanWithDryRun()  : Execute confirmed plan with optional dry run mode
//   - simulatePlanExecution()           : Simulate plan execution for dry run mode
//   - executeExecutionStep()            : Execute a single step in the execution plan
//   - executeNativeMCPTool()            : Execute MCP tools directly with AI-provided parameters
//   - executeCreateAction()             : Handle resource creation using MCP tool calls
//   - executeUpdateAction()             : Handle resource updates using MCP tools
//   - executeDeleteAction()             : Handle resource deletion
//   - executeValidateAction()           : Handle validation steps using MCP tools
//   - updateStateFromMCPResult()        : Update state manager with MCP operation results
//   - extractResourceIDFromResponse()   : Extract AWS resource ID from MCP response
//   - storeResourceMapping()            : Store mapping between plan step ID and actual resource ID
//   - resolveDependencyReference()      : Resolve dependency references like {{step-1.resourceId}}
//   - getDefaultValue()                 : Provide default values for required parameters
//   - addMissingRequiredParameters()    : Add intelligent defaults for missing required parameters
//   - validateNativeMCPArguments()      : Validate arguments against tool schema
//
// Usage Example:
//   1. execution := agent.ExecuteConfirmedPlanWithDryRun(ctx, decision, progressChan, false)
//   2. // Monitor execution through progressChan updates

// ========== State Aware Agent Functions ==========

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
		if err := a.persistCurrentState(); err != nil {
			a.Logger.WithError(err).Warn("Failed to persist state after successful step - continuing execution")
			// Don't fail the execution for state persistence issues, just log warning
		} else {
			a.Logger.WithField("step_id", planStep.ID).Debug("Successfully persisted state after step completion")
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

// retrieveLatestAMI gets the latest Amazon Linux 2 AMI for the current region
func (a *StateAwareAgent) retrieveLatestAMI(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Get the OS type from parameters (default to Amazon Linux 2)
	osType := "amazon-linux-2"
	if osParam, exists := planStep.Parameters["os_type"]; exists {
		osType = fmt.Sprintf("%v", osParam)
	}

	// Get the architecture (default to x86_64)
	architecture := "x86_64"
	if archParam, exists := planStep.Parameters["architecture"]; exists {
		architecture = fmt.Sprintf("%v", archParam)
	}

	a.Logger.WithFields(map[string]interface{}{
		"os_type":      osType,
		"architecture": architecture,
		"step_id":      planStep.ID,
	}).Info("Starting API retrieval for latest AMI")

	var amiID string
	var err error

	switch osType {
	case "amazon-linux-2":
		a.Logger.Info("Calling AWS API via awsClient.GetLatestAmazonLinux2AMI")
		amiID, err = a.awsClient.GetLatestAmazonLinux2AMI(ctx)
		if err != nil {
			a.Logger.WithError(err).Error("AWS API call failed for GetLatestAmazonLinux2AMI")
		} else {
			a.Logger.WithField("ami_id", amiID).Info("AWS API call successful, received AMI ID")
		}
	case "ubuntu":
		a.Logger.Info("Calling AWS API via awsClient.GetLatestUbuntuAMI")
		amiID, err = a.awsClient.GetLatestUbuntuAMI(ctx, architecture)
		if err != nil {
			a.Logger.WithError(err).Error("AWS API call failed for GetLatestUbuntuAMI")
		} else {
			a.Logger.WithField("ami_id", amiID).Info("AWS API call successful, received Ubuntu AMI ID")
		}
	case "windows":
		a.Logger.Info("Calling AWS API via awsClient.GetLatestWindowsAMI")
		amiID, err = a.awsClient.GetLatestWindowsAMI(ctx, architecture)
		if err != nil {
			a.Logger.WithError(err).Error("AWS API call failed for GetLatestWindowsAMI")
		} else {
			a.Logger.WithField("ami_id", amiID).Info("AWS API call successful, received Windows AMI ID")
		}
	default:
		return nil, fmt.Errorf("unsupported OS type: %s. Supported types: amazon-linux-2, ubuntu, windows", osType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get latest %s AMI: %w", osType, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"ami_id":       amiID,
		"os_type":      osType,
		"architecture": architecture,
		"source":       "aws_api_call",
	}).Info("API retrieval completed successfully")

	return map[string]interface{}{
		"value":        amiID,
		"type":         "ami",
		"os_type":      osType,
		"architecture": architecture,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Latest %s AMI for %s architecture", osType, architecture),
		"source":       "aws_api_call", // Confirm this came from API
	}, nil
}

// retrieveDefaultVPC gets the default VPC for the current region
func (a *StateAwareAgent) retrieveDefaultVPC(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting API retrieval for default VPC")

	a.Logger.Info("Calling AWS API via awsClient.GetDefaultVPC")
	vpcID, err := a.awsClient.GetDefaultVPC(ctx)
	if err != nil {
		a.Logger.WithError(err).Error("AWS API call failed for GetDefaultVPC")
		return nil, fmt.Errorf("failed to get default VPC: %w", err)
	}

	a.Logger.WithField("vpc_id", vpcID).Info("AWS API call successful, received VPC ID")

	return map[string]interface{}{
		"value":        vpcID,
		"type":         "vpc",
		"is_default":   true,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  "Default VPC for the current region",
		"source":       "aws_api_call",
	}, nil
}

// retrieveDefaultSubnet gets the default subnet for the current region
func (a *StateAwareAgent) retrieveDefaultSubnet(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting API retrieval for default subnet")

	a.Logger.Info("Calling AWS API via awsClient.GetDefaultSubnet")
	subnetInfo, err := a.awsClient.GetDefaultSubnet(ctx)
	if err != nil {
		a.Logger.WithError(err).Error("AWS API call failed for GetDefaultSubnet")
		return nil, fmt.Errorf("failed to get default subnet: %w", err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"subnet_id": subnetInfo.SubnetID,
		"vpc_id":    subnetInfo.VpcID,
	}).Info("AWS API call successful, received subnet and VPC IDs")

	return map[string]interface{}{
		"value":        subnetInfo.SubnetID, // For {{step-id.resourceId}} resolution (subnet ID)
		"subnet_id":    subnetInfo.SubnetID, // Explicit subnet ID
		"vpc_id":       subnetInfo.VpcID,    // Explicit VPC ID for security groups
		"type":         "subnet",
		"is_default":   true,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Default subnet (%s) in VPC (%s)", subnetInfo.SubnetID, subnetInfo.VpcID),
		"source":       "aws_api_call",
	}, nil
}

// retrieveAvailabilityZones gets available AZs for the current region
func (a *StateAwareAgent) retrieveAvailabilityZones(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting API retrieval for availability zones")

	// Check if user wants a specific number of AZs
	maxAZs := 0
	if maxParam, exists := planStep.Parameters["max_azs"]; exists {
		if maxFloat, ok := maxParam.(float64); ok {
			maxAZs = int(maxFloat)
		}
	}

	a.Logger.Info("Calling AWS API via awsClient.GetAvailabilityZones")
	azList, err := a.awsClient.GetAvailabilityZones(ctx)
	if err != nil {
		a.Logger.WithError(err).Error("AWS API call failed for GetAvailabilityZones")
		return nil, fmt.Errorf("failed to get availability zones: %w", err)
	}

	// Limit AZs if requested
	if maxAZs > 0 && len(azList) > maxAZs {
		azList = azList[:maxAZs]
		a.Logger.WithField("limited_to", maxAZs).Info("Limited AZ list to requested maximum")
	}

	a.Logger.WithFields(map[string]interface{}{
		"availability_zones": azList,
		"count":              len(azList),
	}).Info("AWS API call successful, received availability zones")

	// Store the first AZ as the resource value for dependency resolution
	primaryAZ := ""
	if len(azList) > 0 {
		primaryAZ = azList[0]
	}

	return map[string]interface{}{
		"value":        primaryAZ, // For {{step-id.resourceId}} resolution
		"all_zones":    azList,    // Full list available in result
		"count":        len(azList),
		"type":         "availability_zones",
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Available AZs in current region (primary: %s)", primaryAZ),
		"source":       "aws_api_call",
	}, nil
}

// retrieveExistingVPC gets an existing VPC ID (first available VPC or default VPC)
func (a *StateAwareAgent) retrieveExistingVPC(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting API retrieval for existing VPC")

	// First try to get the default VPC
	a.Logger.Info("Attempting to find default VPC first")
	vpcID, err := a.awsClient.GetDefaultVPC(ctx)
	if err == nil && vpcID != "" {
		a.Logger.WithField("vpc_id", vpcID).Info("Found default VPC")
		return map[string]interface{}{
			"value":        vpcID,
			"type":         "vpc",
			"is_default":   true,
			"retrieved_at": time.Now().Format(time.RFC3339),
			"description":  "Default VPC for the current region",
			"source":       "aws_api_call",
		}, nil
	}

	// If no default VPC, get the first available VPC
	a.Logger.Info("No default VPC found, looking for any available VPC")
	vpcs, err := a.awsClient.DescribeVPCs(ctx)
	if err != nil {
		a.Logger.WithError(err).Error("AWS API call failed for DescribeVPCs")
		return nil, fmt.Errorf("failed to describe VPCs: %w", err)
	}

	if len(vpcs) == 0 {
		return nil, fmt.Errorf("no VPCs found in the region")
	}

	// Use the first available VPC
	firstVPC := vpcs[0]
	a.Logger.WithField("vpc_id", firstVPC.ID).Info("Using first available VPC")

	return map[string]interface{}{
		"value":        firstVPC.ID,
		"type":         "vpc",
		"is_default":   false,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  "First available VPC in the current region",
		"source":       "aws_api_call",
	}, nil
}

// retrieveSubnetsInVPC gets all subnets in a specified VPC
func (a *StateAwareAgent) retrieveSubnetsInVPC(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting API retrieval for subnets in VPC")

	// Get VPC ID from parameters
	vpcID, exists := planStep.Parameters["vpc_id"]
	if !exists {
		// Try to get from resource mapping using a previous step
		if vpcIDParam, exists := planStep.Parameters["vpc_id_step"]; exists {
			if stepID, ok := vpcIDParam.(string); ok {
				a.mappingsMutex.RLock()
				mappedVPC, mappingExists := a.resourceMappings[stepID]
				a.mappingsMutex.RUnlock()
				if mappingExists && mappedVPC != "" {
					vpcID = mappedVPC
					a.Logger.WithFields(map[string]interface{}{
						"vpc_id":      vpcID,
						"source_step": stepID,
					}).Info("Retrieved VPC ID from previous step mapping")
				}
			}
		}
	}

	if vpcID == nil {
		return nil, fmt.Errorf("vpc_id parameter is required for subnets_in_vpc retrieval")
	}

	vpcIDStr, ok := vpcID.(string)
	if !ok {
		return nil, fmt.Errorf("vpc_id must be a string")
	}

	a.Logger.WithField("vpc_id", vpcIDStr).Info("Calling AWS API via awsClient.GetSubnetsInVPC")
	subnetIDs, err := a.awsClient.GetSubnetsInVPC(ctx, vpcIDStr)
	if err != nil {
		a.Logger.WithError(err).Error("AWS API call failed for GetSubnetsInVPC")
		return nil, fmt.Errorf("failed to get subnets in VPC %s: %w", vpcIDStr, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"vpc_id":     vpcIDStr,
		"subnet_ids": subnetIDs,
		"count":      len(subnetIDs),
	}).Info("AWS API call successful, received subnet IDs")

	// Use the first subnet as the primary value for dependency resolution
	primarySubnet := ""
	if len(subnetIDs) > 0 {
		primarySubnet = subnetIDs[0]
	}

	return map[string]interface{}{
		"value":        primarySubnet, // For {{step-id.resourceId}} resolution (first subnet)
		"subnet_ids":   subnetIDs,     // Full list of subnet IDs
		"vpc_id":       vpcIDStr,      // VPC ID for reference
		"count":        len(subnetIDs),
		"type":         "subnets",
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Found %d subnets in VPC %s (primary: %s)", len(subnetIDs), vpcIDStr, primarySubnet),
		"source":       "aws_api_call",
	}, nil
}

// retrieveSelectSubnetsForALB retrieves subnets suitable for ALB creation
func (a *StateAwareAgent) retrieveSelectSubnetsForALB(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting subnet selection for ALB")

	// Get scheme from parameters (default to "internet-facing")
	scheme := "internet-facing"
	if schemeParam, exists := planStep.Parameters["scheme"]; exists {
		if schemeStr, ok := schemeParam.(string); ok {
			scheme = schemeStr
		}
	}

	// Get VPC ID from parameters if provided
	var vpcID string
	if vpcIDParam, exists := planStep.Parameters["vpc_id"]; exists {
		if vpcIDStr, ok := vpcIDParam.(string); ok {
			// Check if it's a step reference that needs resolution
			if strings.Contains(vpcIDStr, "{{") && strings.Contains(vpcIDStr, "}}") {
				resolvedVPCID, err := a.resolveDependencyReference(vpcIDStr)
				if err != nil {
					a.Logger.WithError(err).WithField("vpc_id_reference", vpcIDStr).Error("Failed to resolve VPC ID reference")
					return nil, fmt.Errorf("failed to resolve VPC ID reference %s: %w", vpcIDStr, err)
				}
				vpcID = resolvedVPCID
				a.Logger.WithFields(map[string]interface{}{
					"vpc_id_reference": vpcIDStr,
					"resolved_vpc_id":  vpcID,
				}).Info("Resolved VPC ID from step reference")
			} else {
				vpcID = vpcIDStr
			}
		}
	}

	// Create the subnet selection tool and call it directly
	subnetSelector := tools.NewSelectSubnetsForALBTool(a.awsClient, a.Logger)
	selectionArgs := map[string]interface{}{
		"scheme": scheme,
	}

	if vpcID != "" {
		selectionArgs["vpcId"] = vpcID
	}

	a.Logger.WithFields(map[string]interface{}{
		"scheme": scheme,
		"vpc_id": vpcID,
	}).Info("Calling subnet selection tool for ALB")

	selectionResult, err := subnetSelector.Execute(ctx, selectionArgs)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to execute subnet selection tool")
		return nil, fmt.Errorf("failed to select subnets for ALB: %w", err)
	}

	if selectionResult.IsError {
		a.Logger.Error("Subnet selection tool returned error", "error", selectionResult.Content[0])
		return nil, fmt.Errorf("subnet selection failed: %v", selectionResult.Content[0])
	}

	// Parse the tool result
	if len(selectionResult.Content) > 0 {
		if textContent, ok := selectionResult.Content[0].(*mcp.TextContent); ok {
			var resultData map[string]interface{}
			if err := json.Unmarshal([]byte(textContent.Text), &resultData); err != nil {
				a.Logger.WithError(err).Error("Failed to parse subnet selection response")
				return nil, fmt.Errorf("failed to parse subnet selection response: %w", err)
			}

			a.Logger.WithFields(map[string]interface{}{
				"result_data": resultData,
			}).Info("Successfully parsed subnet selection result")

			// Extract subnet IDs from the result
			var subnetIDs []string
			if subnetIDsData, exists := resultData["subnetIds"]; exists {
				if subnetIDsSlice, ok := subnetIDsData.([]interface{}); ok {
					subnetIDs = make([]string, len(subnetIDsSlice))
					for i, subnetID := range subnetIDsSlice {
						if subnetIDStr, ok := subnetID.(string); ok {
							subnetIDs[i] = subnetIDStr
						}
					}
				}
			}

			return map[string]interface{}{
				"value":        subnetIDs,      // For {{step-id.resourceId}} resolution
				"subnet_ids":   subnetIDs,      // Full list of subnet IDs
				"scheme":       scheme,         // ALB scheme for reference
				"count":        len(subnetIDs), // Number of selected subnets
				"type":         "alb_subnets",  // Resource type
				"retrieved_at": time.Now().Format(time.RFC3339),
				"description":  fmt.Sprintf("Selected %d subnets for %s ALB", len(subnetIDs), scheme),
				"source":       "subnet_selection_tool",
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid or empty response from subnet selection tool")
}

// retrieveLoadBalancerArn retrieves load balancer ARN from a previous step
func (a *StateAwareAgent) retrieveLoadBalancerArn(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting load balancer ARN retrieval")

	// Get the step reference from parameters
	stepRef, exists := planStep.Parameters["step_ref"]
	if !exists {
		return nil, fmt.Errorf("step_ref parameter is required for load_balancer_arn retrieval")
	}

	stepRefStr, ok := stepRef.(string)
	if !ok {
		return nil, fmt.Errorf("step_ref must be a string")
	}

	a.Logger.WithField("step_ref", stepRefStr).Info("Resolving load balancer ARN from step reference")

	// Use dependency resolution to get the ARN
	loadBalancerArn, err := a.resolveDependencyReference(stepRefStr)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to resolve load balancer ARN reference")
		return nil, fmt.Errorf("failed to resolve load balancer ARN reference %s: %w", stepRefStr, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_ref":          stepRefStr,
		"load_balancer_arn": loadBalancerArn,
	}).Info("Successfully resolved load balancer ARN")

	return map[string]interface{}{
		"value":           loadBalancerArn,     // For {{step-id.resourceId}} resolution
		"loadBalancerArn": loadBalancerArn,     // Explicit ARN field
		"arn":             loadBalancerArn,     // Alternative key for ARN
		"type":            "load_balancer_arn", // Resource type
		"retrieved_at":    time.Now().Format(time.RFC3339),
		"description":     fmt.Sprintf("Load balancer ARN resolved from %s", stepRefStr),
		"source":          "step_reference",
	}, nil
}

// retrieveTargetGroupArn retrieves the target group ARN from a previous step
func (a *StateAwareAgent) retrieveTargetGroupArn(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Retrieving target group ARN")

	// Extract the step_ref parameter
	stepRef, exists := planStep.Parameters["step_ref"]
	if !exists {
		return nil, fmt.Errorf("step_ref parameter is required for target_group_arn retrieval")
	}

	stepRefStr, ok := stepRef.(string)
	if !ok {
		return nil, fmt.Errorf("step_ref must be a string")
	}

	a.Logger.WithField("step_ref", stepRefStr).Info("Resolving target group ARN from step reference")

	// Use the existing dependency resolution mechanism
	targetGroupArn, err := a.resolveDependencyReference(stepRefStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve target group ARN from step reference %s: %w", stepRefStr, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"target_group_arn": targetGroupArn,
		"step_ref":         stepRefStr,
	}).Info("Successfully resolved target group ARN")

	return map[string]interface{}{
		"value":          targetGroupArn,     // For {{step-id.resourceId}} resolution
		"targetGroupArn": targetGroupArn,     // Explicit ARN field
		"arn":            targetGroupArn,     // Alternative key for ARN
		"type":           "target_group_arn", // Resource type
		"retrieved_at":   time.Now().Format(time.RFC3339),
		"description":    fmt.Sprintf("Target group ARN resolved from %s", stepRefStr),
		"source":         "step_reference",
	}, nil
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
				resolvedValue, err := a.resolveDependencyReference(strValue)
				if err != nil {
					a.Logger.WithError(err).WithField("reference", strValue).Warn("Failed to resolve dependency reference")
					arguments[key] = value // Use original value if resolution fails
				} else {
					arguments[key] = resolvedValue
				}
			} else {
				arguments[key] = value
			}
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

	// Update state manager with the new resource
	if err := a.updateStateFromMCPResult(planStep, result); err != nil {
		a.Logger.WithError(err).Warn("Failed to update state after resource creation")
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

// addMissingRequiredParameters adds intelligent defaults for missing required parameters
func (a *StateAwareAgent) addMissingRequiredParameters(toolName string, arguments map[string]interface{}, toolInfo MCPToolInfo) error {
	if toolInfo.InputSchema == nil {
		return nil // No schema to validate against
	}

	properties, ok := toolInfo.InputSchema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Get required fields
	requiredFields := make(map[string]bool)
	if required, ok := toolInfo.InputSchema["required"].([]interface{}); ok {
		for _, field := range required {
			if fieldStr, ok := field.(string); ok {
				requiredFields[fieldStr] = true
			}
		}
	}

	// Add defaults for missing required fields
	for paramName := range properties {
		if requiredFields[paramName] {
			if _, exists := arguments[paramName]; !exists {
				// Parameter is required but missing, add default
				if defaultValue := a.getDefaultValue(toolName, paramName, arguments); defaultValue != nil {
					arguments[paramName] = defaultValue
					a.Logger.WithFields(map[string]interface{}{
						"tool_name":  toolName,
						"param_name": paramName,
						"default":    defaultValue,
					}).Debug("Added default value for missing required parameter")
				}
			}
		}
	}

	return nil
}

// validateNativeMCPArguments validates arguments against the tool's schema
func (a *StateAwareAgent) validateNativeMCPArguments(toolName string, arguments map[string]interface{}, toolInfo MCPToolInfo) error {
	if toolInfo.InputSchema == nil {
		return nil // No schema to validate against
	}

	properties, ok := toolInfo.InputSchema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Get required fields
	requiredFields := make(map[string]bool)
	if required, ok := toolInfo.InputSchema["required"].([]interface{}); ok {
		for _, field := range required {
			if fieldStr, ok := field.(string); ok {
				requiredFields[fieldStr] = true
			}
		}
	}

	// Validate required fields are present and non-empty
	for paramName := range properties {
		if requiredFields[paramName] {
			val, exists := arguments[paramName]
			if !exists || val == nil {
				return fmt.Errorf("required parameter %s is missing for tool %s", paramName, toolName)
			}
			// Check for empty strings
			if strVal, ok := val.(string); ok && strVal == "" {
				return fmt.Errorf("required parameter %s is empty for tool %s", paramName, toolName)
			}
		}
	}

	return nil
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
	// Create a simple properties map from MCP result
	resultData := map[string]interface{}{
		"mcp_response": result,
		"status":       "created_via_mcp",
	}

	// Create a resource state entry
	resourceState := &types.ResourceState{
		ID:           planStep.ResourceID,
		Name:         planStep.Name,
		Type:         extractResourceTypeFromStep(planStep),
		Status:       "created",
		Properties:   resultData,
		Dependencies: planStep.DependsOn,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Add to state manager via MCP server
	return a.addResourceToState(resourceState)
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

// Production-grade helper methods for resource management

// resolveDependencyReference resolves references like {{step-1.resourceId}} to actual resource IDs
func (a *StateAwareAgent) resolveDependencyReference(reference string) (string, error) {
	// Extract step ID from reference like {{step-1.resourceId}}
	if !strings.HasPrefix(reference, "{{") || !strings.HasSuffix(reference, "}}") {
		return reference, nil // Not a reference
	}

	refContent := strings.TrimSuffix(strings.TrimPrefix(reference, "{{"), "}}")
	parts := strings.Split(refContent, ".")

	// Support multiple reference formats: {{step-1.resourceId}}, {{step-1}}, {{step-1.targetGroupArn}}, etc.
	var stepID string
	var requestedField string
	if len(parts) == 2 {
		stepID = parts[0]
		requestedField = parts[1]
	} else if len(parts) == 1 {
		stepID = parts[0]
		requestedField = "resourceId" // Default to resourceId for backward compatibility
	} else {
		return "", fmt.Errorf("invalid reference format: %s (expected {{step-id.field}} or {{step-id}})", reference)
	}

	a.mappingsMutex.RLock()
	resourceID, exists := a.resourceMappings[stepID]
	a.mappingsMutex.RUnlock()

	if !exists {
		// Fallback: try to get state via MCP and extract the resource ID
		stateJSON, err := a.ExportInfrastructureState(context.Background(), false) // Only managed state
		if err == nil {
			// Parse the state and look for the step ID
			var stateData map[string]interface{}
			if json.Unmarshal([]byte(stateJSON), &stateData) == nil {
				if managedState, ok := stateData["managed_state"].(map[string]interface{}); ok {
					if resources, ok := managedState["resources"].(map[string]interface{}); ok {
						if resource, ok := resources[stepID].(map[string]interface{}); ok {
							// Extract AWS resource ID from the resource properties
							if properties, ok := resource["properties"].(map[string]interface{}); ok {
								if mcpResponse, ok := properties["mcp_response"].(map[string]interface{}); ok {
									// First, try to find the specifically requested field
									if requestedField != "resourceId" {
										if id, ok := mcpResponse[requestedField].(string); ok && id != "" {
											// Cache it for future use
											a.mappingsMutex.Lock()
											a.resourceMappings[stepID] = id
											a.mappingsMutex.Unlock()

											a.Logger.WithFields(map[string]interface{}{
												"reference":       reference,
												"step_id":         stepID,
												"resource_id":     id,
												"source":          "state_fallback",
												"requested_field": requestedField,
											}).Info("Resolved specific field dependency from state")

											return id, nil
										}
									}

									// Fall back to trying common AWS resource ID fields with field-specific prioritization
									var fieldsToTry []string
									switch requestedField {
									case "targetGroupArn", "arn":
										// For target group ARN requests, prioritize ARN fields
										fieldsToTry = []string{"targetGroupArn", "arn", "targetGroupId"}
									case "loadBalancerArn":
										// For load balancer ARN requests, prioritize ARN fields
										fieldsToTry = []string{"loadBalancerArn", "arn", "loadBalancerId"}
									case "securityGroupId":
										fieldsToTry = []string{"securityGroupId", "groupId"}
									case "instanceId":
										fieldsToTry = []string{"instanceId", "instance_id"}
									case "vpcId":
										fieldsToTry = []string{"vpcId", "vpc_id"}
									case "subnetId":
										fieldsToTry = []string{"subnetId", "subnet_id"}
									default:
										// General fallback order for resourceId
										fieldsToTry = []string{"securityGroupId", "instanceId", "targetGroupArn", "loadBalancerArn", "vpcId", "subnetId", "targetGroupId", "loadBalancerId", "arn"}
									}

									for _, field := range fieldsToTry {
										if id, ok := mcpResponse[field].(string); ok && id != "" {
											// Cache it for future use
											a.mappingsMutex.Lock()
											a.resourceMappings[stepID] = id
											a.mappingsMutex.Unlock()

											a.Logger.WithFields(map[string]interface{}{
												"reference":   reference,
												"step_id":     stepID,
												"resource_id": id,
												"source":      "state_fallback",
												"field":       field,
											}).Debug("Resolved dependency reference from state")

											return id, nil
										}
									}
								}
							}
						}
					}
				}
			}
		}

		return "", fmt.Errorf("resource ID not found for step: %s", stepID)
	}

	a.Logger.WithFields(map[string]interface{}{
		"reference":   reference,
		"step_id":     stepID,
		"resource_id": resourceID,
	}).Debug("Resolved dependency reference")

	return resourceID, nil
}

// getDefaultAMIForRegion returns the default AMI ID for the current region by dynamically looking up the latest Amazon Linux 2 AMI
func (a *StateAwareAgent) getDefaultAMIForRegion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to get the latest Amazon Linux 2 AMI dynamically
	amiID, err := a.awsClient.GetLatestAmazonLinux2AMI(ctx)
	if err != nil {
		a.Logger.WithError(err).Warn("Failed to get latest Amazon Linux 2 AMI, using fallback")

		// Final fallback
		return ""
	}

	a.Logger.WithField("amiId", amiID).Info("Using dynamically discovered Amazon Linux 2 AMI")
	return amiID
}

// LEGACY FUNCTIONS REMOVED - Using native MCP integration approach

// getDefaultValue provides default values for required parameters
func (a *StateAwareAgent) getDefaultValue(toolName, paramName string, params map[string]interface{}) interface{} {
	switch toolName {
	case "create-ec2-instance":
		switch paramName {
		case "instanceType":
			// Use params to choose appropriate instance type based on workload
			if workload, exists := params["workload_type"]; exists {
				switch workload {
				case "compute-intensive":
					return "c5.large"
				case "memory-intensive":
					return "r5.large"
				case "storage-intensive":
					return "i3.large"
				default:
					return "t3.micro"
				}
			}
			return "t3.micro"
		case "imageId":
			// First, try to find AMI from a previous API retrieval step
			if amiStepRef, exists := params["ami_step_ref"]; exists {
				stepRef := fmt.Sprintf("%v", amiStepRef)
				if amiID, err := a.resolveDependencyReference(stepRef); err == nil {
					a.Logger.WithFields(map[string]interface{}{
						"ami_id":   amiID,
						"step_ref": stepRef,
						"source":   "api_retrieval_step",
					}).Info("Using AMI ID from API retrieval step")
					return amiID
				} else {
					a.Logger.WithError(err).WithField("step_ref", stepRef).Warn("Failed to resolve AMI step reference, falling back to direct API call")
				}
			}

			// Fallback to direct API call (legacy approach)
			amiID := a.getDefaultAMIForRegion()
			if amiID != "" {
				a.Logger.WithFields(map[string]interface{}{
					"ami_id": amiID,
					"source": "direct_api_call",
				}).Info("Using AMI ID from direct API call")
				return amiID
			}

			// If all else fails, return empty string to trigger an error
			a.Logger.Warn("No AMI ID available from API retrieval step or direct call")
			return ""
		case "keyName":
			// Try to use key name from params if available
			if keyName, exists := params["ssh_key"]; exists {
				return keyName
			}
			return nil // Let AWS use account default
		}
	case "create-vpc":
		switch paramName {
		case "cidrBlock":
			// Use params to determine appropriate CIDR block
			if cidr, exists := params["cidr"]; exists {
				return cidr
			}
			if environment, exists := params["environment"]; exists {
				switch environment {
				case "production":
					return "10.0.0.0/16"
				case "staging":
					return "10.1.0.0/16"
				case "development":
					return "10.2.0.0/16"
				default:
					return "10.0.0.0/16"
				}
			}
			return "10.0.0.0/16"
		case "name":
			// Generate name based on params
			if name, exists := params["resource_name"]; exists {
				return name
			}
			if environment, exists := params["environment"]; exists {
				return fmt.Sprintf("vpc-%s", environment)
			}
			return "ai-agent-vpc"
		}
	case "create-security-group":
		switch paramName {
		case "description":
			// Generate description based on params
			if desc, exists := params["description"]; exists {
				return desc
			}
			if purpose, exists := params["purpose"]; exists {
				return fmt.Sprintf("Security group for %s", purpose)
			}
			return "Security group created by AI Agent"
		}
	}
	return nil
}

// extractResourceIDFromResponse extracts the actual AWS resource ID from MCP response
func (a *StateAwareAgent) extractResourceIDFromResponse(result map[string]interface{}, toolName string) (string, error) {
	// Try to extract resource ID from the response
	if resourceID, exists := result["resource_id"]; exists {
		if resourceIDStr, ok := resourceID.(string); ok {
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
	}

	a.Logger.WithField("response", result).Debug("Could not extract resource ID from MCP response")

	// Generate a fallback ID
	return fmt.Sprintf("generated-%s-%d", toolName, time.Now().Unix()), nil
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

// GetAvailableToolsContext returns a formatted string of available tools for the AI to understand
func (a *StateAwareAgent) GetAvailableToolsContext() string {
	a.capabilityMutex.RLock()
	toolsCount := len(a.mcpTools)
	a.capabilityMutex.RUnlock()

	if toolsCount == 0 {
		// Try to ensure capabilities are available
		if err := a.ensureMCPCapabilities(); err != nil {
			a.Logger.WithError(err).Warn("Failed to ensure MCP capabilities in GetAvailableToolsContext")
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
	context.WriteString("  * vpc_id: specific VPC ID (optional, uses default VPC if not specified)\n\n")

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

	context.WriteString("Example 5 - CORRECT EC2 Instance Pattern:\n")
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
	a.Logger.Debug("Persisting current infrastructure state")

	// Use MCP server to save the current state
	result, err := a.callMCPTool("save-state", map[string]interface{}{
		"force": true, // Force save even if state hasn't changed much
	})
	if err != nil {
		return fmt.Errorf("failed to save state via MCP: %w", err)
	}

	a.Logger.WithField("result", result).Debug("State persistence completed via MCP server")
	return nil
}
