package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// AnalyzeInfrastructureStateTool analyzes current infrastructure state and detects drift
type AnalyzeInfrastructureStateTool struct {
	*BaseTool
	deps *StateAwareToolDependencies
}

// NewAnalyzeInfrastructureStateTool creates a new infrastructure state analysis tool
func NewAnalyzeInfrastructureStateTool(deps *StateAwareToolDependencies, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"scan_live": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to scan live infrastructure",
				"default":     true,
			},
		},
	}

	baseTool := NewBaseTool(
		"analyze-infrastructure-state",
		"Analyze current infrastructure state and detect drift",
		"state",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Analyze infrastructure with live scan",
		map[string]interface{}{
			"scan_live": true,
		},
		"Infrastructure analysis completed with drift detection",
	)

	return &AnalyzeInfrastructureStateTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *AnalyzeInfrastructureStateTool) ValidateArguments(args map[string]interface{}) error {
	// Optional validation - scan_live defaults to true
	if scanLive, exists := args["scan_live"]; exists {
		if _, ok := scanLive.(bool); !ok {
			return fmt.Errorf("scan_live must be a boolean")
		}
	}
	return nil
}

// Execute performs the infrastructure state analysis
func (t *AnalyzeInfrastructureStateTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Load state
	if err := t.deps.StateManager.LoadState(ctx); err != nil {
		t.logger.WithError(err).Warn("Failed to load state from file, continuing with current state")
	}

	scanLive := true
	if val, ok := args["scan_live"].(bool); ok {
		scanLive = val
	}

	t.logger.WithField("scan_live", scanLive).Info("Analyzing infrastructure state")

	result := map[string]interface{}{
		"managed_resources": t.deps.StateManager.GetState().Resources,
	}

	if scanLive {
		// Discover live infrastructure
		discoveredResources, err := t.deps.DiscoveryScanner.DiscoverInfrastructure(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
		}

		result["discovered_resources"] = discoveredResources

		// Detect drift
		var driftDetections []*types.ChangeDetection
		for _, resource := range discoveredResources {
			if _, exists := t.deps.StateManager.GetResource(resource.ID); exists {
				drift, err := t.deps.StateManager.DetectDrift(ctx, resource.Properties, resource.ID)
				if err != nil {
					t.logger.WithError(err).WithField("resource_id", resource.ID).Warn("Failed to detect drift")
					continue
				}
				if drift != nil {
					driftDetections = append(driftDetections, drift)
				}
			}
		}

		result["drift_detections"] = driftDetections
	}

	// Marshal the structured data as JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analysis result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonData),
			},
		},
	}, nil
}

// VisualizeDependencyGraphTool generates dependency graph visualization
type VisualizeDependencyGraphTool struct {
	*BaseTool
	deps *StateAwareToolDependencies
}

// NewVisualizeDependencyGraphTool creates a new dependency graph visualization tool
func NewVisualizeDependencyGraphTool(deps *StateAwareToolDependencies, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Output format: text, mermaid",
				"default":     "text",
				"enum":        []string{"text", "mermaid"},
			},
			"include_bottlenecks": map[string]interface{}{
				"type":        "boolean",
				"description": "Include bottleneck analysis",
				"default":     true,
			},
		},
	}

	baseTool := NewBaseTool(
		"visualize-dependency-graph",
		"Generate dependency graph visualization",
		"visualization",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Generate text dependency graph",
		map[string]interface{}{
			"format":              "text",
			"include_bottlenecks": true,
		},
		"Dependency graph generated with bottleneck analysis",
	)

	return &VisualizeDependencyGraphTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *VisualizeDependencyGraphTool) ValidateArguments(args map[string]interface{}) error {
	if format, exists := args["format"]; exists {
		if formatStr, ok := format.(string); ok {
			if formatStr != "text" && formatStr != "mermaid" {
				return fmt.Errorf("format must be 'text' or 'mermaid'")
			}
		} else {
			return fmt.Errorf("format must be a string")
		}
	}

	if includeBottlenecks, exists := args["include_bottlenecks"]; exists {
		if _, ok := includeBottlenecks.(bool); !ok {
			return fmt.Errorf("include_bottlenecks must be a boolean")
		}
	}
	return nil
}

