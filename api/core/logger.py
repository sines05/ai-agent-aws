"""
Centralized logging configuration for the AI Infrastructure Agent.

This module provides a `setup_logger` function that configures and returns a
standard Python logger. It is designed to replicate the logging format from the
original Go application, ensuring consistency during the migration.
"""

import logging
import sys

def setup_logger() -> logging.Logger:
    """
    Sets up and returns a configured logger.

    The logger is configured to output to the console (stdout) with a
    format that includes a timestamp, log level, and the message.
    This function ensures that a single, consistent logger is used
    throughout the application.

    Returns:
        A configured instance of logging.Logger.
    """
    logger = logging.getLogger("ai_infra_agent")
    logger.setLevel(logging.INFO)

    # Prevent duplicate handlers if this function is called multiple times
    if not logger.handlers:
        # Configure console handler
        handler = logging.StreamHandler(sys.stdout)
        formatter = logging.Formatter(
            "[%(asctime)s] [%(levelname)s] %(message)s",
            datefmt="%Y-%m-%d %H:%M:%S",
        )
        handler.setFormatter(formatter)
        logger.addHandler(handler)

    return logger

# Initialize a default logger for easy import
log = setup_logger()
