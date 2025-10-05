"""
Main API Server for the AI Infrastructure Agent.

This module initializes a Flask application and wires together all the core
components of the agent. It serves the frontend static files and provides the
necessary API endpoints for the frontend to communicate with the backend.
"""

import os
import openai
from flask import Flask, jsonify, request, send_from_directory
from flask_cors import CORS

# Import core components
from core.config import load_config
from core.logger import setup_logger
from core.settings import SettingsLoader
from core.state import StateManager
from core.aws import AWSClient
from core.agent import Planner, Executor

# --- Initialization ---

# Load configuration
# Construct the path to the config file in the project root
config_path = os.path.abspath(os.path.join(os.path.dirname(__file__), '..', 'config.yaml'))
config = load_config(config_path)

# Set up logger
log = setup_logger()

# Initialize core components
log.info("Initializing core components...")
# Construct the path to the settings directory in the project root
settings_path = os.path.abspath(os.path.join(os.path.dirname(__file__), '..', 'settings'))
settings = SettingsLoader(settings_path=settings_path)
state_manager = StateManager(config.state.file_path)
aws_client = AWSClient(region_name=config.aws.region)

# Initialize OpenAI client (handle different providers as needed)
# Note: This assumes OpenAI for now. A factory function would be better for multi-provider support.
api_key = os.getenv("OPENAI_API_KEY", config.agent.openai_api_key)
if not api_key:
    log.warning("OpenAI API key not found. Planner will not function.")
    openai_client = None
else:
    openai_client = openai.OpenAI(api_key=api_key)

planner = Planner(settings, openai_client)
executor = Executor(aws_client, state_manager)
log.info("Core components initialized successfully.")

# --- Flask App Setup ---

# The frontend build directory is relative to the project root, not the api directory
build_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), '..', 'web', 'build'))
static_dir = os.path.join(build_dir, 'static')

app = Flask(__name__, static_folder=static_dir, static_url_path='/static')
CORS(app, resources={r"/api/*": {"origins": "*"}})

# --- API Endpoints ---

@app.route('/api/state', methods=['GET'])
def get_state():
    """Returns the current infrastructure state."""
    log.info("API call: GET /api/state")
    state = state_manager.load_state()
    return jsonify(state)

@app.route('/api/requests', methods=['POST'])
def create_request():
    """Takes a user prompt, calls the Planner, and returns a plan."""
    log.info("API call: POST /api/requests")
    if not openai_client:
        return jsonify({"error": "Planner is not configured; API key missing."}), 500

    data = request.get_json()
    user_request = data.get('prompt')
    if not user_request:
        return jsonify({"error": "Prompt is required"}), 400

    current_state = state_manager.load_state()
    try:
        plan = planner.create_plan(user_request, current_state)
        return jsonify(plan)
    except Exception as e:
        log.error(f"Error creating plan: {e}")
        return jsonify({"error": str(e)}), 500

@app.route('/api/plans/execute', methods=['POST'])
def execute_plan_endpoint():
    """Takes a plan, calls the Executor, and returns the new state."""
    log.info("API call: POST /api/plans/execute")
    plan = request.get_json()
    if not plan:
        return jsonify({"error": "Plan is required"}), 400

    current_state = state_manager.load_state()
    try:
        new_state = executor.execute_plan(plan, current_state)
        return jsonify(new_state)
    except Exception as e:
        log.error(f"Error executing plan: {e}")
        return jsonify({"error": str(e)}), 500

# --- Static File Serving for Frontend ---

@app.route('/', defaults={'path': ''})
@app.route('/<path:path>')
def serve_frontend(path):
    """Serves the frontend application's entry point and root assets."""
    if path.startswith("api/"):
        return jsonify({"error": "API endpoint not found"}), 404

    if path and os.path.exists(os.path.join(build_dir, path)):
        return send_from_directory(build_dir, path)
    
    return send_from_directory(build_dir, 'index.html')

if __name__ == '__main__':
    log.info(f"Starting Flask server on http://{config.web.host}:{config.web.port}")
    app.run(host=config.web.host, port=config.web.port, debug=config.agent.enable_debug)
