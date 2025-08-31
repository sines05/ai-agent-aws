package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/adapters"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// CreateLoadBalancerTool implements MCPTool for creating load balancers
type CreateLoadBalancerTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreateLoadBalancerTool creates a new load balancer creation tool
func NewCreateLoadBalancerTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name for the load balancer",
			},
			"scheme": map[string]interface{}{
				"type":        "string",
				"description": "The scheme (internet-facing or internal)",
				"default":     "internet-facing",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "The type (application, network, gateway)",
				"default":     "application",
			},
			"subnetIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of subnet IDs",
			},
			"securityGroupIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of security group IDs",
			},
		},
		"required": []string{"name", "subnetIds"},
	}

	return &CreateLoadBalancerTool{
		BaseTool: &BaseTool{
			name:        "create-load-balancer",
			description: "Create a new application load balancer",
			inputSchema: inputSchema,
			logger:      logger,
		},
		adapter: adapters.NewALBSpecializedAdapter(awsClient, logger),
	}
}

func (t *CreateLoadBalancerTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, ok := arguments["name"].(string)
	if !ok || name == "" {
		return t.CreateErrorResponse("name is required")
	}

	scheme, _ := arguments["scheme"].(string)
	if scheme == "" {
		scheme = "internet-facing"
	}

	lbType, _ := arguments["type"].(string)
	if lbType == "" {
		lbType = "application"
	}

	// Use the ALB specialized adapter to create load balancer
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "create-load-balancer", arguments)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create load balancer: %v", err))
	}

	message := fmt.Sprintf("Load balancer %s created successfully", name)
	data := map[string]interface{}{
		"name":           name,
		"scheme":         scheme,
		"type":           lbType,
		"loadBalancer":   result,
		"loadBalancerId": result.ID,
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateTargetGroupTool implements MCPTool for creating target groups
type CreateTargetGroupTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreateTargetGroupTool creates a new target group creation tool
func NewCreateTargetGroupTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name for the target group",
			},
			"protocol": map[string]interface{}{
				"type":        "string",
				"description": "The protocol (HTTP, HTTPS, TCP)",
				"default":     "HTTP",
			},
			"port": map[string]interface{}{
				"type":        "integer",
				"description": "The port number",
				"default":     80,
			},
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "The VPC ID",
			},
			"targetType": map[string]interface{}{
				"type":        "string",
				"description": "The target type (instance, ip, lambda)",
				"default":     "instance",
			},
		},
		"required": []string{"name", "vpcId"},
	}

	return &CreateTargetGroupTool{
		BaseTool: &BaseTool{
			name:        "create-target-group",
			description: "Create a new target group",
			inputSchema: inputSchema,
			logger:      logger,
		},
		adapter: adapters.NewALBSpecializedAdapter(awsClient, logger),
	}
}

func (t *CreateTargetGroupTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, ok := arguments["name"].(string)
	if !ok || name == "" {
		return t.CreateErrorResponse("name is required")
	}

	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return t.CreateErrorResponse("vpcId is required")
	}

	protocol, _ := arguments["protocol"].(string)
	if protocol == "" {
		protocol = "HTTP"
	}

	port := 80
	if portArg, exists := arguments["port"]; exists {
		if portFloat, ok := portArg.(float64); ok {
			port = int(portFloat)
		}
	}

	targetType, _ := arguments["targetType"].(string)
	if targetType == "" {
		targetType = "instance"
	}

	// Use the ALB specialized adapter to create target group
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "create-target-group", arguments)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create target group: %v", err))
	}

	message := fmt.Sprintf("Target group %s created successfully", name)
	data := map[string]interface{}{
		"name":          name,
		"protocol":      protocol,
		"port":          port,
		"vpcId":         vpcID,
		"targetType":    targetType,
		"targetGroup":   result,
		"targetGroupId": result.ID,
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateListenerTool implements MCPTool for creating listeners
type CreateListenerTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreateListenerTool creates a new listener creation tool
func NewCreateListenerTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"loadBalancerArn": map[string]interface{}{
				"type":        "string",
				"description": "The ARN of the load balancer",
			},
			"protocol": map[string]interface{}{
				"type":        "string",
				"description": "The protocol (HTTP, HTTPS)",
				"default":     "HTTP",
			},
			"port": map[string]interface{}{
				"type":        "integer",
				"description": "The port number",
				"default":     80,
			},
			"targetGroupArn": map[string]interface{}{
				"type":        "string",
				"description": "The ARN of the target group",
			},
		},
		"required": []string{"loadBalancerArn", "targetGroupArn"},
	}

	return &CreateListenerTool{
		BaseTool: &BaseTool{
			name:        "create-listener",
			description: "Create a new listener for a load balancer",
			inputSchema: inputSchema,
			logger:      logger,
		},
		adapter: adapters.NewALBSpecializedAdapter(awsClient, logger),
	}
}

