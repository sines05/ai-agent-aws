package adapters

import (
	"context"
	"fmt"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// Helper functions for parameter extraction
func getString(params map[string]interface{}, key string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return ""
}

func getStringSlice(params map[string]interface{}, key string) []string {
	if val, ok := params[key].([]interface{}); ok {
		result := make([]string, len(val))
		for i, v := range val {
			if str, ok := v.(string); ok {
				result[i] = str
			}
		}
		return result
	}
	return []string{}
}

func getStringMap(params map[string]interface{}, key string) map[string]string {
	if val, ok := params[key].(map[string]interface{}); ok {
		result := make(map[string]string)
		for k, v := range val {
			if str, ok := v.(string); ok {
				result[k] = str
			}
		}
		return result
	}
	return map[string]string{}
}

func getInt32(params map[string]interface{}, key string, defaultVal int32) int32 {
	if val, ok := params[key].(float64); ok {
		return int32(val)
	}
	if val, ok := params[key].(int); ok {
		return int32(val)
	}
	if val, ok := params[key].(int32); ok {
		return val
	}
	return defaultVal
}

func getBool(params map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultVal
}

// ALBAdapter implements the AWSResourceAdapter interface for Application Load Balancers
type ALBAdapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewALBAdapter creates a new ALB adapter
func NewALBAdapter(client *aws.Client, logger *logging.Logger) interfaces.AWSResourceAdapter {
	base := NewBaseAWSAdapter(client, logger, "alb")
	return &ALBAdapter{
		BaseAWSAdapter: base,
		client:         client,
	}
}

// Create creates a new Application Load Balancer
func (a *ALBAdapter) Create(ctx context.Context, params interface{}) (*types.AWSResource, error) {
	createParams, ok := params.(aws.CreateLoadBalancerParams)
	if !ok {
		return nil, fmt.Errorf("invalid parameters for ALB creation, expected aws.CreateLoadBalancerParams")
	}

	alb, err := a.client.CreateApplicationLoadBalancer(ctx, createParams)
	if err != nil {
		return nil, err
	}

	return alb, nil
}

// List returns all Application Load Balancers
func (a *ALBAdapter) List(ctx context.Context) ([]*types.AWSResource, error) {
	return a.client.DescribeLoadBalancers(ctx)
}

// Get retrieves a specific Application Load Balancer
func (a *ALBAdapter) Get(ctx context.Context, id string) (*types.AWSResource, error) {
	return a.client.GetLoadBalancer(ctx, id)
}

// Update updates an Application Load Balancer (limited operations available)
func (a *ALBAdapter) Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error) {
	// ALB modifications are complex, should use specialized operations
	return nil, fmt.Errorf("ALB updates should use specialized operations")
}

// Delete deletes an Application Load Balancer
func (a *ALBAdapter) Delete(ctx context.Context, id string) error {
	// Note: ALB deletion is not implemented in the AWS client yet
	return fmt.Errorf("ALB deletion not yet implemented in AWS client")
}

// GetSupportedOperations returns the operations supported by this adapter
func (a *ALBAdapter) GetSupportedOperations() []string {
	return []string{
		"create",
		"list",
		"get",
		"delete",
		"create-target-group",
		"attach-target-group",
		"create-listener",
	}
}

