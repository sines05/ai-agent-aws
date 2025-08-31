package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// registerServerToolsModern integrates the modern tool manager with the existing MCP server
func (s *Server) registerServerToolsModern() {
	s.Logger.Info("Integrating modern tool manager with existing MCP server")

	// Create tool manager with server reference for state-aware tools
	toolManager := NewToolManager(s.AWSClient, s.Logger, s)

	// Get all modern tools
	modernTools := toolManager.ListAvailableTools()

	s.Logger.WithField("modernToolCount", len(modernTools)).Info("Registering modern tools alongside legacy tools")

	// Register each modern tool with the MCP server
	for _, tool := range modernTools {
		s.registerServerTool(tool, toolManager)
	}

	// State-aware tools are now registered automatically via the tool manager
}

// registerServerTool registers a single modern tool with the MCP server
func (s *Server) registerServerTool(tool interfaces.MCPTool, toolManager *ToolManager) {
	name := tool.Name()

	// Create a simple tool registration that delegates to the tool manager
	// For now, we'll use a simplified approach that works with the existing MCP patterns

	switch name {
	// EC2 Tools
	case "create-ec2-instance":
		s.mcpServer.AddTool(
			mcp.NewTool("create-ec2-instance-modern",
				mcp.WithDescription("Create a new EC2 instance (modern adapter-based implementation)"),
				mcp.WithString("imageId", mcp.Description("AMI ID to use for the instance"), mcp.Required()),
				mcp.WithString("instanceType", mcp.Description("EC2 instance type (e.g., t2.micro, t3.small)"), mcp.Required()),
				mcp.WithString("keyName", mcp.Description("Name of the key pair to use for SSH access")),
				mcp.WithString("securityGroupId", mcp.Description("Security group ID to assign to the instance")),
				mcp.WithString("subnetId", mcp.Description("Subnet ID where the instance should be launched")),
				mcp.WithString("name", mcp.Description("Name tag for the instance")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "start-ec2-instance":
		s.mcpServer.AddTool(
			mcp.NewTool("start-ec2-instance-modern",
				mcp.WithDescription("Start a stopped EC2 instance (modern adapter-based implementation)"),
				mcp.WithString("instanceId", mcp.Description("EC2 instance ID to start"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "stop-ec2-instance":
		s.mcpServer.AddTool(
			mcp.NewTool("stop-ec2-instance-modern",
				mcp.WithDescription("Stop a running EC2 instance (modern adapter-based implementation)"),
				mcp.WithString("instanceId", mcp.Description("EC2 instance ID to stop"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "terminate-ec2-instance":
		s.mcpServer.AddTool(
			mcp.NewTool("terminate-ec2-instance-modern",
				mcp.WithDescription("Terminate an EC2 instance (modern adapter-based implementation)"),
				mcp.WithString("instanceId", mcp.Description("EC2 instance ID to terminate"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-ami-from-instance":
		s.mcpServer.AddTool(
			mcp.NewTool("create-ami-from-instance-modern",
				mcp.WithDescription("Create an AMI from an EC2 instance (modern adapter-based implementation)"),
				mcp.WithString("instanceId", mcp.Description("EC2 instance ID to create AMI from"), mcp.Required()),
				mcp.WithString("name", mcp.Description("Name for the new AMI"), mcp.Required()),
				mcp.WithString("description", mcp.Description("Description for the new AMI")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "get-latest-amazon-linux-ami":
		s.mcpServer.AddTool(
			mcp.NewTool("get-latest-amazon-linux-ami-modern",
				mcp.WithDescription("Find the latest Amazon Linux 2 AMI ID in the current region"),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "get-latest-ubuntu-ami":
		s.mcpServer.AddTool(
			mcp.NewTool("get-latest-ubuntu-ami-modern",
				mcp.WithDescription("Find the latest Ubuntu LTS AMI ID in the current region"),
				mcp.WithString("architecture", mcp.Description("The architecture (x86_64, arm64, defaults to x86_64)")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "get-latest-windows-ami":
		s.mcpServer.AddTool(
			mcp.NewTool("get-latest-windows-ami-modern",
				mcp.WithDescription("Find the latest Windows Server AMI ID in the current region"),
				mcp.WithString("architecture", mcp.Description("The architecture (x86_64, arm64, defaults to x86_64)")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	// Networking Tools
	case "create-vpc":
		s.mcpServer.AddTool(
			mcp.NewTool("create-vpc-modern",
				mcp.WithDescription("Create a new VPC (modern adapter-based implementation)"),
				mcp.WithString("cidrBlock", mcp.Description("CIDR block for the VPC"), mcp.Required()),
				mcp.WithString("name", mcp.Description("Name tag for the VPC")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-subnet":
		s.mcpServer.AddTool(
			mcp.NewTool("create-subnet-modern",
				mcp.WithDescription("Create a new subnet (modern adapter-based implementation)"),
				mcp.WithString("vpcId", mcp.Description("VPC ID where the subnet will be created"), mcp.Required()),
				mcp.WithString("cidrBlock", mcp.Description("CIDR block for the subnet"), mcp.Required()),
				mcp.WithString("availabilityZone", mcp.Description("Availability zone for the subnet")),
				mcp.WithString("name", mcp.Description("Name tag for the subnet")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-internet-gateway":
		s.mcpServer.AddTool(
			mcp.NewTool("create-internet-gateway-modern",
				mcp.WithDescription("Create an Internet Gateway (modern adapter-based implementation)"),
				mcp.WithString("vpcId", mcp.Description("VPC ID to attach the Internet Gateway"), mcp.Required()),
				mcp.WithString("name", mcp.Description("Name tag for the Internet Gateway")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-private-subnet":
		s.mcpServer.AddTool(
			mcp.NewTool("create-private-subnet-modern",
				mcp.WithDescription("Create a private subnet (modern adapter-based implementation)"),
				mcp.WithString("vpcId", mcp.Description("VPC ID where the subnet will be created"), mcp.Required()),
				mcp.WithString("cidrBlock", mcp.Description("CIDR block for the subnet"), mcp.Required()),
				mcp.WithString("availabilityZone", mcp.Description("Availability zone for the subnet"), mcp.Required()),
				mcp.WithString("name", mcp.Description("Name tag for the subnet")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-public-subnet":
		s.mcpServer.AddTool(
			mcp.NewTool("create-public-subnet-modern",
				mcp.WithDescription("Create a public subnet (modern adapter-based implementation)"),
				mcp.WithString("vpcId", mcp.Description("VPC ID where the subnet will be created"), mcp.Required()),
				mcp.WithString("cidrBlock", mcp.Description("CIDR block for the subnet"), mcp.Required()),
				mcp.WithString("availabilityZone", mcp.Description("Availability zone for the subnet"), mcp.Required()),
				mcp.WithString("name", mcp.Description("Name tag for the subnet")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-nat-gateway":
		s.mcpServer.AddTool(
			mcp.NewTool("create-nat-gateway-modern",
				mcp.WithDescription("Create a NAT Gateway (modern adapter-based implementation)"),
				mcp.WithString("subnetId", mcp.Description("Public subnet ID where the NAT Gateway will be created"), mcp.Required()),
				mcp.WithString("name", mcp.Description("Name tag for the NAT Gateway")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-public-route-table":
		s.mcpServer.AddTool(
			mcp.NewTool("create-public-route-table-modern",
				mcp.WithDescription("Create a public route table (modern adapter-based implementation)"),
				mcp.WithString("vpcId", mcp.Description("VPC ID where the route table will be created"), mcp.Required()),
				mcp.WithString("internetGatewayId", mcp.Description("Internet Gateway ID for public route"), mcp.Required()),
				mcp.WithString("name", mcp.Description("Name tag for the route table")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-private-route-table":
		s.mcpServer.AddTool(
			mcp.NewTool("create-private-route-table-modern",
				mcp.WithDescription("Create a private route table (modern adapter-based implementation)"),
				mcp.WithString("vpcId", mcp.Description("VPC ID where the route table will be created"), mcp.Required()),
				mcp.WithString("natGatewayId", mcp.Description("NAT Gateway ID for private route"), mcp.Required()),
				mcp.WithString("name", mcp.Description("Name tag for the route table")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "associate-route-table":
		s.mcpServer.AddTool(
			mcp.NewTool("associate-route-table-modern",
				mcp.WithDescription("Associate route table with subnet (modern adapter-based implementation)"),
				mcp.WithString("routeTableId", mcp.Description("Route table ID to associate"), mcp.Required()),
				mcp.WithString("subnetId", mcp.Description("Subnet ID to associate with"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	// Security Group Tools
	case "list-security-groups":
		s.mcpServer.AddTool(
			mcp.NewTool("list-security-groups-modern",
				mcp.WithDescription("List all security groups (modern adapter-based implementation)"),
				mcp.WithString("vpcId", mcp.Description("VPC ID to filter security groups")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-security-group":
		s.mcpServer.AddTool(
			mcp.NewTool("create-security-group-modern",
				mcp.WithDescription("Create a new security group (modern adapter-based implementation)"),
				mcp.WithString("groupName", mcp.Description("Name of the security group"), mcp.Required()),
				mcp.WithString("description", mcp.Description("Description of the security group"), mcp.Required()),
				mcp.WithString("vpcId", mcp.Description("VPC ID where the security group will be created")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "add-security-group-ingress-rule":
		s.mcpServer.AddTool(
			mcp.NewTool("add-security-group-ingress-rule-modern",
				mcp.WithDescription("Add an ingress rule to security group (modern adapter-based implementation)"),
				mcp.WithString("groupId", mcp.Description("Security group ID"), mcp.Required()),
				mcp.WithString("protocol", mcp.Description("Protocol: tcp, udp, icmp, or -1 for all"), mcp.Required()),
				mcp.WithNumber("fromPort", mcp.Description("Start port (required for TCP/UDP)")),
				mcp.WithNumber("toPort", mcp.Description("End port (required for TCP/UDP)")),
				mcp.WithString("cidrBlocks", mcp.Description("Comma-separated CIDR blocks to allow")),
				mcp.WithString("sourceSG", mcp.Description("Source security group ID")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "add-security-group-egress-rule":
		s.mcpServer.AddTool(
			mcp.NewTool("add-security-group-egress-rule-modern",
				mcp.WithDescription("Add an egress rule to security group (modern adapter-based implementation)"),
				mcp.WithString("groupId", mcp.Description("Security group ID"), mcp.Required()),
				mcp.WithString("protocol", mcp.Description("Protocol: tcp, udp, icmp, or -1 for all"), mcp.Required()),
				mcp.WithNumber("fromPort", mcp.Description("Start port (required for TCP/UDP)")),
				mcp.WithNumber("toPort", mcp.Description("End port (required for TCP/UDP)")),
				mcp.WithString("cidrBlocks", mcp.Description("Comma-separated CIDR blocks to allow")),
				mcp.WithString("sourceSG", mcp.Description("Source security group ID")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "delete-security-group":
		s.mcpServer.AddTool(
			mcp.NewTool("delete-security-group-modern",
				mcp.WithDescription("Delete a security group (modern adapter-based implementation)"),
				mcp.WithString("groupId", mcp.Description("Security group ID to delete"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	// ASG Tools
	case "create-launch-template":
		s.mcpServer.AddTool(
			mcp.NewTool("create-launch-template-modern",
				mcp.WithDescription("Create a new launch template (modern adapter-based implementation)"),
				mcp.WithString("templateName", mcp.Description("Name of the launch template"), mcp.Required()),
				mcp.WithString("imageId", mcp.Description("AMI ID to use"), mcp.Required()),
				mcp.WithString("instanceType", mcp.Description("Instance type"), mcp.Required()),
				mcp.WithString("keyName", mcp.Description("Key pair name")),
				mcp.WithString("securityGroupIds", mcp.Description("Comma-separated security group IDs")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-auto-scaling-group":
		s.mcpServer.AddTool(
			mcp.NewTool("create-auto-scaling-group-modern",
				mcp.WithDescription("Create a new Auto Scaling Group (modern adapter-based implementation)"),
				mcp.WithString("autoScalingGroupName", mcp.Description("Name of the Auto Scaling Group"), mcp.Required()),
				mcp.WithString("launchTemplateId", mcp.Description("Launch template ID"), mcp.Required()),
				mcp.WithString("subnetIds", mcp.Description("Comma-separated subnet IDs"), mcp.Required()),
				mcp.WithNumber("minSize", mcp.Description("Minimum size of the group"), mcp.Required()),
				mcp.WithNumber("maxSize", mcp.Description("Maximum size of the group"), mcp.Required()),
				mcp.WithNumber("desiredCapacity", mcp.Description("Desired capacity of the group"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	// ALB Tools
	case "create-load-balancer":
		s.mcpServer.AddTool(
			mcp.NewTool("create-load-balancer-modern",
				mcp.WithDescription("Create a new Application Load Balancer (modern adapter-based implementation)"),
				mcp.WithString("name", mcp.Description("Name of the load balancer"), mcp.Required()),
				mcp.WithString("subnetIds", mcp.Description("Comma-separated subnet IDs"), mcp.Required()),
				mcp.WithString("securityGroupIds", mcp.Description("Comma-separated security group IDs")),
				mcp.WithString("scheme", mcp.Description("Load balancer scheme (internet-facing or internal)")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-target-group":
		s.mcpServer.AddTool(
			mcp.NewTool("create-target-group-modern",
				mcp.WithDescription("Create a new target group (modern adapter-based implementation)"),
				mcp.WithString("name", mcp.Description("Name of the target group"), mcp.Required()),
				mcp.WithString("protocol", mcp.Description("Protocol (HTTP, HTTPS, TCP, etc.)"), mcp.Required()),
				mcp.WithNumber("port", mcp.Description("Port number"), mcp.Required()),
				mcp.WithString("vpcId", mcp.Description("VPC ID"), mcp.Required()),
				mcp.WithString("targetType", mcp.Description("Target type (instance, ip, lambda)")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-listener":
		s.mcpServer.AddTool(
			mcp.NewTool("create-listener-modern",
				mcp.WithDescription("Create a listener for load balancer (modern adapter-based implementation)"),
				mcp.WithString("loadBalancerArn", mcp.Description("Load balancer ARN"), mcp.Required()),
				mcp.WithString("protocol", mcp.Description("Listener protocol (HTTP, HTTPS)"), mcp.Required()),
				mcp.WithNumber("port", mcp.Description("Listener port"), mcp.Required()),
				mcp.WithString("targetGroupArn", mcp.Description("Target group ARN for default action"), mcp.Required()),
				mcp.WithString("certificateArn", mcp.Description("SSL certificate ARN for HTTPS listeners")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "register-targets":
		s.mcpServer.AddTool(
			mcp.NewTool("register-targets-modern",
				mcp.WithDescription("Register targets with a load balancer target group"),
				mcp.WithString("targetGroupArn", mcp.Description("The ARN of the target group"), mcp.Required()),
				mcp.WithString("targetIds", mcp.Description("Comma-separated list of target IDs (instance IDs, IP addresses, etc.)"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "deregister-targets":
		s.mcpServer.AddTool(
			mcp.NewTool("deregister-targets-modern",
				mcp.WithDescription("Deregister targets from a load balancer target group"),
				mcp.WithString("targetGroupArn", mcp.Description("The ARN of the target group"), mcp.Required()),
				mcp.WithString("targetIds", mcp.Description("Comma-separated list of target IDs (instance IDs, IP addresses, etc.)"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	// RDS Tools
	case "create-db-subnet-group":
		s.mcpServer.AddTool(
			mcp.NewTool("create-db-subnet-group-modern",
				mcp.WithDescription("Create a database subnet group (modern adapter-based implementation)"),
				mcp.WithString("dbSubnetGroupName", mcp.Description("Name for the DB subnet group"), mcp.Required()),
				mcp.WithString("dbSubnetGroupDescription", mcp.Description("Description for the DB subnet group"), mcp.Required()),
				mcp.WithString("subnetIds", mcp.Description("Comma-separated subnet IDs for the DB subnet group"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	// RDS Tools
	case "create-db-instance":
		s.mcpServer.AddTool(
			mcp.NewTool("create-db-instance-modern",
				mcp.WithDescription("Create a new RDS database instance (modern adapter-based implementation)"),
				mcp.WithString("dbInstanceIdentifier", mcp.Description("Database instance identifier"), mcp.Required()),
				mcp.WithString("dbInstanceClass", mcp.Description("Database instance class"), mcp.Required()),
				mcp.WithString("engine", mcp.Description("Database engine"), mcp.Required()),
				mcp.WithString("masterUsername", mcp.Description("Master username"), mcp.Required()),
				mcp.WithString("masterUserPassword", mcp.Description("Master password"), mcp.Required()),
				mcp.WithNumber("allocatedStorage", mcp.Description("Allocated storage in GB"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "start-db-instance":
		s.mcpServer.AddTool(
			mcp.NewTool("start-db-instance-modern",
				mcp.WithDescription("Start a stopped RDS database instance (modern adapter-based implementation)"),
				mcp.WithString("dbInstanceIdentifier", mcp.Description("Database instance identifier"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "stop-db-instance":
		s.mcpServer.AddTool(
			mcp.NewTool("stop-db-instance-modern",
				mcp.WithDescription("Stop a running RDS database instance (modern adapter-based implementation)"),
				mcp.WithString("dbInstanceIdentifier", mcp.Description("Database instance identifier"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "delete-db-instance":
		s.mcpServer.AddTool(
			mcp.NewTool("delete-db-instance-modern",
				mcp.WithDescription("Delete an RDS database instance (modern adapter-based implementation)"),
				mcp.WithString("dbInstanceIdentifier", mcp.Description("Database instance identifier"), mcp.Required()),
				mcp.WithBoolean("skipFinalSnapshot", mcp.Description("Skip final snapshot creation"), mcp.Required()),
				mcp.WithString("finalSnapshotIdentifier", mcp.Description("Final snapshot identifier")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "create-db-snapshot":
		s.mcpServer.AddTool(
			mcp.NewTool("create-db-snapshot-modern",
				mcp.WithDescription("Create a snapshot of an RDS database instance (modern adapter-based implementation)"),
				mcp.WithString("dbSnapshotIdentifier", mcp.Description("Snapshot identifier"), mcp.Required()),
				mcp.WithString("dbInstanceIdentifier", mcp.Description("Database instance identifier"), mcp.Required()),
			),
			s.createModernToolHandler(name, toolManager),
		)

	// List Tools
	case "list-db-instances":
		s.mcpServer.AddTool(
			mcp.NewTool("list-db-instances-modern",
				mcp.WithDescription("List all RDS database instances (modern adapter-based implementation)"),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "list-db-snapshots":
		s.mcpServer.AddTool(
			mcp.NewTool("list-db-snapshots-modern",
				mcp.WithDescription("List all RDS database snapshots (modern adapter-based implementation)"),
			),
			s.createModernToolHandler(name, toolManager),
		)

	// State-aware tools
	case "analyze-infrastructure-state-advanced":
		s.mcpServer.AddTool(
			mcp.NewTool("analyze-infrastructure-state-advanced",
				mcp.WithDescription("Analyze current infrastructure state and detect drift with advanced capabilities"),
				mcp.WithBoolean("scan_live", mcp.Description("Whether to scan live infrastructure")),
				mcp.WithBoolean("include_drift", mcp.Description("Include drift detection")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "visualize-dependency-graph":
		s.mcpServer.AddTool(
			mcp.NewTool("visualize-dependency-graph",
				mcp.WithDescription("Generate dependency graph visualization with bottleneck analysis"),
				mcp.WithString("format", mcp.Description("Output format: text, mermaid")),
				mcp.WithBoolean("include_bottlenecks", mcp.Description("Include bottleneck analysis")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "detect-conflicts":
		s.mcpServer.AddTool(
			mcp.NewTool("detect-conflicts",
				mcp.WithDescription("Detect conflicts in infrastructure configuration"),
				mcp.WithBoolean("auto_resolve", mcp.Description("Automatically resolve conflicts where possible")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "export-state-aware":
		s.mcpServer.AddTool(
			mcp.NewTool("export-state-aware",
				mcp.WithDescription("Export comprehensive infrastructure state with dependencies and conflicts"),
				mcp.WithBoolean("include_discovered", mcp.Description("Include discovered (unmanaged) resources")),
				mcp.WithBoolean("include_dependencies", mcp.Description("Include dependency graph")),
				mcp.WithBoolean("include_conflicts", mcp.Description("Include conflict analysis")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "save-state":
		s.mcpServer.AddTool(
			mcp.NewTool("save-state",
				mcp.WithDescription("Save current infrastructure state to persistent storage"),
				mcp.WithBoolean("force", mcp.Description("Force save even if state hasn't changed")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "add-resource-to-state":
		s.mcpServer.AddTool(
			mcp.NewTool("add-resource-to-state",
				mcp.WithDescription("Add a resource to the managed infrastructure state"),
				mcp.WithString("resource_id", mcp.Description("Resource ID"), mcp.Required()),
				mcp.WithString("resource_name", mcp.Description("Resource name"), mcp.Required()),
				mcp.WithString("resource_type", mcp.Description("Resource type"), mcp.Required()),
				mcp.WithString("status", mcp.Description("Resource status")),
				mcp.WithObject("properties", mcp.Description("Resource properties")),
				mcp.WithArray("dependencies", mcp.Description("Resource dependencies")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	case "plan-deployment":
		s.mcpServer.AddTool(
			mcp.NewTool("plan-deployment",
				mcp.WithDescription("Generate deployment plan with dependency ordering"),
				mcp.WithBoolean("include_levels", mcp.Description("Include deployment levels for parallel execution")),
				mcp.WithArray("target_resources", mcp.Description("Specific resources to include in plan")),
			),
			s.createModernToolHandler(name, toolManager),
		)

	default:
		s.Logger.WithField("toolName", name).Debug("Skipping registration for modern tool (not explicitly mapped)")
	}
}

// createModernToolHandler creates a handler function that delegates to the tool manager
func (s *Server) createModernToolHandler(toolName string, toolManager *ToolManager) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					mcp.NewTextContent("Invalid arguments format"),
				},
			}, nil
		}

		s.Logger.WithField("toolName", toolName).WithField("arguments", arguments).Info("Executing modern tool via tool manager")
		return toolManager.ExecuteTool(ctx, toolName, arguments)
	}
}