func (t *CreateListenerTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	loadBalancerArn, ok := arguments["loadBalancerArn"].(string)
	if !ok || loadBalancerArn == "" {
		return t.CreateErrorResponse("loadBalancerArn is required")
	}

	targetGroupArn, ok := arguments["targetGroupArn"].(string)
	if !ok || targetGroupArn == "" {
		return t.CreateErrorResponse("targetGroupArn is required")
	}

	protocol, _ := arguments["protocol"].(string)
	if protocol == "" {
		protocol = "HTTP"
	}

	port := 80
	if portArg, exists := arguments["port"]; exists {
		if portFloat, ok := portArg.(float64); ok {
			port = int(portFloat)
		}
	}

	// Use the ALB specialized adapter to create listener
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "create-listener", arguments)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create listener: %v", err))
	}

	message := "Listener created successfully for load balancer"
	data := map[string]interface{}{
		"loadBalancerArn": loadBalancerArn,
		"targetGroupArn":  targetGroupArn,
		"protocol":        protocol,
		"port":            port,
		"listener":        result,
		"listenerId":      result.ID,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListLoadBalancersTool implements MCPTool for listing load balancers
type ListLoadBalancersTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewListLoadBalancersTool creates a new load balancer listing tool
func NewListLoadBalancersTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-load-balancers",
		"List all load balancers",
		"alb",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"List all load balancers",
		map[string]interface{}{},
		"Retrieved 3 load balancers",
	)

	adapter := adapters.NewALBAdapter(awsClient, logger)

	return &ListLoadBalancersTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}

}

// Execute lists all Load Balancers
func (t *ListLoadBalancersTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Listing Load Balancers...")

	// List all Load Balancers using the adapter
	loadBalancers, err := t.adapter.List(ctx)
	if err != nil {
		t.logger.Error("Failed to list Load Balancers", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list Load Balancers: %v", err))
	}

	message := fmt.Sprintf("Successfully retrieved %d Load Balancers", len(loadBalancers))
	data := map[string]interface{}{
		"loadBalancers": loadBalancers,
		"count":         len(loadBalancers),
	}

	return t.CreateSuccessResponse(message, data)
}

// ListTargetGroupsTool implements MCPTool for listing target groups
type ListTargetGroupsTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewListTargetGroupsTool creates a new target group listing tool
func NewListTargetGroupsTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-target-groups",
		"List all target groups",
		"alb",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"List all target groups",
		map[string]interface{}{},
		"Retrieved 2 target groups",
	)

	// Use ALB specialized adapter for Target Group operations
	adapter := adapters.NewALBSpecializedAdapter(awsClient, logger)

	return &ListTargetGroupsTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *ListTargetGroupsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Listing Target Groups...")

	// Use the ALB specialized adapter for target group operations
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "list-target-groups", map[string]interface{}{})
	if err != nil {
		t.logger.Error("Failed to list Target Groups", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list Target Groups: %v", err))
	}

	// Extract target groups from adapter result Details
	var targetGroups []interface{}
	var count int
	if result.Details != nil {
		if tgList, exists := result.Details["targetGroups"]; exists {
			targetGroups, _ = tgList.([]interface{})
		}
		if countVal, exists := result.Details["count"]; exists {
			count, _ = countVal.(int)
		}
	}

	message := fmt.Sprintf("Successfully retrieved %d Target Groups", count)
	data := map[string]interface{}{
		"targetGroups": targetGroups,
		"count":        count,
	}

	return t.CreateSuccessResponse(message, data)
}

