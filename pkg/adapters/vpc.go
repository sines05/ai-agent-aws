package adapters

import (
	"context"
	"fmt"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// VPCAdapter implements the AWSResourceAdapter interface for VPC resources
type VPCAdapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewVPCAdapter creates a new VPC adapter
func NewVPCAdapter(client *aws.Client, logger *logging.Logger) interfaces.AWSResourceAdapter {
	base := NewBaseAWSAdapter(client, logger, "vpc")
	return &VPCAdapter{
		BaseAWSAdapter: base,
		client:         client,
	}
}

// Create creates a new VPC
func (v *VPCAdapter) Create(ctx context.Context, params interface{}) (*types.AWSResource, error) {
	createParams, ok := params.(aws.CreateVPCParams)
	if !ok {
		return nil, fmt.Errorf("invalid parameters for VPC creation, expected aws.CreateVPCParams")
	}

	return v.client.CreateVPC(ctx, createParams)
}

// List returns all VPCs
func (v *VPCAdapter) List(ctx context.Context) ([]*types.AWSResource, error) {
	return v.client.DescribeVPCs(ctx)
}

// Get retrieves a specific VPC
func (v *VPCAdapter) Get(ctx context.Context, id string) (*types.AWSResource, error) {
	vpcs, err := v.client.DescribeVPCs(ctx)
	if err != nil {
		return nil, err
	}

	for _, vpc := range vpcs {
		if vpc.ID == id {
			return vpc, nil
		}
	}

	return nil, fmt.Errorf("VPC %s not found", id)
}

// Update updates a VPC (limited operations available)
func (v *VPCAdapter) Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error) {
	// VPCs have very limited update operations
	return nil, fmt.Errorf("VPC updates not supported via standard interface, use specialized operations")
}

// Delete deletes a VPC (not implemented in AWS client)
func (v *VPCAdapter) Delete(ctx context.Context, id string) error {
	// VPC deletion is complex and requires removing all dependencies first
	// For now, return not implemented error
	return fmt.Errorf("VPC deletion not implemented - requires manual dependency cleanup")
}

// GetSupportedOperations returns the operations supported by this adapter
func (v *VPCAdapter) GetSupportedOperations() []string {
	return []string{
		"create",
		"list",
		"get",
		"delete",
		"create-subnet",
		"create-internet-gateway",
		"create-route-table",
	}
}

// ValidateParams validates VPC-specific parameters
func (v *VPCAdapter) ValidateParams(operation string, params interface{}) error {
	switch operation {
	case "create":
		createParams, ok := params.(aws.CreateVPCParams)
		if !ok {
			return fmt.Errorf("invalid parameters for create operation")
		}
		if createParams.CidrBlock == "" {
			return fmt.Errorf("cidrBlock is required for VPC creation")
		}
		return nil
	case "get", "delete":
		if params == nil {
			return fmt.Errorf("VPC ID is required for %s operation", operation)
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// SubnetAdapter implements the AWSResourceAdapter interface for Subnet resources
type SubnetAdapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewSubnetAdapter creates a new Subnet adapter
func NewSubnetAdapter(client *aws.Client, logger *logging.Logger) interfaces.AWSResourceAdapter {
	base := NewBaseAWSAdapter(client, logger, "subnet")
	return &SubnetAdapter{
		BaseAWSAdapter: base,
		client:         client,
	}
}

// Create creates a new subnet
func (s *SubnetAdapter) Create(ctx context.Context, params interface{}) (*types.AWSResource, error) {
	createParams, ok := params.(aws.CreateSubnetParams)
	if !ok {
		return nil, fmt.Errorf("invalid parameters for subnet creation, expected aws.CreateSubnetParams")
	}

	return s.client.CreateSubnet(ctx, createParams)
}

// List returns all subnets
func (s *SubnetAdapter) List(ctx context.Context) ([]*types.AWSResource, error) {
	return s.client.DescribeSubnetsAll(ctx)
}

// Get retrieves a specific subnet
func (s *SubnetAdapter) Get(ctx context.Context, id string) (*types.AWSResource, error) {
	subnets, err := s.client.DescribeSubnetsAll(ctx)
	if err != nil {
		return nil, err
	}

	for _, subnet := range subnets {
		if subnet.ID == id {
			return subnet, nil
		}
	}

	return nil, fmt.Errorf("subnet %s not found", id)
}

// Update updates a subnet (limited operations available)
func (s *SubnetAdapter) Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error) {
	return nil, fmt.Errorf("subnet updates not supported via standard interface")
}

// Delete deletes a subnet (not implemented in AWS client)
func (s *SubnetAdapter) Delete(ctx context.Context, id string) error {
	// Subnet deletion not implemented in current AWS client
	return fmt.Errorf("subnet deletion not implemented in AWS client")
}

// GetSupportedOperations returns the operations supported by this adapter
func (s *SubnetAdapter) GetSupportedOperations() []string {
	return []string{
		"create",
		"list",
		"get",
		"delete",
	}
}

// ValidateParams validates subnet-specific parameters
func (s *SubnetAdapter) ValidateParams(operation string, params interface{}) error {
	switch operation {
	case "create":
		createParams, ok := params.(aws.CreateSubnetParams)
		if !ok {
			return fmt.Errorf("invalid parameters for create operation")
		}
		if createParams.VpcID == "" {
			return fmt.Errorf("vpcId is required for subnet creation")
		}
		if createParams.CidrBlock == "" {
			return fmt.Errorf("cidrBlock is required for subnet creation")
		}
		return nil
	case "get", "delete":
		if params == nil {
			return fmt.Errorf("subnet ID is required for %s operation", operation)
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// VPCSpecializedAdapter adds VPC-specific operations
type VPCSpecializedAdapter struct {
	interfaces.AWSResourceAdapter
	client *aws.Client
}

// NewVPCSpecializedAdapter creates an adapter with specialized VPC operations
func NewVPCSpecializedAdapter(client *aws.Client, logger *logging.Logger) interfaces.SpecializedOperations {
	baseAdapter := NewVPCAdapter(client, logger)
	return &VPCSpecializedAdapter{
		AWSResourceAdapter: baseAdapter,
		client:             client,
	}
}

// ExecuteSpecialOperation handles VPC-specific operations
func (v *VPCSpecializedAdapter) ExecuteSpecialOperation(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
	switch operation {
	case "create-subnet":
		subnetParams, ok := params.(aws.CreateSubnetParams)
		if !ok {
			return nil, fmt.Errorf("subnet parameters required for create-subnet operation")
		}
		return v.client.CreateSubnet(ctx, subnetParams)

	case "create-internet-gateway":
		igwParams, ok := params.(aws.CreateInternetGatewayParams)
		if !ok {
			return nil, fmt.Errorf("internet gateway parameters required")
		}
		vpcID, _ := params.(map[string]interface{})["vpcId"].(string)
		return v.client.CreateInternetGateway(ctx, igwParams, vpcID)

	case "create-route-table":
		routeParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("route table parameters required")
		}
		vpcID, _ := routeParams["vpcId"].(string)
		name, _ := routeParams["name"].(string)

		if vpcID == "" {
			return nil, fmt.Errorf("vpcId is required for route table creation")
		}

		return v.client.CreateRouteTable(ctx, vpcID, name)

	default:
		return nil, fmt.Errorf("unsupported specialized operation: %s", operation)
	}
}

// GetSpecialOperations returns the specialized operations available
func (v *VPCSpecializedAdapter) GetSpecialOperations() []string {
	return []string{"create-subnet", "create-internet-gateway", "create-route-table"}
}
