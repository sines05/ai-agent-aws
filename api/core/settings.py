"""
AI Knowledge and Settings Loader for the AI Infrastructure Agent.

This module is responsible for loading the AI's "external brain"—the rules,
patterns, and templates that define its behavior. It replicates the data-loading
mechanism from the Go project, loading YAML files and prompt templates from the
`settings/` directory.
"""

import os
import yaml
from typing import Dict, Any

class SettingsLoader:
    """
    Loads and holds the AI's operational settings and prompt templates.
    """
    def __init__(self, settings_path: str = "settings"):
        """
        Initializes the SettingsLoader and loads all necessary files.

        Args:
            settings_path: The path to the settings directory.

        Raises:
            FileNotFoundError: If a required settings file is not found.
            yaml.YAMLError: If a YAML file cannot be parsed.
        """
        self.settings_path = settings_path
        self.field_mappings: Dict[str, Any] = {}
        self.resource_extraction: Dict[str, Any] = {}
        self.resource_patterns: Dict[str, Any] = {}
        self.prompts: Dict[str, str] = {}

        self._load_all_settings()

    def _load_yaml_file(self, file_name: str) -> Dict[str, Any]:
        """Loads a single YAML file from the settings directory."""
        file_path = os.path.join(self.settings_path, file_name)
        try:
            with open(file_path, 'r') as f:
                return yaml.safe_load(f)
        except FileNotFoundError:
            raise FileNotFoundError(f"Settings file not found: {file_path}")
        except yaml.YAMLError as e:
            raise yaml.YAMLError(f"Error parsing YAML file {file_path}: {e}")

    def _load_prompts(self):
        """Loads all .txt prompt templates from the templates subdirectory."""
        templates_path = os.path.join(self.settings_path, "templates")
        if not os.path.isdir(templates_path):
            # Handle case where templates directory might be missing
            return

        for file_name in os.listdir(templates_path):
            if file_name.endswith(".txt"):
                prompt_key = file_name.replace(".txt", "")
                file_path = os.path.join(templates_path, file_name)
                with open(file_path, 'r') as f:
                    self.prompts[prompt_key] = f.read()

    def _load_all_settings(self):
        """Loads all YAML configurations and prompt templates."""
        self.field_mappings = self._load_yaml_file("field-mappings-enhanced.yaml")
        self.resource_extraction = self._load_yaml_file("resource-extraction-enhanced.yaml")
        self.resource_patterns = self._load_yaml_file("resource-patterns-enhanced.yaml")
        self._load_prompts()
