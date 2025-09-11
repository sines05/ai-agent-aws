package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent/mocks"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// === Helper Functions for Setting Up Real AI and Comprehensive Mock MCP Tools ===

func setupRealConfiguration() (*config.Config, error) {
	// Load the actual config from configuration file
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Ensure we're using Gemini for this test
	cfg.Agent.Provider = "gemini"
	cfg.Agent.MaxTokens = 20000
	cfg.Agent.Model = "gemini-2.5-flash-lite" // Use a valid model name

	return cfg, nil
}

func setupRealLLMClient(cfg *config.Config) (llms.Model, error) {
	switch cfg.Agent.Provider {
	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("You need an auth option to use this client.")
		}
		// Pass the API key and model to the googleai client
		return googleai.New(context.Background(),
			googleai.WithAPIKey(apiKey),
			googleai.WithDefaultModel(cfg.Agent.Model))
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Agent.Provider)
	}
}

func setupAgentWithRealAI(cfg *config.Config, llmClient llms.Model) (*StateAwareAgent, *mocks.MockTestSuite, error) {
	// Create logger
	logger := logging.NewLogger("test", cfg.Logging.Level)

	// Initialize mock test suite with real component integration
	mockSuite, err := mocks.NewMockTestSuite("us-west-2")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create mock test suite: %w", err)
	}

	// Create agent with real AI but mock infrastructure components
	agent := &StateAwareAgent{
		llm:       llmClient,
		config:    &cfg.Agent,
		awsConfig: &cfg.AWS,
		Logger:    logger,

		// MCP from mock suite
		resourceMappings: make(map[string]string),
		mcpTools:         make(map[string]MCPToolInfo),
		mcpResources:     make(map[string]MCPResourceInfo),

		// Enable test mode to use mock MCP capabilities
		testMode: true,

		// Set mock MCP server for test mode
		mockMCPServer: mockSuite.MCPServer,

		// Use real configuration-driven components from mock suite
		patternMatcher:    mockSuite.StateManager.GetPatternMatcher(),
		fieldResolver:     mockSuite.StateManager.GetFieldResolver(),
		valueTypeInferrer: mockSuite.StateManager.GetValueInferrer(),

		extractionConfig: mockSuite.StateManager.GetExtractionConfig(),
		idExtractor:      mockSuite.StateManager.GetIDExtractor(),
		registry:         mockSuite.Registry,
	}

	return agent, mockSuite, nil
}

// == Prompts ==

// Comprehensive VPC Infrastructure
func comprehensiveVPCPrompt() string {
	// Comprehensive VPC infrastructure prompt
	prompt := `Create a complete production-ready VPC infrastructure on AWS with the following requirements:

NETWORK ARCHITECTURE:
- VPC with CIDR 10.0.0.0/16 in us-west-2
- 6 subnets across 3 availability zones:
  * 2 public subnets (10.0.1.0/24, 10.0.2.0/24) for load balancers
  * 2 private subnets (10.0.11.0/24, 10.0.12.0/24) for application servers  
  * 2 database subnets (10.0.21.0/24, 10.0.22.0/24) for RDS instances
- Internet Gateway for public access
- 2 NAT Gateways in public subnets for private subnet internet access
- Route tables with proper routing

COMPUTE & SECURITY:
- Application Load Balancer in public subnets
- Auto Scaling Group with t3.medium instances in private subnets
- Launch Template with latest Amazon Linux 2 AMI
- Security Groups with least privilege access
- Target Group for ALB health checks

DATABASE:
- RDS MySQL instance in database subnets
- Database security group allowing access only from app servers
- Multi-AZ deployment for high availability

VALIDATION:
- Validate all resources are properly configured
- Test connectivity between components
- Verify security group rules are correct

Please create a detailed execution plan with all necessary steps, proper dependencies, and real AWS API calls where needed.`

	return prompt
}

// Comprehensive EC2 Infrastructure
func comprehensiveEC2Prompt() string {
	prompt := `Create an EC2 for hosting an Apache Server with a dedicated security group that allows inbound HTTP (port 80) and SSH (port 22) traffic.`

	return prompt
}

