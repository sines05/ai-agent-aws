package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/adapters"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// CreateLaunchTemplateTool implements MCPTool for creating launch templates
type CreateLaunchTemplateTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreateLaunchTemplateTool creates a new launch template creation tool
func NewCreateLaunchTemplateTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"templateName": map[string]interface{}{
				"type":        "string",
				"description": "The name for the launch template",
			},
			"imageId": map[string]interface{}{
				"type":        "string",
				"description": "The AMI ID to use",
			},
			"instanceType": map[string]interface{}{
				"type":        "string",
				"description": "The instance type",
			},
			"keyName": map[string]interface{}{
				"type":        "string",
				"description": "The key pair name",
			},
			"securityGroupIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of security group IDs",
			},
		},
		"required": []string{"templateName", "imageId", "instanceType"},
	}

	baseTool := NewBaseTool(
		"create-launch-template",
		"Create a new launch template for Auto Scaling Groups",
		"autoscaling",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Create launch template",
		map[string]interface{}{
			"templateName":     "web-server-template",
			"imageId":          "ami-0123456789abcdef0",
			"instanceType":     "t3.micro",
			"keyName":          "my-key",
			"securityGroupIds": []string{"sg-12345678"},
		},
		"Created launch template web-server-template",
	)

	// Cast to ASGSpecializedAdapter for specialized operations
	adapter := adapters.NewASGSpecializedAdapter(awsClient, logger)

	return &CreateLaunchTemplateTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *CreateLaunchTemplateTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	templateName, ok := arguments["templateName"].(string)
	if !ok || templateName == "" {
		return t.CreateErrorResponse("templateName is required")
	}

	imageId, ok := arguments["imageId"].(string)
	if !ok || imageId == "" {
		return t.CreateErrorResponse("imageId is required")
	}

	instanceType, ok := arguments["instanceType"].(string)
	if !ok || instanceType == "" {
		return t.CreateErrorResponse("instanceType is required")
	}

	keyName, _ := arguments["keyName"].(string)

	// Handle security group IDs
	var securityGroupIds []string
	if sgIds, ok := arguments["securityGroupIds"].([]interface{}); ok {
		for _, sgId := range sgIds {
			if id, ok := sgId.(string); ok {
				securityGroupIds = append(securityGroupIds, id)
			}
		}
	}

	// Create launch template parameters
	params := aws.CreateLaunchTemplateParams{
		LaunchTemplateName: templateName,
		ImageID:            imageId,
		InstanceType:       instanceType,
		KeyName:            keyName,
		SecurityGroupIDs:   securityGroupIds,
		Tags:               map[string]string{},
	}

	// Use the specialized adapter to create the launch template
	template, err := t.adapter.ExecuteSpecialOperation(ctx, "create-launch-template", params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create launch template: %s", err.Error()))
	}

	message := fmt.Sprintf("Created launch template %s (%s)", template.ID, templateName)
	data := map[string]interface{}{
		"templateId":       template.ID,
		"templateName":     templateName,
		"imageId":          imageId,
		"instanceType":     instanceType,
		"keyName":          keyName,
		"securityGroupIds": securityGroupIds,
		"resource":         template,
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateAutoScalingGroupTool implements MCPTool for creating auto scaling groups
type CreateAutoScalingGroupTool struct {
	*BaseTool
	adapter *adapters.ASGAdapter
}

// NewCreateAutoScalingGroupTool creates a new auto scaling group creation tool
func NewCreateAutoScalingGroupTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"asgName": map[string]interface{}{
				"type":        "string",
				"description": "The name for the auto scaling group",
			},
			"launchTemplateName": map[string]interface{}{
				"type":        "string",
				"description": "The launch template name to use",
			},
			"minSize": map[string]interface{}{
				"type":        "number",
				"description": "Minimum number of instances",
			},
			"maxSize": map[string]interface{}{
				"type":        "number",
				"description": "Maximum number of instances",
			},
			"desiredCapacity": map[string]interface{}{
				"type":        "number",
				"description": "Desired number of instances",
			},
			"subnetIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of subnet IDs",
			},
		},
		"required": []string{"asgName", "launchTemplateName", "minSize", "maxSize", "desiredCapacity", "subnetIds"},
	}

	baseTool := NewBaseTool(
		"create-auto-scaling-group",
		"Create a new Auto Scaling Group",
		"autoscaling",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Create auto scaling group",
		map[string]interface{}{
			"asgName":            "web-server-asg",
			"launchTemplateName": "web-server-template",
			"minSize":            1,
			"maxSize":            5,
			"desiredCapacity":    2,
			"subnetIds":          []string{"subnet-12345678", "subnet-87654321"},
		},
		"Created auto scaling group web-server-asg",
	)

	// Cast to ASGAdapter for type safety
	adapter := adapters.NewASGAdapter(awsClient, logger).(*adapters.ASGAdapter)

	return &CreateAutoScalingGroupTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *CreateAutoScalingGroupTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	asgName, ok := arguments["asgName"].(string)
	if !ok || asgName == "" {
		return t.CreateErrorResponse("asgName is required")
	}

	launchTemplateName, ok := arguments["launchTemplateName"].(string)
	if !ok || launchTemplateName == "" {
		return t.CreateErrorResponse("launchTemplateName is required")
	}

	minSize, ok := arguments["minSize"].(float64)
	if !ok {
		return t.CreateErrorResponse("minSize is required")
	}

	maxSize, ok := arguments["maxSize"].(float64)
	if !ok {
		return t.CreateErrorResponse("maxSize is required")
	}

	desiredCapacity, ok := arguments["desiredCapacity"].(float64)
	if !ok {
		return t.CreateErrorResponse("desiredCapacity is required")
	}

	// Handle subnet IDs
	var subnetIds []string
	if sIds, ok := arguments["subnetIds"].([]interface{}); ok {
		for _, sId := range sIds {
			if id, ok := sId.(string); ok {
				subnetIds = append(subnetIds, id)
			}
		}
	}

	if len(subnetIds) == 0 {
		return t.CreateErrorResponse("subnetIds is required and must not be empty")
	}

	// Create auto scaling group parameters
	params := aws.CreateAutoScalingGroupParams{
		AutoScalingGroupName: asgName,
		LaunchTemplateName:   launchTemplateName,
		MinSize:              int32(minSize),
		MaxSize:              int32(maxSize),
		DesiredCapacity:      int32(desiredCapacity),
		VPCZoneIdentifiers:   subnetIds,
		HealthCheckType:      "EC2", // Default
		Tags:                 map[string]string{},
	}

	// Use the adapter to create the auto scaling group
	asg, err := t.adapter.Create(ctx, params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create auto scaling group: %s", err.Error()))
	}

	message := fmt.Sprintf("Created auto scaling group %s with launch template %s", asg.ID, launchTemplateName)
	data := map[string]interface{}{
		"asgId":              asg.ID,
		"asgName":            asgName,
		"launchTemplateName": launchTemplateName,
		"minSize":            int32(minSize),
		"maxSize":            int32(maxSize),
		"desiredCapacity":    int32(desiredCapacity),
		"subnetIds":          subnetIds,
		"resource":           asg,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListAutoScalingGroupsTool implements MCPTool for listing auto scaling groups
type ListAutoScalingGroupsTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewListAutoScalingGroupsTool creates a new auto scaling group listing tool
func NewListAutoScalingGroupsTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-auto-scaling-groups",
		"List all auto scaling groups",
		"autoscaling",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"List all auto scaling groups",
		map[string]interface{}{},
		"Retrieved 4 auto scaling groups",
	)

	adapter := adapters.NewASGAdapter(awsClient, logger)

	return &ListAutoScalingGroupsTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *ListAutoScalingGroupsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Listing Auto Scaling Groups...")

	// List all Auto Scaling Groups using the adapter
	asgs, err := t.adapter.List(ctx)
	if err != nil {
		t.logger.Error("Failed to list Auto Scaling Groups", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list Auto Scaling Groups: %v", err))
	}

	message := fmt.Sprintf("Successfully retrieved %d Auto Scaling Groups", len(asgs))
	data := map[string]interface{}{
		"autoScalingGroups": asgs,
		"count":             len(asgs),
	}

	return t.CreateSuccessResponse(message, data)
}