// Execute performs the dependency graph visualization
func (t *VisualizeDependencyGraphTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	format := "text"
	if val, ok := args["format"].(string); ok {
		format = val
	}

	includeBottlenecks := true
	if val, ok := args["include_bottlenecks"].(bool); ok {
		includeBottlenecks = val
	}

	t.logger.WithFields(map[string]interface{}{
		"format":              format,
		"include_bottlenecks": includeBottlenecks,
	}).Info("Visualizing dependency graph")

	// Discover infrastructure to build graph
	discoveredResources, err := t.deps.DiscoveryScanner.DiscoverInfrastructure(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
	}

	// Build dependency graph
	if err := t.deps.GraphManager.BuildGraph(ctx, discoveredResources); err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	analyzer := t.deps.GraphAnalyzer
	var visualization string

	switch format {
	case "mermaid":
		visualization = analyzer.GenerateMermaidDiagram()
	default:
		visualization = analyzer.GenerateTextualRepresentation()
	}

	if includeBottlenecks {
		bottlenecks := analyzer.FindBottlenecks()
		if len(bottlenecks) > 0 {
			visualization += "\n\nBottlenecks:\n"
			for _, bottleneck := range bottlenecks {
				visualization += fmt.Sprintf("- %v\n", bottleneck)
			}
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: visualization,
			},
		},
	}, nil
}

// DetectConflictsTool detects conflicts in infrastructure configuration
type DetectConflictsTool struct {
	*BaseTool
	deps *StateAwareToolDependencies
}

// NewDetectConflictsTool creates a new conflict detection tool
func NewDetectConflictsTool(deps *StateAwareToolDependencies, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"auto_resolve": map[string]interface{}{
				"type":        "boolean",
				"description": "Automatically resolve conflicts where possible",
				"default":     false,
			},
		},
	}

	baseTool := NewBaseTool(
		"detect-infrastructure-conflicts",
		"Detect conflicts in infrastructure configuration",
		"analysis",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Detect conflicts with auto-resolution",
		map[string]interface{}{
			"auto_resolve": true,
		},
		"Conflicts detected and resolved automatically",
	)

	return &DetectConflictsTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *DetectConflictsTool) ValidateArguments(args map[string]interface{}) error {
	if autoResolve, exists := args["auto_resolve"]; exists {
		if _, ok := autoResolve.(bool); !ok {
			return fmt.Errorf("auto_resolve must be a boolean")
		}
	}
	return nil
}

// Execute performs conflict detection
func (t *DetectConflictsTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	autoResolve := false
	if val, ok := args["auto_resolve"].(bool); ok {
		autoResolve = val
	}

	t.logger.WithField("auto_resolve", autoResolve).Info("Detecting infrastructure conflicts")

	// Discover infrastructure
	discoveredResources, err := t.deps.DiscoveryScanner.DiscoverInfrastructure(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
	}

	// Detect conflicts
	conflicts, err := t.deps.ConflictResolver.DetectConflicts(ctx, discoveredResources)
	if err != nil {
		return nil, fmt.Errorf("failed to detect conflicts: %w", err)
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d conflicts\n\n", len(conflicts)))

	for _, conflict := range conflicts {
		output.WriteString(fmt.Sprintf("- %s: %s (Resource: %s)\n",
			conflict.ConflictType, conflict.Details, conflict.ResourceID))
	}

	if autoResolve && len(conflicts) > 0 {
		output.WriteString("\nResolution Results:\n")
		for _, conflict := range conflicts {
			resolution, err := t.deps.ConflictResolver.ResolveConflict(ctx, conflict)
			if err != nil {
				t.logger.WithError(err).WithField("resource_id", conflict.ResourceID).Warn("Failed to resolve conflict")
				output.WriteString(fmt.Sprintf("- Failed to resolve %s: %v\n", conflict.ResourceID, err))
				continue
			}
			output.WriteString(fmt.Sprintf("- Resolved %s: %v\n", conflict.ResourceID, resolution))
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: output.String(),
			},
		},
	}, nil
}

