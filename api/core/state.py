"""
State Management for the AI Infrastructure Agent.

This module provides a StateManager class that handles loading and saving the
infrastructure state to a JSON file. This ensures that the agent can maintain
a persistent record of the resources it manages.
"""

import json
import os
from typing import Dict, Any

class StateManager:
    """
    Manages the loading and saving of the infrastructure state.
    """
    def __init__(self, state_file_path: str):
        """
        Initializes the StateManager.

        Args:
            state_file_path: The path to the JSON file where the state is stored.
        """
        self.state_file_path = state_file_path

    def load_state(self) -> Dict[str, Any]:
        """
        Loads the infrastructure state from the state file.

        If the state file does not exist, it returns an empty dictionary.

        Returns:
            A dictionary representing the current infrastructure state.
        """
        if not os.path.exists(self.state_file_path):
            return {}
        try:
            with open(self.state_file_path, 'r') as f:
                return json.load(f)
        except json.JSONDecodeError:
            # Handle cases where the file is empty or corrupt
            return {}

    def save_state(self, state: Dict[str, Any]):
        """
        Saves the infrastructure state to the state file.

        Args:
            state: A dictionary representing the current infrastructure state.
        """
        with open(self.state_file_path, 'w') as f:
            json.dump(state, f, indent=2)
