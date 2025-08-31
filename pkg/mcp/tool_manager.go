package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/tools"
)

// ToolManager manages the integration between adapter-based tools and MCP server
type ToolManager struct {
	registry  interfaces.ToolRegistry
	awsClient *aws.Client
	logger    *logging.Logger
	server    *Server // Reference to server for state-aware tools
}

// NewToolManager creates a new tool manager
func NewToolManager(awsClient *aws.Client, logger *logging.Logger, server *Server) *ToolManager {
	registry := tools.NewToolRegistry(logger)

	tm := &ToolManager{
		registry:  registry,
		awsClient: awsClient,
		logger:    logger,
		server:    server,
	}

	// Initialize and register all tools
	tm.initializeTools()

	return tm
}

// initializeTools creates and registers all adapter-based tools
func (tm *ToolManager) initializeTools() {
	// EC2 Tools
	tm.registerEC2Tools()

	// Networking Tools
	tm.registerNetworkingTools()

	// Security Group Tools
	tm.registerSecurityGroupTools()

	// ASG Tools
	tm.registerASGTools()

	// ALB Tools (TODO: implement when ALB adapter is ready)
	tm.registerALBTools()

	// RDS Tools (TODO: implement when RDS adapter is ready)
	tm.registerRDSTools()

	// State-Aware Tools
	tm.registerStateAwareTools()
}

// registerEC2Tools registers all EC2-related tools
func (tm *ToolManager) registerEC2Tools() {
	// Create EC2 Instance tool
	createInstanceTool := tools.NewCreateEC2InstanceTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createInstanceTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create EC2 instance tool")
	}

	// Start EC2 Instance tool
	startInstanceTool := tools.NewStartEC2InstanceTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(startInstanceTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register start EC2 instance tool")
	}

	// Stop EC2 Instance tool
	stopInstanceTool := tools.NewStopEC2InstanceTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(stopInstanceTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register stop EC2 instance tool")
	}

	// Terminate EC2 Instance tool
	terminateInstanceTool := tools.NewTerminateEC2InstanceTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(terminateInstanceTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register terminate EC2 instance tool")
	}

	// Create AMI from Instance tool
	createAMITool := tools.NewCreateAMIFromInstanceTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createAMITool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create AMI from instance tool")
	}

	// AMI Discovery Tools
	getLatestAmazonLinuxAMITool := tools.NewGetLatestAmazonLinuxAMITool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(getLatestAmazonLinuxAMITool); err != nil {
		tm.logger.WithError(err).Error("Failed to register get latest Amazon Linux AMI tool")
	}

	getLatestUbuntuAMITool := tools.NewGetLatestUbuntuAMITool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(getLatestUbuntuAMITool); err != nil {
		tm.logger.WithError(err).Error("Failed to register get latest Ubuntu AMI tool")
	}

	getLatestWindowsAMITool := tools.NewGetLatestWindowsAMITool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(getLatestWindowsAMITool); err != nil {
		tm.logger.WithError(err).Error("Failed to register get latest Windows AMI tool")
	}
}

// registerNetworkingTools registers all networking-related tools
func (tm *ToolManager) registerNetworkingTools() {
	// VPC tools
	createVPCTool := tools.NewCreateVPCTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createVPCTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create VPC tool")
	}

	createIGWTool := tools.NewCreateInternetGatewayTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createIGWTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create internet gateway tool")
	}

	createNATTool := tools.NewCreateNATGatewayTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createNATTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create NAT gateway tool")
	}

	createPublicRTTool := tools.NewCreatePublicRouteTableTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createPublicRTTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create public route table tool")
	}

	createPrivateRTTool := tools.NewCreatePrivateRouteTableTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createPrivateRTTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create private route table tool")
	}

	associateRTTool := tools.NewAssociateRouteTableTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(associateRTTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register associate route table tool")
	}

	// Subnet tools
	createSubnetTool := tools.NewCreateSubnetTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createSubnetTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create subnet tool")
	}

	createPrivateSubnetTool := tools.NewCreatePrivateSubnetTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createPrivateSubnetTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create private subnet tool")
	}

	createPublicSubnetTool := tools.NewCreatePublicSubnetTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createPublicSubnetTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create public subnet tool")
	}
}

// registerSecurityGroupTools registers all security group tools
func (tm *ToolManager) registerSecurityGroupTools() {
	// List Security Groups tool
	listSGTool := tools.NewListSecurityGroupsTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(listSGTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register list security groups tool")
	}

	// Create Security Group tool
	createSGTool := tools.NewCreateSecurityGroupTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createSGTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create security group tool")
	}

	// Add Ingress Rule tool
	addIngressTool := tools.NewAddSecurityGroupIngressRuleTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(addIngressTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register add security group ingress rule tool")
	}

	// Add Egress Rule tool
	addEgressTool := tools.NewAddSecurityGroupEgressRuleTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(addEgressTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register add security group egress rule tool")
	}

	// Delete Security Group tool
	deleteSGTool := tools.NewDeleteSecurityGroupTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(deleteSGTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register delete security group tool")
	}
}