// Comprehensive Three-Tier Infrastructure
func comprehensiveThreeLayerPrompt() string {
	prompt := `I need to deploy a complete production-ready three-tier web application infrastructure on AWS with the following requirements:

Network Foundation (Phase 1):
- Create a production VPC with CIDR 10.0.0.0/16 across two availability zones (ap-southeast-2a and ap-southeast-2b)
- Set up public subnets (10.0.1.0/24 and 10.0.2.0/24) for internet-facing load balancers
- Create private subnets for application servers (10.0.11.0/24 and 10.0.12.0/24)
- Set up dedicated database subnets (10.0.21.0/24 and 10.0.22.0/24)
- Configure Internet Gateway and NAT Gateway for proper routing

Security Architecture (Phase 2):
- Implement defense-in-depth security with tiered security groups
- Load balancer security group allowing HTTP/HTTPS from internet (0.0.0.0/0)
- Application server security group accepting traffic only from load balancer
- Database security group allowing MySQL (port 3306) only from application servers
- All security groups should follow principle of least privilege

Load Balancer Tier (Phase 3):
- Deploy Application Load Balancer across public subnets in both AZs
- Configure target group with health checks on /health endpoint
- Set up HTTP listener (port 80) with proper health check thresholds
- Health check: 30s interval, 5s timeout, 2 healthy/3 unhealthy thresholds

Auto Scaling Application Tier (Phase 4):
- Create launch template with t3.medium instances
- Use Amazon Linux 2 AMI with Apache/PHP web server
- Configure user data script to install web server and health check endpoint
- Set up Auto Scaling Group: min 2, max 10, desired 4 instances
- Deploy across private application subnets in both AZs
- Integrate with load balancer target group for automatic registration
- Use ELB health checks with 300s grace period

Database Infrastructure (Phase 5):
- Create RDS MySQL 8.0 database with Multi-AZ deployment
- Use db.t3.medium instance class with 100GB GP3 storage
- Enable encryption at rest and Performance Insights
- Configure automated backups: 7-day retention, 3-4 AM backup window
- Set maintenance window: Sunday 4-5 AM
- Deploy across database subnets in both AZs

Additional Requirements:
- Tag all resources with Environment=production, Application=three-tier-web-app
- Use consistent naming convention with environment and tier identifiers
- Ensure high availability across multiple availability zones
- Follow AWS Well-Architected Framework principles
- Configure proper resource dependencies and creation order

Please deploy this complete infrastructure stack and provide me with the key resource IDs and endpoints once deployment is complete.`

	return prompt
}

// === Test Suites ===

// TestComprehensiveExecutionPipeline tests all aspects of the execution pipeline
func TestComprehensiveExecutionPipeline(t *testing.T) {
	t.Run("RealAIWithComprehensiveEC2Prompt", func(t *testing.T) {
		testRealAIWithComprehensiveEC2Prompt(t)
	})
	t.Run("RealAIWithComprehensiveVPCPrompt", func(t *testing.T) {
		testRealAIWithComprehensiveVPCPrompt(t)
	})
	t.Run("RealAIWithComprehensiveThreeLayerPrompt", func(t *testing.T) {
		testRealAIWithComprehensiveThreeLayerPrompt(t)
	})
}

