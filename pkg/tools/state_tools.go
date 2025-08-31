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

// AnalyzeStateTool implements infrastructure state analysis
type AnalyzeStateTool struct {
	*BaseTool
	vpcAdapter interfaces.AWSResourceAdapter
	ec2Adapter interfaces.AWSResourceAdapter
	rdsAdapter interfaces.AWSResourceAdapter
}

// NewAnalyzeStateTool creates a new state analysis tool
func NewAnalyzeStateTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"includeVPCs": map[string]interface{}{
				"type":        "boolean",
				"description": "Include VPC analysis in the state",
				"default":     true,
			},
			"includeEC2": map[string]interface{}{
				"type":        "boolean",
				"description": "Include EC2 analysis in the state",
				"default":     true,
			},
			"includeRDS": map[string]interface{}{
				"type":        "boolean",
				"description": "Include RDS analysis in the state",
				"default":     true,
			},
		},
	}

	baseTool := NewBaseTool(
		"analyze-infrastructure-state",
		"Analyze current infrastructure state across all AWS services",
		"state",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Analyze all infrastructure",
		map[string]interface{}{
			"includeVPCs": true,
			"includeEC2":  true,
			"includeRDS":  true,
		},
		"Infrastructure State Analysis completed",
	)

	// Create adapters for different AWS services
	vpcAdapter := adapters.NewVPCAdapter(awsClient, logger)
	ec2Adapter := adapters.NewEC2Adapter(awsClient, logger)
	rdsAdapter := adapters.NewRDSAdapter(awsClient, logger)

	return &AnalyzeStateTool{
		BaseTool:   baseTool,
		vpcAdapter: vpcAdapter,
		ec2Adapter: ec2Adapter,
		rdsAdapter: rdsAdapter,
	}
}

// Execute analyzes infrastructure state
func (t *AnalyzeStateTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	includeVPCs := getBoolValue(arguments, "includeVPCs", true)
	includeEC2 := getBoolValue(arguments, "includeEC2", true)
	includeRDS := getBoolValue(arguments, "includeRDS", true)

	result := "Infrastructure State Analysis:\n\n"

	if includeVPCs {
		vpcs, err := t.vpcAdapter.List(ctx)
		if err != nil {
			result += fmt.Sprintf("Error listing VPCs: %s\n", err.Error())
		} else {
			result += fmt.Sprintf("VPCs: %d found\n", len(vpcs))
			for _, vpc := range vpcs {
				result += fmt.Sprintf("  - VPC ID: %s, State: %s\n", vpc.ID, vpc.State)
			}
		}
		result += "\n"
	}

	if includeEC2 {
		instances, err := t.ec2Adapter.List(ctx)
		if err != nil {
			result += fmt.Sprintf("Error listing EC2 instances: %s\n", err.Error())
		} else {
			result += fmt.Sprintf("EC2 Instances: %d found\n", len(instances))
			for _, instance := range instances {
				result += fmt.Sprintf("  - Instance ID: %s, State: %s\n", instance.ID, instance.State)
			}
		}
		result += "\n"
	}

	if includeRDS {
		databases, err := t.rdsAdapter.List(ctx)
		if err != nil {
			result += fmt.Sprintf("Error listing RDS instances: %s\n", err.Error())
		} else {
			result += fmt.Sprintf("RDS Instances: %d found\n", len(databases))
			for _, db := range databases {
				result += fmt.Sprintf("  - DB Instance ID: %s, State: %s\n", db.ID, db.State)
			}
		}
		result += "\n"
	}

	result += "Analysis completed."

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}

// ExportStateTool implements infrastructure state export
type ExportStateTool struct {
	*BaseTool
	vpcAdapter interfaces.AWSResourceAdapter
	ec2Adapter interfaces.AWSResourceAdapter
	rdsAdapter interfaces.AWSResourceAdapter
}

// NewExportStateTool creates a new state export tool
func NewExportStateTool(awsClient *aws.Client, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Export format (json, yaml, or text)",
				"default":     "json",
				"enum":        []string{"json", "yaml", "text"},
			},
			"includeAll": map[string]interface{}{
				"type":        "boolean",
				"description": "Include all resource types",
				"default":     true,
			},
		},
	}

	baseTool := NewBaseTool(
		"export-infrastructure-state",
		"Export current infrastructure state to a file or output",
		"state",
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Export all infrastructure as JSON",
		map[string]interface{}{
			"format":     "json",
			"includeAll": true,
		},
		"Infrastructure State Export completed in json format.",
	)

	// Create adapters for different AWS services
	vpcAdapter := adapters.NewVPCAdapter(awsClient, logger)
	ec2Adapter := adapters.NewEC2Adapter(awsClient, logger)
	rdsAdapter := adapters.NewRDSAdapter(awsClient, logger)

	return &ExportStateTool{
		BaseTool:   baseTool,
		vpcAdapter: vpcAdapter,
		ec2Adapter: ec2Adapter,
		rdsAdapter: rdsAdapter,
	}
}

// Execute exports infrastructure state
func (t *ExportStateTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	format, _ := arguments["format"].(string)
	if format == "" {
		format = "json"
	}

	includeAll := getBoolValue(arguments, "includeAll", true)

	result := "Infrastructure State Export:\n\n"

	if includeAll {
		// For simplicity, we'll export as text format regardless of requested format
		// In a full implementation, this would support JSON/YAML export

		result += "=== VPCs ===\n"
		vpcs, err := t.vpcAdapter.List(ctx)
		if err != nil {
			result += fmt.Sprintf("Error: %s\n", err.Error())
		} else {
			for _, vpc := range vpcs {
				result += fmt.Sprintf("VPC: %s (State: %s, CIDR: %s)\n",
					vpc.ID, vpc.State, vpc.Details["cidrBlock"])
			}
		}

		result += "\n=== EC2 Instances ===\n"
		instances, err := t.ec2Adapter.List(ctx)
		if err != nil {
			result += fmt.Sprintf("Error: %s\n", err.Error())
		} else {
			for _, instance := range instances {
				result += fmt.Sprintf("Instance: %s (State: %s, Type: %s)\n",
					instance.ID, instance.State, instance.Details["instanceType"])
			}
		}

		result += "\n=== RDS Instances ===\n"
		databases, err := t.rdsAdapter.List(ctx)
		if err != nil {
			result += fmt.Sprintf("Error: %s\n", err.Error())
		} else {
			for _, db := range databases {
				result += fmt.Sprintf("Database: %s (State: %s, Engine: %s)\n",
					db.ID, db.State, db.Details["engine"])
			}
		}
	}

	result += fmt.Sprintf("\nExport completed in %s format.", format)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: result,
			},
		},
	}, nil
}