// RegisterTargetsTool implements MCPTool for registering targets with a target group
type RegisterTargetsTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewRegisterTargetsTool creates a new target registration tool
func NewRegisterTargetsTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"targetGroupArn": map[string]interface{}{
				"type":        "string",
				"description": "The ARN of the target group",
			},
			"targetIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of target IDs (instance IDs, IP addresses, etc.)",
			},
		},
		"required": []interface{}{"targetGroupArn", "targetIds"},
	}

	return &RegisterTargetsTool{
		BaseTool: &BaseTool{
			name:        "register-targets",
			description: "Register targets with a load balancer target group",
			inputSchema: inputSchema,
			logger:      logger,
		},
		adapter: adapters.NewALBSpecializedAdapter(awsClient, logger),
	}
}

func (t *RegisterTargetsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	targetGroupArn, ok := arguments["targetGroupArn"].(string)
	if !ok {
		return t.CreateErrorResponse("targetGroupArn is required and must be a string")
	}

	targetIdsInterface, ok := arguments["targetIds"].([]interface{})
	if !ok {
		return t.CreateErrorResponse("targetIds is required and must be an array")
	}

	// Convert interface slice to string slice
	targetIds := make([]string, len(targetIdsInterface))
	for i, id := range targetIdsInterface {
		if str, ok := id.(string); ok {
			targetIds[i] = str
		} else {
			return t.CreateErrorResponse(fmt.Sprintf("target ID at index %d must be a string", i))
		}
	}

	// Use the ALB specialized adapter to register targets
	params := map[string]interface{}{
		"targetGroupArn": targetGroupArn,
		"instanceIds":    targetIds,
	}

	result, err := t.adapter.ExecuteSpecialOperation(ctx, "register-targets", params)
	if err != nil {
		t.logger.WithError(err).Error("Failed to register targets")
		return t.CreateErrorResponse(fmt.Sprintf("Failed to register targets: %v", err))
	}

	message := fmt.Sprintf("Successfully registered %d targets with target group %s", len(targetIds), targetGroupArn)
	data := map[string]interface{}{
		"targetGroupArn": targetGroupArn,
		"targetIds":      targetIds,
		"status":         "registered",
		"result":         result,
	}

	return t.CreateSuccessResponse(message, data)
}

// DeregisterTargetsTool implements MCPTool for deregistering targets from a target group
type DeregisterTargetsTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewDeregisterTargetsTool creates a new target deregistration tool
func NewDeregisterTargetsTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"targetGroupArn": map[string]interface{}{
				"type":        "string",
				"description": "The ARN of the target group",
			},
			"targetIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of target IDs (instance IDs, IP addresses, etc.)",
			},
		},
		"required": []interface{}{"targetGroupArn", "targetIds"},
	}

	return &DeregisterTargetsTool{
		BaseTool: &BaseTool{
			name:        "deregister-targets",
			description: "Deregister targets from a load balancer target group",
			inputSchema: inputSchema,
			logger:      logger,
		},
		adapter: adapters.NewALBSpecializedAdapter(awsClient, logger),
	}
}

func (t *DeregisterTargetsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	targetGroupArn, ok := arguments["targetGroupArn"].(string)
	if !ok {
		return t.CreateErrorResponse("targetGroupArn is required and must be a string")
	}

	targetIdsInterface, ok := arguments["targetIds"].([]interface{})
	if !ok {
		return t.CreateErrorResponse("targetIds is required and must be an array")
	}

	// Convert interface slice to string slice
	targetIds := make([]string, len(targetIdsInterface))
	for i, id := range targetIdsInterface {
		if str, ok := id.(string); ok {
			targetIds[i] = str
		} else {
			return t.CreateErrorResponse(fmt.Sprintf("target ID at index %d must be a string", i))
		}
	}

	// Use the ALB specialized adapter to deregister targets
	params := map[string]interface{}{
		"targetGroupArn": targetGroupArn,
		"instanceIds":    targetIds,
	}

	result, err := t.adapter.ExecuteSpecialOperation(ctx, "deregister-targets", params)
	if err != nil {
		t.logger.WithError(err).Error("Failed to deregister targets")
		return t.CreateErrorResponse(fmt.Sprintf("Failed to deregister targets: %v", err))
	}

	message := fmt.Sprintf("Successfully deregistered %d targets from target group %s", len(targetIds), targetGroupArn)
	data := map[string]interface{}{
		"targetGroupArn": targetGroupArn,
		"targetIds":      targetIds,
		"status":         "deregistered",
		"result":         result,
	}

	return t.CreateSuccessResponse(message, data)
}