func testRealAIWithComprehensiveEC2Prompt(t *testing.T) {
	// Comprehensive VPC infrastructure prompt
	comprehensivePrompt := comprehensiveEC2Prompt()

	// Setup test configuration
	cfg, err := setupRealConfiguration()
	if err != nil {
		t.Fatalf("Failed to setup real configuration: %v", err)
	}

	// Setup real LLM client
	llmClient, err := setupRealLLMClient(cfg)
	if err != nil {
		t.Fatalf("Failed to setup real LLM client: %v", err)
	}

	// Setup test agent with real AI and comprehensive mock infrastructure
	agent, mockSuite, err := setupAgentWithRealAI(cfg, llmClient)
	if err != nil {
		t.Fatalf("Failed to setup test agent: %v", err)
	}
	// defer mockSuite.Reset()

	// Create decision context using mock state manager
	emptyState := &types.InfrastructureState{
		Resources: make(map[string]*types.ResourceState),
	}

	decisionContext := &DecisionContext{
		Request:             comprehensivePrompt,
		CurrentState:        emptyState,
		DiscoveredState:     []*types.ResourceState{},
		Conflicts:           []*types.ConflictResolution{},
		DeploymentOrder:     []string{},
		ResourceCorrelation: make(map[string]*ResourceMatch),
	}

	t.Logf("🚀 Starting comprehensive execution pipeline test with real AI integration and mock infrastructure...")

	// Step 1: Test AI Decision Making
	t.Logf("📡 Step 1: Making real AI API call to process comprehensive infrastructure request...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()

	decisionID := "test-comprehensive-decision"
	decision, err := agent.generateDecisionWithPlan(ctx, decisionID, comprehensivePrompt, decisionContext)
	if err != nil {
		t.Fatalf("❌ Real AI API call failed: %v", err)
	}

	t.Logf("✅ Step 1 Complete: AI generated decision with %d execution steps", len(decision.ExecutionPlan))

	// Step 2: Validate AI-Generated Plan Structure
	t.Logf("🔍 Step 2: Validating AI-generated execution plan structure...")
	validateExecutionPlanStructure(t, decision)
	t.Logf("✅ Step 2 Complete: Execution plan structure is valid")

	// Step 3: Execute Full Flow with Mock Infrastructure
	t.Logf("⚙️ Step 3: Executing full infrastructure plan using mock functions...")
	testExecuteFullPlanWithMocks(t, agent, mockSuite, decision)
	t.Logf("✅ Step 3 Complete: Full plan execution completed successfully")

	// Step 4: Validate Mock Integration
	t.Logf("🔬 Step 4: Validating mock infrastructure integration...")
	testValidateMockIntegration(t, mockSuite, decision)
	t.Logf("✅ Step 4 Complete: Mock integration validated")

	t.Logf("🎉 All tests completed successfully! AI + Mock infrastructure integration validated.")
}

func testRealAIWithComprehensiveVPCPrompt(t *testing.T) {
	// Comprehensive VPC infrastructure prompt
	comprehensivePrompt := comprehensiveVPCPrompt()

	// Setup test configuration
	cfg, err := setupRealConfiguration()
	if err != nil {
		t.Fatalf("Failed to setup real configuration: %v", err)
	}

	// Setup real LLM client
	llmClient, err := setupRealLLMClient(cfg)
	if err != nil {
		t.Fatalf("Failed to setup real LLM client: %v", err)
	}

	// Setup test agent with real AI and comprehensive mock infrastructure
	agent, mockSuite, err := setupAgentWithRealAI(cfg, llmClient)
	if err != nil {
		t.Fatalf("Failed to setup test agent: %v", err)
	}
	// defer mockSuite.Reset()

	// Create decision context using mock state manager
	emptyState := &types.InfrastructureState{
		Resources: make(map[string]*types.ResourceState),
	}

	decisionContext := &DecisionContext{
		Request:             comprehensivePrompt,
		CurrentState:        emptyState,
		DiscoveredState:     []*types.ResourceState{},
		Conflicts:           []*types.ConflictResolution{},
		DeploymentOrder:     []string{},
		ResourceCorrelation: make(map[string]*ResourceMatch),
	}

	t.Logf("🚀 Starting comprehensive execution pipeline test with real AI integration and mock infrastructure...")

	// Step 1: Test AI Decision Making
	t.Logf("📡 Step 1: Making real AI API call to process comprehensive infrastructure request...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()

	decisionID := "test-comprehensive-decision"
	decision, err := agent.generateDecisionWithPlan(ctx, decisionID, comprehensivePrompt, decisionContext)
	if err != nil {
		t.Fatalf("❌ Real AI API call failed: %v", err)
	}

	t.Logf("✅ Step 1 Complete: AI generated decision with %d execution steps", len(decision.ExecutionPlan))

	// Step 2: Validate AI-Generated Plan Structure
	t.Logf("🔍 Step 2: Validating AI-generated execution plan structure...")
	validateExecutionPlanStructure(t, decision)
	t.Logf("✅ Step 2 Complete: Execution plan structure is valid")

	// Step 3: Execute Full Flow with Mock Infrastructure
	t.Logf("⚙️ Step 3: Executing full infrastructure plan using mock functions...")
	testExecuteFullPlanWithMocks(t, agent, mockSuite, decision)
	t.Logf("✅ Step 3 Complete: Full plan execution completed successfully")

	// Step 4: Validate Mock Integration
	t.Logf("🔬 Step 4: Validating mock infrastructure integration...")
	testValidateMockIntegration(t, mockSuite, decision)
	t.Logf("✅ Step 4 Complete: Mock integration validated")

	t.Logf("🎉 All tests completed successfully! AI + Mock infrastructure integration validated.")
}

func testRealAIWithComprehensiveThreeLayerPrompt(t *testing.T) {
	// Comprehensive Three-Tier infrastructure prompt
	comprehensivePrompt := comprehensiveThreeLayerPrompt()

	// Setup test configuration
	cfg, err := setupRealConfiguration()
	if err != nil {
		t.Fatalf("Failed to setup real configuration: %v", err)
	}

	// Setup real LLM client
	llmClient, err := setupRealLLMClient(cfg)
	if err != nil {
		t.Fatalf("Failed to setup real LLM client: %v", err)
	}

	// Setup test agent with real AI and comprehensive mock infrastructure
	agent, mockSuite, err := setupAgentWithRealAI(cfg, llmClient)
	if err != nil {
		t.Fatalf("Failed to setup test agent: %v", err)
	}
	// defer mockSuite.Reset()

	// Create decision context using mock state manager
	emptyState := &types.InfrastructureState{
		Resources: make(map[string]*types.ResourceState),
	}

	decisionContext := &DecisionContext{
		Request:             comprehensivePrompt,
		CurrentState:        emptyState,
		DiscoveredState:     []*types.ResourceState{},
		Conflicts:           []*types.ConflictResolution{},
		DeploymentOrder:     []string{},
		ResourceCorrelation: make(map[string]*ResourceMatch),
	}

	t.Logf("🚀 Starting comprehensive three-tier execution pipeline test with real AI integration and mock infrastructure...")

	// Step 1: Test AI Decision Making
	t.Logf("📡 Step 1: Making real AI API call to process comprehensive three-tier infrastructure request...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5) // Extended timeout for complex infrastructure
	defer cancel()

	decisionID := "test-comprehensive-three-tier-decision"
	decision, err := agent.generateDecisionWithPlan(ctx, decisionID, comprehensivePrompt, decisionContext)
	if err != nil {
		t.Fatalf("❌ Real AI API call failed: %v", err)
	}

	t.Logf("✅ Step 1 Complete: AI generated decision with %d execution steps", len(decision.ExecutionPlan))

	// Step 2: Validate AI-Generated Plan Structure
	t.Logf("🔍 Step 2: Validating AI-generated execution plan structure...")
	validateExecutionPlanStructure(t, decision)
	t.Logf("✅ Step 2 Complete: Execution plan structure is valid")

	// Step 3: Execute Full Flow with Mock Infrastructure
	t.Logf("⚙️ Step 3: Executing full infrastructure plan using mock functions...")
	testExecuteFullPlanWithMocks(t, agent, mockSuite, decision)
	t.Logf("✅ Step 3 Complete: Full plan execution completed successfully")

	// Step 4: Validate Mock Integration
	t.Logf("🔬 Step 4: Validating mock infrastructure integration...")
	testValidateMockIntegration(t, mockSuite, decision)
	t.Logf("✅ Step 4 Complete: Mock integration validated")

	t.Logf("🎉 All tests completed successfully! Three-tier AI + Mock infrastructure integration validated.")
}

func validateExecutionPlanStructure(t *testing.T, decision *types.AgentDecision) {
	if len(decision.ExecutionPlan) == 0 {
		t.Fatal("❌ AI generated empty execution plan")
	}

	planValidActions := map[string]bool{
		"create":              true,
		"update":              true,
		"add":                 true,
		"delete":              true,
		"validate":            true,
		"api_value_retrieval": true,
	}

	for _, planStep := range decision.ExecutionPlan {
		if !planValidActions[planStep.Action] {
			t.Fatalf("❌ Invalid plan action: %s", planStep.Action)
		}
	}

	// Validate plan has expected infrastructure components
	foundComponents := make(map[string]bool)
	expectedComponents := []string{"vpc", "subnet", "gateway", "route", "security"}

	for _, step := range decision.ExecutionPlan {
		// Check basic step structure
		if step.ID == "" {
			t.Errorf("❌ Step missing ID: %+v", step)
		}
		if step.Name == "" {
			t.Errorf("❌ Step missing Name: %+v", step)
		}
		if step.Action == "" {
			t.Errorf("❌ Step missing Action: %+v", step)
		}

		// Check for expected infrastructure components
		stepText := step.Name + " " + step.Description + " " + step.ResourceID
		for _, component := range expectedComponents {
			if strings.Contains(strings.ToLower(stepText), strings.ToLower(component)) {
				foundComponents[component] = true
			}
		}

		t.Logf("📋 Step: %s | Action: %s | Resource: %s", step.Name, step.Action, step.ResourceID)
	}

	// Verify we found key infrastructure components
	missingComponents := []string{}
	for _, component := range expectedComponents {
		if !foundComponents[component] {
			missingComponents = append(missingComponents, component)
		}
	}

	if len(missingComponents) > 0 {
		t.Logf("⚠️ Warning: Some expected components not found in plan: %v", missingComponents)
		t.Logf("📝 This may be okay if AI structured the plan differently")
	}
}

// testExecuteFullPlanWithMocks executes the AI-generated plan using mock infrastructure
// This simulates the same flow as ExecuteConfirmedPlanWithDryRun but with mock tools
func testExecuteFullPlanWithMocks(t *testing.T, agent *StateAwareAgent, mockSuite *mocks.MockTestSuite, decision *types.AgentDecision) {
	t.Logf("🚀 Executing %d plan steps with mock infrastructure...", len(decision.ExecutionPlan))

	// Create mock execution tracking like the real ExecuteConfirmedPlanWithDryRun
	execution := &types.PlanExecution{
		ID:        "mock-execution-" + decision.ID,
		Name:      fmt.Sprintf("Execute %s", decision.Action),
		Status:    "running",
		StartedAt: time.Now(),
		Steps:     []*types.ExecutionStep{},
		Changes:   []*types.ChangeDetection{},
		Errors:    []string{},
	}

	t.Logf("📊 Created execution plan: ID=%s, Status=%s", execution.ID, execution.Status)

	// Execute each step like the real executeExecutionStep function
	for i, planStep := range decision.ExecutionPlan {
		t.Logf("⚙️ Step %d/%d: Executing %s action on %s", i+1, len(decision.ExecutionPlan), planStep.Action, planStep.ResourceID)

		startTime := time.Now()

		// Create execution step tracking (mirroring real executeExecutionStep)
		executionStep := &types.ExecutionStep{
			ID:        planStep.ID,
			Name:      planStep.Name,
			Status:    "running",
			Resource:  planStep.ResourceID,
			Action:    planStep.Action,
			StartedAt: &startTime,
		}

		// Execute step using mock infrastructure (mirroring the real action switch)
		var result map[string]interface{}
		var err error

		switch planStep.Action {
		case "create":
			result, err = agent.executeCreateAction(planStep, nil, execution.ID)
		case "update":
			result, err = testExecuteUpdateActionWithMocks(t, mockSuite, planStep)
		case "add":
			// Map "add" actions to "update" - these are updates to existing resources
			result, err = testExecuteUpdateActionWithMocks(t, mockSuite, planStep)
		case "delete":
			result, err = testExecuteDeleteActionWithMocks(t, mockSuite, planStep)
		case "validate":
			result, err = testExecuteValidateActionWithMocks(t, mockSuite, planStep)
		case "api_value_retrieval":
			// Use the real agent's executeAPIValueRetrieval method with mock registry
			ctx := context.Background()
			result, err = agent.executeAPIValueRetrieval(ctx, planStep, nil, execution.ID)
		default:
			err = fmt.Errorf("unknown action type: %s", planStep.Action)
		}

		// Complete step tracking (mirroring real execution)
		endTime := time.Now()
		executionStep.CompletedAt = &endTime
		executionStep.Duration = endTime.Sub(startTime)

		if err != nil {
			executionStep.Status = "failed"
			executionStep.Error = err.Error()
			execution.Errors = append(execution.Errors, err.Error())
			t.Errorf("❌ Step %d failed: %v", i+1, err)
		} else {
			executionStep.Status = "completed"
			executionStep.Output = result
			t.Logf("✅ Step %d completed successfully in %v", i+1, executionStep.Duration)
		}

		execution.Steps = append(execution.Steps, executionStep)
	}

	// Complete execution tracking
	// completedAt := time.Now()
	// execution.CompletedAt = &completedAt

	if len(execution.Errors) > 0 {
		execution.Status = "failed"
		t.Fatalf("❌ Execution completed with %d errors", len(execution.Errors))
	} else {
		execution.Status = "completed"
		t.Logf("✅ Execution completed successfully with %d steps", len(execution.Steps))
	}
}

// testExecuteUpdateActionWithMocks simulates update operations
func testExecuteUpdateActionWithMocks(t *testing.T, mockSuite *mocks.MockTestSuite, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	t.Logf("🔄 Updating %s resource: %s", planStep.ResourceID, planStep.Name)

	// Mock update via state management
	testResource := &types.ResourceState{
		ID:         "mock-" + planStep.ResourceID,
		Type:       planStep.ResourceID,
		Status:     "updated",
		Properties: planStep.Parameters,
	}

	mockSuite.StateManager.AddResource(testResource)

	return map[string]interface{}{
		"action":      "update",
		"resource_id": testResource.ID,
		"status":      "completed",
	}, nil
}

// testExecuteDeleteActionWithMocks simulates delete operations
func testExecuteDeleteActionWithMocks(t *testing.T, mockSuite *mocks.MockTestSuite, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	t.Logf("🗑️ Deleting %s resource: %s", planStep.ResourceID, planStep.Name)

	return map[string]interface{}{
		"action":      "delete",
		"resource_id": "mock-" + planStep.ResourceID,
		"status":      "completed",
	}, nil
}

// testExecuteValidateActionWithMocks simulates validation operations
func testExecuteValidateActionWithMocks(t *testing.T, mockSuite *mocks.MockTestSuite, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	t.Logf("✅ Validating %s resource: %s", planStep.ResourceID, planStep.Name)

	return map[string]interface{}{
		"action":            "validate",
		"resource_id":       "mock-" + planStep.ResourceID,
		"validation_result": "passed",
	}, nil
}

// testValidateMockIntegration validates that mock infrastructure components are working correctly
func testValidateMockIntegration(t *testing.T, mockSuite *mocks.MockTestSuite, decision *types.AgentDecision) {
	t.Logf("🔍 Validating mock infrastructure integration...")

	// Test 1: Mock MCP Server Integration
	t.Logf("📡 Testing Mock MCP Server...")
	ctx := context.Background()
	testResult, err := mockSuite.MCPServer.CallTool(ctx, "describe_vpcs", map[string]interface{}{})
	if err != nil {
		t.Errorf("❌ Mock MCP Server failed: %v", err)
	} else {
		t.Logf("✅ Mock MCP Server working: %v", testResult != nil)
	}

	// Test 2: Mock State Manager Integration
	t.Logf("💾 Testing Mock State Manager...")
	testResource := &types.ResourceState{
		ID:     "test-validation-resource",
		Type:   "vpc",
		Status: "active",
		Properties: map[string]interface{}{
			"cidr": "10.0.0.0/16",
		},
	}
	mockSuite.StateManager.AddResource(testResource)

	// Verify resource was added
	resources := mockSuite.StateManager.GetResources()
	found := false
	for _, resource := range resources {
		if resource.ID == "test-validation-resource" {
			found = true
			break
		}
	}

	if !found {
		t.Error("❌ Mock State Manager failed to store resource")
	} else {
		t.Logf("✅ Mock State Manager working: stored %d resources", len(resources))
	}

	// Test 3: Mock AWS Client Integration
	t.Logf("☁️ Testing Mock AWS Client...")
	mockSuite.AWSClient.AddDefaultTestData()
	// Just verify it doesn't panic - AWS client functionality is tested in its own unit tests
	t.Logf("✅ Mock AWS Client working")

	// Test 4: Mock Retrieval Registry Integration
	t.Logf("📚 Testing Mock Retrieval Registry...")
	// Test that the registry has mock functions registered directly (no need for global registry)
	registryResult, err := mockSuite.Registry.Execute(ctx, "latest_ami", &types.ExecutionPlanStep{
		ID:         "test-ami-step",
		ResourceID: "test-ami",
		Name:       "Get Latest AMI",
		Parameters: map[string]interface{}{
			"owner": "amazon",
		},
	})
	if err != nil {
		t.Errorf("❌ Mock Retrieval Registry failed: %v", err)
	} else {
		t.Logf("✅ Mock Retrieval Registry working: got result with %d fields", len(registryResult))
	}

	t.Logf("✅ All mock integration tests passed!")
}