// ValidateParams validates ALB-specific parameters
func (a *ALBAdapter) ValidateParams(operation string, params interface{}) error {
	switch operation {
	case "create":
		createParams, ok := params.(aws.CreateLoadBalancerParams)
		if !ok {
			return fmt.Errorf("invalid parameters for create operation")
		}
		if createParams.Name == "" {
			return fmt.Errorf("name is required for ALB creation")
		}
		if len(createParams.Subnets) == 0 {
			return fmt.Errorf("subnets are required for ALB creation")
		}
		return nil
	case "get", "delete":
		if params == nil {
			return fmt.Errorf("load balancer ARN is required for %s operation", operation)
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// ALBSpecializedAdapter adds ALB-specific operations
type ALBSpecializedAdapter struct {
	interfaces.AWSResourceAdapter
	client *aws.Client
}

// NewALBSpecializedAdapter creates an adapter with specialized ALB operations
func NewALBSpecializedAdapter(client *aws.Client, logger *logging.Logger) interfaces.SpecializedOperations {
	baseAdapter := NewALBAdapter(client, logger)
	return &ALBSpecializedAdapter{
		AWSResourceAdapter: baseAdapter,
		client:             client,
	}
}

// ExecuteSpecialOperation handles ALB-specific operations
func (a *ALBSpecializedAdapter) ExecuteSpecialOperation(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
	switch operation {
	case "create-load-balancer":
		// Extract parameters for load balancer creation
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid parameters for create-load-balancer")
		}

		albParams := aws.CreateLoadBalancerParams{
			Name:           getString(paramsMap, "name"),
			Scheme:         getString(paramsMap, "scheme"),
			Type:           getString(paramsMap, "type"),
			IpAddressType:  getString(paramsMap, "ipAddressType"),
			Subnets:        getStringSlice(paramsMap, "subnetIds"),
			SecurityGroups: getStringSlice(paramsMap, "securityGroupIds"),
			Tags:           getStringMap(paramsMap, "tags"),
		}

		loadBalancer, err := a.client.CreateApplicationLoadBalancer(ctx, albParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create load balancer: %w", err)
		}

		return loadBalancer, nil

	case "create-target-group":
		// Handle both old parameter format and new format
		if tgParams, ok := params.(aws.CreateTargetGroupParams); ok {
			return a.client.CreateTargetGroup(ctx, tgParams)
		}

		// Extract parameters from map for new format
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid parameters for create-target-group")
		}

		tgParams := aws.CreateTargetGroupParams{
			Name:                       getString(paramsMap, "name"),
			Protocol:                   getString(paramsMap, "protocol"),
			Port:                       getInt32(paramsMap, "port", 80),
			VpcID:                      getString(paramsMap, "vpcId"),
			TargetType:                 getString(paramsMap, "targetType"),
			HealthCheckEnabled:         getBool(paramsMap, "healthCheckEnabled", true),
			HealthCheckPath:            getString(paramsMap, "healthCheckPath"),
			HealthCheckProtocol:        getString(paramsMap, "healthCheckProtocol"),
			HealthCheckIntervalSeconds: getInt32(paramsMap, "healthCheckIntervalSeconds", 30),
			HealthCheckTimeoutSeconds:  getInt32(paramsMap, "healthCheckTimeoutSeconds", 5),
			HealthyThresholdCount:      getInt32(paramsMap, "healthyThresholdCount", 2),
			UnhealthyThresholdCount:    getInt32(paramsMap, "unhealthyThresholdCount", 2),
			Matcher:                    getString(paramsMap, "matcher"),
			Tags:                       getStringMap(paramsMap, "tags"),
		}

		targetGroup, err := a.client.CreateTargetGroup(ctx, tgParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create target group: %w", err)
		}

		return targetGroup, nil

	case "attach-target-group":
		attachParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("attach parameters required")
		}

		loadBalancerArn, _ := attachParams["loadBalancerArn"].(string)
		targetGroupArn, _ := attachParams["targetGroupArn"].(string)
		port, _ := attachParams["port"].(int32)

		if loadBalancerArn == "" || targetGroupArn == "" {
			return nil, fmt.Errorf("loadBalancerArn and targetGroupArn are required")
		}

		listenerParams := aws.CreateListenerParams{
			LoadBalancerArn:       loadBalancerArn,
			Port:                  port,
			Protocol:              "HTTP", // Default protocol
			DefaultTargetGroupArn: targetGroupArn,
		}

		return a.client.CreateListener(ctx, listenerParams)

	case "create-listener":
		// Handle both old parameter format and new format
		if listenerParams, ok := params.(aws.CreateListenerParams); ok {
			return a.client.CreateListener(ctx, listenerParams)
		}

		// Extract parameters from map for new format
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid parameters for create-listener")
		}

		listenerParams := aws.CreateListenerParams{
			LoadBalancerArn:       getString(paramsMap, "loadBalancerArn"),
			Protocol:              getString(paramsMap, "protocol"),
			Port:                  getInt32(paramsMap, "port", 80),
			DefaultTargetGroupArn: getString(paramsMap, "defaultTargetGroupArn"),
			CertificateArn:        getString(paramsMap, "certificateArn"),
		}

		listener, err := a.client.CreateListener(ctx, listenerParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create listener: %w", err)
		}

		return listener, nil

	case "list-target-groups":
		// List all Target Groups
		targetGroups, err := a.client.DescribeTargetGroups(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list target groups: %w", err)
		}

		// Return a summary resource with all target groups
		return &types.AWSResource{
			ID:    "target-group-list",
			Type:  "target-group-list",
			State: "available",
			Details: map[string]interface{}{
				"count":        len(targetGroups),
				"targetGroups": targetGroups,
			},
		}, nil

	case "register-targets":
		// Extract parameters for target registration
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid parameters for register-targets")
		}

		targetGroupArn := getString(paramsMap, "targetGroupArn")
		instanceIds := getStringSlice(paramsMap, "instanceIds")

		err := a.client.RegisterTargets(ctx, targetGroupArn, instanceIds)
		if err != nil {
			return nil, fmt.Errorf("failed to register targets: %w", err)
		}

		return &types.AWSResource{
			ID:    targetGroupArn,
			Type:  "target-registration",
			State: "registered",
			Details: map[string]interface{}{
				"targetGroupArn": targetGroupArn,
				"instanceIds":    instanceIds,
				"count":          len(instanceIds),
			},
		}, nil

	case "deregister-targets":
		// Extract parameters for target deregistration
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid parameters for deregister-targets")
		}

		targetGroupArn := getString(paramsMap, "targetGroupArn")
		instanceIds := getStringSlice(paramsMap, "instanceIds")

		err := a.client.DeregisterTargets(ctx, targetGroupArn, instanceIds)
		if err != nil {
			return nil, fmt.Errorf("failed to deregister targets: %w", err)
		}

		return &types.AWSResource{
			ID:    targetGroupArn,
			Type:  "target-deregistration",
			State: "deregistered",
			Details: map[string]interface{}{
				"targetGroupArn": targetGroupArn,
				"instanceIds":    instanceIds,
				"count":          len(instanceIds),
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported specialized operation: %s", operation)
	}
}

// GetSpecialOperations returns the specialized operations available
func (a *ALBSpecializedAdapter) GetSpecialOperations() []string {
	return []string{
		"create-load-balancer",
		"create-target-group",
		"attach-target-group",
		"create-listener",
		"list-target-groups",
		"register-targets",
		"deregister-targets",
	}
}