// StateAwareExportTool exports current infrastructure state to JSON with state-aware features
type StateAwareExportTool struct {
	*BaseTool
	deps *StateAwareToolDependencies
}

// NewStateAwareExportTool creates a new state-aware export tool
func NewStateAwareExportTool(deps *StateAwareToolDependencies, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"include_discovered": map[string]interface{}{
				"type":        "boolean",
				"description": "Include discovered (unmanaged) resources",
				"default":     false,
			},
		},
	}

	baseTool := NewBaseTool(
		"export-infrastructure-state",
		"Export current infrastructure state to JSON",
		"state",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Export state with discovered resources",
		map[string]interface{}{
			"include_discovered": true,
		},
		"Infrastructure state exported with discovered resources",
	)

	return &StateAwareExportTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *StateAwareExportTool) ValidateArguments(args map[string]interface{}) error {
	if includeDiscovered, exists := args["include_discovered"]; exists {
		if _, ok := includeDiscovered.(bool); !ok {
			return fmt.Errorf("include_discovered must be a boolean")
		}
	}
	return nil
}

// Execute performs state export
func (t *StateAwareExportTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Load state
	if err := t.deps.StateManager.LoadState(ctx); err != nil {
		t.logger.WithError(err).Warn("Failed to load state from file, continuing with current state")
	}

	includeDiscovered := false
	if val, ok := args["include_discovered"].(bool); ok {
		includeDiscovered = val
	}

	t.logger.WithField("include_discovered", includeDiscovered).Info("Exporting infrastructure state")

	state := t.deps.StateManager.GetState()
	result := map[string]interface{}{
		"managed_state": state,
	}

	if includeDiscovered {
		discoveredResources, err := t.deps.DiscoveryScanner.DiscoverInfrastructure(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
		}
		result["discovered_resources"] = discoveredResources
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonData),
			},
		},
	}, nil
}

// SaveStateTool saves current infrastructure state to persistent storage
type SaveStateTool struct {
	*BaseTool
	deps *StateAwareToolDependencies
}

// NewSaveStateTool creates a new state save tool
func NewSaveStateTool(deps *StateAwareToolDependencies, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"force": map[string]interface{}{
				"type":        "boolean",
				"description": "Force save even if state hasn't changed",
				"default":     false,
			},
		},
	}

	baseTool := NewBaseTool(
		"save-state",
		"Save current infrastructure state to persistent storage",
		"state",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Force save state",
		map[string]interface{}{
			"force": true,
		},
		"Infrastructure state saved successfully",
	)

	return &SaveStateTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *SaveStateTool) ValidateArguments(args map[string]interface{}) error {
	if force, exists := args["force"]; exists {
		if _, ok := force.(bool); !ok {
			return fmt.Errorf("force must be a boolean")
		}
	}
	return nil
}

// Execute performs state save
func (t *SaveStateTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Load state
	if err := t.deps.StateManager.LoadState(ctx); err != nil {
		t.logger.WithError(err).Warn("Failed to load state from file, continuing with current state")
	}

	force := false
	if val, ok := args["force"].(bool); ok {
		force = val
	}

	t.logger.WithField("force", force).Info("Saving infrastructure state")

	// Save state using the state manager
	if err := t.deps.StateManager.SaveState(ctx); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf(`{"success": false, "error": "failed to save state: %v"}`, err),
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: `{"success": true, "message": "Infrastructure state saved successfully"}`,
			},
		},
	}, nil
}

