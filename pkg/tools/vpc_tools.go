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

// CreateVPCTool implements VPC creation using the VPC adapter
type CreateVPCTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewCreateVPCTool creates a new VPC creation tool
func NewCreateVPCTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"cidrBlock": map[string]interface{}{
				"type":        "string",
				"description": "CIDR block for the VPC (e.g., '10.0.0.0/16')",
			},
			"enableDnsHostnames": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable DNS hostnames for the VPC",
				"default":     true,
			},
			"enableDnsSupport": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable DNS support for the VPC",
				"default":     true,
			},
		},
		"required": []interface{}{"cidrBlock"},
	}

	baseTool := NewBaseTool(
		"create-vpc",
		"Create a new VPC (Virtual Private Cloud)",
		"networking",
		inputSchema,
		logger,
	)

	adapter := adapters.NewVPCAdapter(awsClient, logger)

	return &CreateVPCTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute creates a VPC using the adapter
func (t *CreateVPCTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract parameters
	cidrBlock, _ := arguments["cidrBlock"].(string)
	enableDnsHostnames := getBoolValue(arguments, "enableDnsHostnames", true)
	enableDnsSupport := getBoolValue(arguments, "enableDnsSupport", true)

	// Build AWS parameters
	createParams := aws.CreateVPCParams{
		CidrBlock:          cidrBlock,
		EnableDnsHostnames: enableDnsHostnames,
		EnableDnsSupport:   enableDnsSupport,
		Tags:               make(map[string]string),
	}

	// Create VPC using adapter
	vpc, err := t.adapter.Create(ctx, createParams)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to create VPC: %s", err.Error()),
				},
			},
		}, nil
	}

	// Return success result
	result := fmt.Sprintf("Successfully created VPC %s with CIDR block %s", vpc.ID, cidrBlock)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// ListVPCsTool implements VPC listing using the VPC adapter
type ListVPCsTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewListVPCsTool creates a new VPC listing tool
func NewListVPCsTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-vpcs",
		"List all VPCs in the region",
		"networking",
		inputSchema,
		logger,
	)

	adapter := adapters.NewVPCAdapter(awsClient, logger)

	return &ListVPCsTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute lists VPCs using the adapter
func (t *ListVPCsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// List VPCs using adapter
	vpcs, err := t.adapter.List(ctx)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to list VPCs: %s", err.Error()),
				},
			},
		}, nil
	}

	// Format results
	result := fmt.Sprintf("Found %d VPCs:\n", len(vpcs))
	for _, vpc := range vpcs {
		result += fmt.Sprintf("- VPC ID: %s, State: %s, Region: %s\n", vpc.ID, vpc.State, vpc.Region)
		if cidrBlock, ok := vpc.Details["cidrBlock"]; ok {
			result += fmt.Sprintf("  CIDR: %s\n", cidrBlock)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// CreateSubnetTool implements subnet creation using the Subnet adapter
type CreateSubnetTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewCreateSubnetTool creates a new subnet creation tool
func NewCreateSubnetTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "VPC ID where the subnet will be created",
			},
			"cidrBlock": map[string]interface{}{
				"type":        "string",
				"description": "CIDR block for the subnet (e.g., '10.0.1.0/24')",
			},
			"availabilityZone": map[string]interface{}{
				"type":        "string",
				"description": "Availability zone for the subnet",
			},
			"mapPublicIpOnLaunch": map[string]interface{}{
				"type":        "boolean",
				"description": "Auto-assign public IP on instance launch",
				"default":     false,
			},
		},
		"required": []interface{}{"vpcId", "cidrBlock", "availabilityZone"},
	}

	baseTool := NewBaseTool(
		"create-subnet",
		"Create a new subnet in a VPC",
		"networking",
		inputSchema,
		logger,
	)

	adapter := adapters.NewSubnetAdapter(awsClient, logger)

	return &CreateSubnetTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute creates a subnet using the adapter
func (t *CreateSubnetTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract parameters
	vpcId, _ := arguments["vpcId"].(string)
	cidrBlock, _ := arguments["cidrBlock"].(string)
	availabilityZone, _ := arguments["availabilityZone"].(string)
	mapPublicIpOnLaunch := getBoolValue(arguments, "mapPublicIpOnLaunch", false)

	// Build AWS parameters
	createParams := aws.CreateSubnetParams{
		VpcID:               vpcId,
		CidrBlock:           cidrBlock,
		AvailabilityZone:    availabilityZone,
		MapPublicIpOnLaunch: mapPublicIpOnLaunch,
		Tags:                make(map[string]string),
	}

	// Create subnet using adapter
	subnet, err := t.adapter.Create(ctx, createParams)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to create subnet: %s", err.Error()),
				},
			},
		}, nil
	}

	// Return success result
	result := fmt.Sprintf("Successfully created subnet %s in VPC %s with CIDR block %s",
		subnet.ID, vpcId, cidrBlock)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// Helper function for boolean values
func getBoolValue(params map[string]interface{}, key string, defaultValue bool) bool {
	if value, ok := params[key].(bool); ok {
		return value
	}
	return defaultValue
}