// ListLaunchTemplatesTool implements MCPTool for listing launch templates
type ListLaunchTemplatesTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewListLaunchTemplatesTool creates a new launch template listing tool
func NewListLaunchTemplatesTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-launch-templates",
		"List all launch templates",
		"autoscaling",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"List all launch templates",
		map[string]interface{}{},
		"Retrieved 3 launch templates",
	)

	// Use EC2 specialized adapter for Launch Template operations
	adapter := adapters.NewEC2SpecializedAdapter(awsClient, logger)

	return &ListLaunchTemplatesTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *ListLaunchTemplatesTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Listing Launch Templates...")

	// Use the EC2 specialized adapter to list Launch Templates
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "list-launch-templates", nil)
	if err != nil {
		t.logger.Error("Failed to list Launch Templates", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list Launch Templates: %v", err))
	}

	// Extract the templates from the result
	count, _ := result.Details["count"].(int)
	templates, _ := result.Details["templates"].([]*types.AWSResource)

	message := fmt.Sprintf("Successfully retrieved %d Launch Templates", count)
	data := map[string]interface{}{
		"launchTemplates": templates,
		"count":           count,
	}

	return t.CreateSuccessResponse(message, data)
}

// UpdateAutoScalingGroupTool implements MCPTool for updating ASG capacity
type UpdateAutoScalingGroupTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
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
		BaseTool: baseTool,
		adapter:  adapters.NewASGAdapter(awsClient, logger),
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

	// Prepare parameters for adapter update
	updateParams := map[string]interface{}{
		"desiredCapacity": desiredCapacity,
	}

	// Use the adapter to update the ASG
	updatedASG, err := t.adapter.Update(ctx, asgName, updateParams)
	if err != nil {
		t.logger.WithError(err).Error("Failed to update Auto Scaling Group")
		return t.CreateErrorResponse(fmt.Sprintf("Failed to update ASG: %v", err))
	}

	message := fmt.Sprintf("Successfully updated Auto Scaling Group %s to desired capacity %d", asgName, desiredCapacity)
	data := map[string]interface{}{
		"asgName":         asgName,
		"desiredCapacity": desiredCapacity,
		"status":          "updated",
		"resource":        updatedASG,
	}

	return t.CreateSuccessResponse(message, data)
}