// AddResourceToStateTool adds a resource to the managed infrastructure state
type AddResourceToStateTool struct {
	*BaseTool
	deps *StateAwareToolDependencies
}

// NewAddResourceToStateTool creates a new add resource tool
func NewAddResourceToStateTool(deps *StateAwareToolDependencies, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"resource_id": map[string]interface{}{
				"type":        "string",
				"description": "Resource ID",
			},
			"resource_name": map[string]interface{}{
				"type":        "string",
				"description": "Resource name",
			},
			"resource_type": map[string]interface{}{
				"type":        "string",
				"description": "Resource type",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Resource status",
				"default":     "created",
			},
			"properties": map[string]interface{}{
				"type":        "object",
				"description": "Resource properties",
			},
			"dependencies": map[string]interface{}{
				"type":        "array",
				"description": "Resource dependencies",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"resource_id", "resource_name", "resource_type"},
	}

	baseTool := NewBaseTool(
		"add-resource-to-state",
		"Add a resource to the managed infrastructure state",
		"state",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Add EC2 instance to state",
		map[string]interface{}{
			"resource_id":   "i-1234567890abcdef0",
			"resource_name": "web-server",
			"resource_type": "ec2-instance",
			"status":        "running",
			"properties": map[string]interface{}{
				"instance_type": "t3.micro",
				"ami_id":        "ami-0abcdef1234567890",
			},
		},
		"Resource added to managed state successfully",
	)

	return &AddResourceToStateTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *AddResourceToStateTool) ValidateArguments(args map[string]interface{}) error {
	// Required fields
	for _, field := range []string{"resource_id", "resource_name", "resource_type"} {
		if val, exists := args[field]; !exists {
			return fmt.Errorf("%s is required", field)
		} else if str, ok := val.(string); !ok || str == "" {
			return fmt.Errorf("%s must be a non-empty string", field)
		}
	}

	// Optional fields
	if status, exists := args["status"]; exists {
		if _, ok := status.(string); !ok {
			return fmt.Errorf("status must be a string")
		}
	}

	if properties, exists := args["properties"]; exists {
		if _, ok := properties.(map[string]interface{}); !ok {
			return fmt.Errorf("properties must be an object")
		}
	}

	if dependencies, exists := args["dependencies"]; exists {
		if deps, ok := dependencies.([]interface{}); ok {
			for _, dep := range deps {
				if _, ok := dep.(string); !ok {
					return fmt.Errorf("dependencies must be an array of strings")
				}
			}
		} else {
			return fmt.Errorf("dependencies must be an array")
		}
	}

	return nil
}

// Execute performs adding resource to state
func (t *AddResourceToStateTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Load state
	if err := t.deps.StateManager.LoadState(ctx); err != nil {
		t.logger.WithError(err).Warn("Failed to load state from file, continuing with current state")
	}

	resourceID := args["resource_id"].(string)
	resourceName := args["resource_name"].(string)
	resourceType := args["resource_type"].(string)

	status := "created"
	if val, ok := args["status"].(string); ok {
		status = val
	}

	var properties map[string]interface{}
	if val, ok := args["properties"].(map[string]interface{}); ok {
		properties = val
	} else {
		properties = make(map[string]interface{})
	}

	var dependencies []string
	if val, ok := args["dependencies"].([]interface{}); ok {
		for _, dep := range val {
			if depStr, ok := dep.(string); ok {
				dependencies = append(dependencies, depStr)
			}
		}
	}

	t.logger.WithFields(map[string]interface{}{
		"resource_id":   resourceID,
		"resource_name": resourceName,
		"resource_type": resourceType,
		"status":        status,
	}).Info("Adding resource to managed state")

	// Create resource state
	resourceState := &types.ResourceState{
		ID:           resourceID,
		Name:         resourceName,
		Type:         resourceType,
		Status:       status,
		Properties:   properties,
		Dependencies: dependencies,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Add to state manager
	if err := t.deps.StateManager.AddResource(ctx, resourceState); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf(`{"success": false, "error": "failed to add resource to state: %v"}`, err),
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf(`{"success": true, "message": "Resource %s added to managed state", "resource_id": "%s"}`, resourceID, resourceID),
			},
		},
	}, nil
}