// registerASGTools registers all Auto Scaling Group tools
func (tm *ToolManager) registerASGTools() {
	// Create Launch Template tool
	createLTTool := tools.NewCreateLaunchTemplateTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createLTTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create launch template tool")
	}

	// Create Auto Scaling Group tool
	createASGTool := tools.NewCreateAutoScalingGroupTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createASGTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create auto scaling group tool")
	}
}

// registerALBTools registers all Application Load Balancer tools
func (tm *ToolManager) registerALBTools() {
	// Create Load Balancer tool
	createLBTool := tools.NewCreateLoadBalancerTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createLBTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create load balancer tool")
	}

	// Create Target Group tool
	createTGTool := tools.NewCreateTargetGroupTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createTGTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create target group tool")
	}

	// Create Listener tool
	createListenerTool := tools.NewCreateListenerTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createListenerTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create listener tool")
	}

	// List Load Balancers tool
	listLBTool := tools.NewListLoadBalancersTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(listLBTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register list load balancers tool")
	}

	// List Target Groups tool
	listTGTool := tools.NewListTargetGroupsTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(listTGTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register list target groups tool")
	}

	// Register Targets tool
	registerTargetsTool := tools.NewRegisterTargetsTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(registerTargetsTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register targets registration tool")
	}

	// Deregister Targets tool
	deregisterTargetsTool := tools.NewDeregisterTargetsTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(deregisterTargetsTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register targets deregistration tool")
	}
}

// registerRDSTools registers all RDS tools
func (tm *ToolManager) registerRDSTools() {
	// Create DB Subnet Group tool
	createDBSubnetGroupTool := tools.NewCreateDBSubnetGroupTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createDBSubnetGroupTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create DB subnet group tool")
	}

	// Create DB Instance tool
	createDBInstanceTool := tools.NewCreateDBInstanceTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createDBInstanceTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create DB instance tool")
	}

	// Start DB Instance tool
	startDBInstanceTool := tools.NewStartDBInstanceTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(startDBInstanceTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register start DB instance tool")
	}

	// Stop DB Instance tool
	stopDBInstanceTool := tools.NewStopDBInstanceTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(stopDBInstanceTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register stop DB instance tool")
	}

	// Delete DB Instance tool
	deleteDBInstanceTool := tools.NewDeleteDBInstanceTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(deleteDBInstanceTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register delete DB instance tool")
	}

	// Create DB Snapshot tool
	createDBSnapshotTool := tools.NewCreateDBSnapshotTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(createDBSnapshotTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register create DB snapshot tool")
	}

	// List DB Instances tool
	listDBInstancesTool := tools.NewListDBInstancesTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(listDBInstancesTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register list DB instances tool")
	}

	// List DB Snapshots tool
	listDBSnapshotsTool := tools.NewListDBSnapshotsTool(tm.awsClient, tm.logger)
	if err := tm.registry.Register(listDBSnapshotsTool); err != nil {
		tm.logger.WithError(err).Error("Failed to register list DB snapshots tool")
	}
}

// registerStateAwareTools registers all state-aware tools that require server dependencies
func (tm *ToolManager) registerStateAwareTools() {
	if tm.server == nil {
		tm.logger.Warn("Server is nil, skipping state-aware tools registration")
		return
	}

	// Create state-aware tool dependencies from server components
	stateAwareDeps := tools.StateAwareToolDependencies{
		StateManager:     tm.server.StateManager,
		DiscoveryScanner: tm.server.DiscoveryScanner,
		GraphManager:     tm.server.GraphManager,
		GraphAnalyzer:    tm.server.GraphAnalyzer,
		ConflictResolver: tm.server.ConflictResolver,
		Config:           tm.server.Config,
	}

	// State-aware tool types to register
	stateAwareToolTypes := []string{
		"analyze-infrastructure-state-advanced",
		"visualize-dependency-graph",
		"detect-conflicts",
		"export-state-aware",
		"save-state",
		"add-resource-to-state",
		"plan-deployment",
	}

	// Create factory for state-aware tools
	factory := tools.NewToolFactory(tm.awsClient, tm.logger)

	// Register each state-aware tool
	for _, toolType := range stateAwareToolTypes {
		tool, err := factory.CreateTool(toolType, stateAwareDeps)
		if err != nil {
			tm.logger.WithError(err).WithField("toolType", toolType).Error("Failed to create state-aware tool")
			continue
		}

		if err := tm.registry.Register(tool); err != nil {
			tm.logger.WithError(err).WithField("toolType", toolType).Error("Failed to register state-aware tool")
			continue
		}

		tm.logger.WithField("toolType", toolType).Info("Successfully registered state-aware tool")
	}
}

// GetRegistry returns the tool registry
func (tm *ToolManager) GetRegistry() interfaces.ToolRegistry {
	return tm.registry
}

// ExecuteTool executes a tool by name with the given arguments
func (tm *ToolManager) ExecuteTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	tool, exists := tm.registry.GetTool(name)
	if !exists {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Tool '%s' not found", name)),
			},
		}, nil
	}

	// Execute the tool
	return tool.Execute(ctx, arguments)
}

// ListAvailableTools returns a list of all available tools
func (tm *ToolManager) ListAvailableTools() []interfaces.MCPTool {
	return tm.registry.ListTools()
}
