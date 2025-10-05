"""
AWS Interaction Layer for the AI Infrastructure Agent.

This module provides a client class for interacting with AWS services. It acts
as an abstraction layer over the AWS SDK (Boto3), offering high-level methods
for the agent to perform actions on cloud resources.
"""

import boto3
from botocore.exceptions import ClientError
from typing import Dict, Any, List

class AWSClient:
    """
    A client for interacting with AWS services using Boto3.
    """
    def __init__(self, region_name: str = "us-west-2"):
        """
        Initializes the AWSClient and Boto3 clients for required services.

        Args:
            region_name: The AWS region to connect to.
        """
        self.ec2_client = boto3.client("ec2", region_name=region_name)
        self.s3_client = boto3.client("s3", region_name=region_name)

    def create_ec2_instance(
        self, image_id: str, instance_type: str, key_name: str, security_group_ids: List[str]
    ) -> Dict[str, Any]:
        """
        Creates a new EC2 instance.

        Args:
            image_id: The ID of the AMI to use.
            instance_type: The instance type (e.g., 't2.micro').
            key_name: The name of the key pair to use.
            security_group_ids: A list of security group IDs.

        Returns:
            The dictionary response from the Boto3 call.
        """
        try:
            response = self.ec2_client.run_instances(
                ImageId=image_id,
                InstanceType=instance_type,
                KeyName=key_name,
                SecurityGroupIds=security_group_ids,
                MinCount=1,
                MaxCount=1,
            )
            return response
        except ClientError as e:
            print(f"Error creating EC2 instance: {e}")
            raise

    def terminate_ec2_instance(self, instance_id: str) -> Dict[str, Any]:
        """
        Terminates an EC2 instance.

        Args:
            instance_id: The ID of the instance to terminate.

        Returns:
            The dictionary response from the Boto3 call.
        """
        try:
            response = self.ec2_client.terminate_instances(InstanceIds=[instance_id])
            return response
        except ClientError as e:
            print(f"Error terminating EC2 instance {instance_id}: {e}")
            raise

    def create_s3_bucket(self, bucket_name: str, region: str) -> Dict[str, Any]:
        """
        Creates a new S3 bucket.

        Args:
            bucket_name: The name of the bucket to create.
            region: The AWS region where the bucket will be created.

        Returns:
            The dictionary response from the Boto3 call.
        """
        try:
            location = {'LocationConstraint': region}
            response = self.s3_client.create_bucket(
                Bucket=bucket_name,
                CreateBucketConfiguration=location
            )
            return response
        except ClientError as e:
            print(f"Error creating S3 bucket {bucket_name}: {e}")
            raise

    def list_vpcs(self) -> Dict[str, Any]:
        """
        Lists all VPCs in the configured region.

        Returns:
            The dictionary response from the Boto3 call.
        """
        try:
            response = self.ec2_client.describe_vpcs()
            return response
        except ClientError as e:
            print(f"Error listing VPCs: {e}")
            raise
