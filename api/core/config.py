"""
Core configuration loading and validation for the AI Infrastructure Agent.

This module translates the Go configuration structs into Python Pydantic models,
ensuring robust type validation and settings management. It provides a single
function, `load_config`, to read a YAML file and parse it into a Config object.
"""

import yaml
from pydantic import BaseModel, Field
from typing import Optional

class AWSConfig(BaseModel):
    """AWS-specific configuration."""
    region: str = "us-west-2"

class MCPConfig(BaseModel):
    """Model Context Protocol (MCP) configuration."""
    server_name: str = "ai-infrastructure-agent"
    version: str = "1.0.0"

class AgentConfig(BaseModel):
    """Configuration for the AI agent."""
    provider: str = "openai"
    model: str = "gpt-4"
    max_tokens: int = 4000
    temperature: float = 0.3
    dry_run: bool = True
    auto_resolve_conflicts: bool = False
    enable_debug: bool = False
    openai_api_key: Optional[str] = Field(None, env="OPENAI_API_KEY")
    gemini_api_key: Optional[str] = Field(None, env="GEMINI_API_KEY")
    anthropic_api_key: Optional[str] = Field(None, env="ANTHROPIC_API_KEY")

class LoggingConfig(BaseModel):
    """Logging configuration."""
    level: str = "info"
    format: str = "text"
    output: str = "stdout"

class StateConfig(BaseModel):
    """State management configuration."""
    file_path: str = "infrastructure.state"
    backup_enabled: bool = True
    backup_dir: str = "./backups"

class WebConfig(BaseModel):
    """Web server configuration."""
    port: int = 8080
    host: str = "localhost"
    template_dir: str = "web/templates"
    static_dir: str = "web/static"
    enable_websockets: bool = True

class Config(BaseModel):
    """Main configuration structure."""
    aws: AWSConfig = Field(default_factory=AWSConfig)
    mcp: MCPConfig = Field(default_factory=MCPConfig)
    agent: AgentConfig = Field(default_factory=AgentConfig)
    logging: LoggingConfig = Field(default_factory=LoggingConfig)
    state: StateConfig = Field(default_factory=StateConfig)
    web: WebConfig = Field(default_factory=WebConfig)

def load_config(path: str) -> Config:
    """
    Loads configuration from a YAML file and returns a Config object.

    Args:
        path: The path to the YAML configuration file.

    Returns:
        An instance of the Config model populated with the file's data.

    Raises:
        FileNotFoundError: If the configuration file is not found.
        yaml.YAMLError: If there is an error parsing the YAML file.
    """
    try:
        with open(path, 'r') as f:
            config_data = yaml.safe_load(f)
        return Config(**config_data)
    except FileNotFoundError:
        raise FileNotFoundError(f"Configuration file not found at: {path}")
    except yaml.YAMLError as e:
        raise yaml.YAMLError(f"Error parsing YAML file at {path}: {e}")
