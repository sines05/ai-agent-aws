package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// UpdateAutoScalingGroupTool implements MCPTool for updating ASG capacity
type UpdateAutoScalingGroupTool struct {
	*BaseTool
	awsClient *aws.Client
}

// NewUpdateAutoScalingGroupTool creates a new ASG update tool
func NewUpdateAutoScalingGroupTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"asgName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the Auto Scaling Group",
			},
			"desiredCapacity": map[string]interface{}{
				"type":        "integer",
				"description": "The desired capacity for the ASG",
			},
		},
		"required": []interface{}{"asgName", "desiredCapacity"},
	}

	baseTool := NewBaseTool(
		"update-auto-scaling-group",
		"Update the desired capacity of an Auto Scaling Group",
		"autoscaling",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Scale ASG",
		map[string]interface{}{
			"asgName":         "web-servers-asg",
			"desiredCapacity": 3,
		},
		"Updated ASG web-servers-asg to desired capacity of 3",
	)

	return &UpdateAutoScalingGroupTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

func (t *UpdateAutoScalingGroupTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	asgName, ok := arguments["asgName"].(string)
	if !ok {
		return t.CreateErrorResponse("asgName is required and must be a string")
	}

	desiredCapacityFloat, ok := arguments["desiredCapacity"].(float64)
	if !ok {
		return t.CreateErrorResponse("desiredCapacity is required and must be a number")
	}
	desiredCapacity := int32(desiredCapacityFloat)

	err := t.awsClient.UpdateAutoScalingGroup(ctx, asgName, desiredCapacity)
	if err != nil {
		t.logger.WithError(err).Error("Failed to update Auto Scaling Group")
		return t.CreateErrorResponse(fmt.Sprintf("Failed to update ASG: %v", err))
	}

	message := fmt.Sprintf("Successfully updated Auto Scaling Group %s to desired capacity %d", asgName, desiredCapacity)
	data := map[string]interface{}{
		"asgName":         asgName,
		"desiredCapacity": desiredCapacity,
		"status":          "updated",
	}

	return t.CreateSuccessResponse(message, data)
}

// DeleteAutoScalingGroupTool implements MCPTool for deleting ASGs
type DeleteAutoScalingGroupTool struct {
	*BaseTool
	awsClient *aws.Client
}

// NewDeleteAutoScalingGroupTool creates a new ASG deletion tool
func NewDeleteAutoScalingGroupTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"asgName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the Auto Scaling Group",
			},
			"forceDelete": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to force delete the ASG (terminate instances)",
				"default":     false,
			},
		},
		"required": []interface{}{"asgName"},
	}

	baseTool := NewBaseTool(
		"delete-auto-scaling-group",
		"Delete an Auto Scaling Group",
		"autoscaling",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Delete ASG",
		map[string]interface{}{
			"asgName":     "old-web-servers-asg",
			"forceDelete": true,
		},
		"Deleted Auto Scaling Group old-web-servers-asg",
	)

	return &DeleteAutoScalingGroupTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

func (t *DeleteAutoScalingGroupTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	asgName, ok := arguments["asgName"].(string)
	if !ok {
		return t.CreateErrorResponse("asgName is required and must be a string")
	}

	forceDelete, _ := arguments["forceDelete"].(bool) // Default to false if not provided

	err := t.awsClient.DeleteAutoScalingGroup(ctx, asgName, forceDelete)
	if err != nil {
		t.logger.WithError(err).Error("Failed to delete Auto Scaling Group")
		return t.CreateErrorResponse(fmt.Sprintf("Failed to delete ASG: %v", err))
	}

	message := fmt.Sprintf("Successfully deleted Auto Scaling Group %s", asgName)
	data := map[string]interface{}{
		"asgName":     asgName,
		"forceDelete": forceDelete,
		"status":      "deleted",
	}

	return t.CreateSuccessResponse(message, data)
}