// PlanDeploymentTool generates deployment plan with dependency ordering
type PlanDeploymentTool struct {
	*BaseTool
	deps *StateAwareToolDependencies
}

// NewPlanDeploymentTool creates a new deployment planning tool
func NewPlanDeploymentTool(deps *StateAwareToolDependencies, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"include_levels": map[string]interface{}{
				"type":        "boolean",
				"description": "Include deployment levels for parallel execution",
				"default":     true,
			},
			"target_resources": map[string]interface{}{
				"type":        "array",
				"description": "Target specific resources for deployment planning",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	baseTool := NewBaseTool(
		"plan-infrastructure-deployment",
		"Generate deployment plan with dependency ordering",
		"planning",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Plan deployment with levels",
		map[string]interface{}{
			"include_levels": true,
		},
		"Deployment plan generated with dependency levels",
	)

	return &PlanDeploymentTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *PlanDeploymentTool) ValidateArguments(args map[string]interface{}) error {
	if includeLevels, exists := args["include_levels"]; exists {
		if _, ok := includeLevels.(bool); !ok {
			return fmt.Errorf("include_levels must be a boolean")
		}
	}

	if targetResources, exists := args["target_resources"]; exists {
		if resources, ok := targetResources.([]interface{}); ok {
			for _, resource := range resources {
				if _, ok := resource.(string); !ok {
					return fmt.Errorf("target_resources must be an array of strings")
				}
			}
		} else {
			return fmt.Errorf("target_resources must be an array")
		}
	}

	return nil
}

// Execute performs deployment planning
func (t *PlanDeploymentTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	var targetResources []string
	if val, ok := args["target_resources"].([]interface{}); ok {
		for _, v := range val {
			if str, ok := v.(string); ok {
				targetResources = append(targetResources, str)
			}
		}
	}

	includeLevels := true
	if val, ok := args["include_levels"].(bool); ok {
		includeLevels = val
	}

	t.logger.WithFields(map[string]interface{}{
		"target_resources": targetResources,
		"include_levels":   includeLevels,
	}).Info("Planning infrastructure deployment")

	// Discover infrastructure to build graph
	discoveredResources, err := t.deps.DiscoveryScanner.DiscoverInfrastructure(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
	}

	// Filter target resources if specified
	if len(targetResources) > 0 {
		var filteredResources []*types.ResourceState
		targetMap := make(map[string]bool)
		for _, target := range targetResources {
			targetMap[target] = true
		}

		for _, resource := range discoveredResources {
			if targetMap[resource.ID] {
				filteredResources = append(filteredResources, resource)
			}
		}
		discoveredResources = filteredResources
	}

	// Build dependency graph
	if err := t.deps.GraphManager.BuildGraph(ctx, discoveredResources); err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Get deployment order
	deploymentOrder, err := t.deps.GraphManager.GetDeploymentOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment order: %w", err)
	}

	var output strings.Builder
	output.WriteString("Deployment Plan:\n\n")
	output.WriteString("Deployment Order:\n")
	for i, resourceID := range deploymentOrder {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, resourceID))
	}

	if includeLevels {
		deploymentLevels, err := t.deps.GraphManager.CalculateDeploymentLevels()
		if err != nil {
			return nil, fmt.Errorf("failed to calculate deployment levels: %w", err)
		}

		output.WriteString("\nDeployment Levels (for parallel execution):\n")
		for i, level := range deploymentLevels {
			output.WriteString(fmt.Sprintf("Level %d: %s\n", i+1, strings.Join(level, ", ")))
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: output.String(),
			},
		},
	}, nil
}