// DeleteAutoScalingGroupTool implements MCPTool for deleting ASGs
type DeleteAutoScalingGroupTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
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
		BaseTool: baseTool,
		adapter:  adapters.NewASGAdapter(awsClient, logger),
	}
}

func (t *DeleteAutoScalingGroupTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	asgName, ok := arguments["asgName"].(string)
	if !ok {
		return t.CreateErrorResponse("asgName is required and must be a string")
	}

	forceDelete, _ := arguments["forceDelete"].(bool) // Default to false if not provided

	// Use the adapter to delete the ASG
	err := t.adapter.Delete(ctx, asgName)
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

// AttachASGToTargetGroupTool implements MCPTool for attaching ASG to target groups
type AttachASGToTargetGroupTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewAttachASGToTargetGroupTool creates a new ASG target group attachment tool
func NewAttachASGToTargetGroupTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"asgName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the Auto Scaling Group",
			},
			"targetGroupArns": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of target group ARNs to attach to the ASG",
			},
		},
		"required": []interface{}{"asgName", "targetGroupArns"},
	}

	baseTool := NewBaseTool(
		"attach-asg-to-target-group",
		"Attach an Auto Scaling Group to load balancer target groups",
		"autoscaling",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Attach ASG to target groups",
		map[string]interface{}{
			"asgName": "web-servers-asg",
			"targetGroupArns": []string{
				"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/web-targets/50dc6c495c0c9188",
			},
		},
		"Successfully attached ASG web-servers-asg to target group",
	)

	return &AttachASGToTargetGroupTool{
		BaseTool: baseTool,
		adapter:  adapters.NewASGSpecializedAdapter(awsClient, logger),
	}
}

func (t *AttachASGToTargetGroupTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	asgName, ok := arguments["asgName"].(string)
	if !ok || asgName == "" {
		return t.CreateErrorResponse("asgName is required")
	}

	// Handle target group ARNs
	var targetGroupArns []string
	if tgArns, ok := arguments["targetGroupArns"].([]interface{}); ok {
		for _, tgArn := range tgArns {
			if arn, ok := tgArn.(string); ok {
				targetGroupArns = append(targetGroupArns, arn)
			}
		}
	}

	if len(targetGroupArns) == 0 {
		return t.CreateErrorResponse("targetGroupArns is required and must not be empty")
	}

	// Prepare parameters for specialized adapter
	attachParams := map[string]interface{}{
		"autoScalingGroupName": asgName,
		"targetGroupArns":      targetGroupArns,
	}

	// Use the specialized adapter to attach ASG to target groups
	updatedASG, err := t.adapter.ExecuteSpecialOperation(ctx, "attach-target-groups", attachParams)
	if err != nil {
		t.logger.WithError(err).Error("Failed to attach ASG to target groups")
		return t.CreateErrorResponse(fmt.Sprintf("Failed to attach ASG to target groups: %v", err))
	}

	message := fmt.Sprintf("Successfully attached Auto Scaling Group %s to %d target group(s)", asgName, len(targetGroupArns))
	data := map[string]interface{}{
		"asgName":          asgName,
		"targetGroupArns":  targetGroupArns,
		"targetGroupCount": len(targetGroupArns),
		"status":           "attached",
		"resource":         updatedASG,
	}

	return t.CreateSuccessResponse(message, data)
}
