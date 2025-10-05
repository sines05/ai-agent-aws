#!/bin/bash

# AI Infrastructure Agent Installation Script
# This script sets up the Python environment for the AI Infrastructure Agent

set -e  # Exit on any error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${PURPLE}================================${NC}"
    echo -e "${PURPLE}$1${NC}"
    echo -e "${PURPLE}================================${NC}"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check Python version
check_python_version() {
    if ! command_exists python3; then
        print_error "Python 3 is not installed. Please install Python 3.11 or higher."
        return 1
    fi

    local current_version=$(python3 -c 'import sys; print(".".join(map(str, sys.version_info[:3])))')
    local required_version="3.11.0"

    if [[ "$(printf '%s\n' "$required_version" "$current_version" | sort -V | head -n1)" == "$required_version" ]]; then
        print_success "Python version check passed: $current_version"
        return 0
    else
        print_error "Python version is $current_version. Version $required_version or higher is required."
        return 1
    fi
}

# Function to setup Python environment
setup_python_env() {
    print_status "Setting up Python environment..."

    if ! command_exists pip3; then
        print_error "pip3 is not installed. Please install pip for Python 3."
        exit 1
    fi

    print_status "Installing dependencies from api/requirements.txt..."
    pip3 install -r api/requirements.txt

    print_success "Python dependencies installed."
}


# Function to setup AWS CLI if not present
setup_aws_cli() {
    print_status "Checking AWS CLI installation..."

    if ! command_exists aws; then
        print_warning "AWS CLI not found. Installing..."
        
        if command_exists pip3; then
            pip3 install awscli
        else
            print_warning "pip3 not found. Please install AWS CLI manually: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
            return 1
        fi
    else
        print_success "AWS CLI found: $(aws --version)"
    fi
}

# Function to create directories
create_directories() {
    print_status "Creating necessary directories..."

    mkdir -p bin
    mkdir -p states
    mkdir -p backups

    print_success "Directories created"
}

# Function to setup configuration
setup_config() {
    print_status "Setting up configuration..."

    if [[ ! -f "config.yaml" ]]; then
        if [[ -f "config.openai.yaml.example" ]]; then
            print_status "Copying example configuration..."
            cp config.openai.yaml.example config.yaml
            print_warning "Please edit config.yaml to set your AWS region and API keys"
        else
            print_error "Example configuration file not found."
        fi
        print_success "Configuration file created: config.yaml"
    else
        print_success "Configuration file already exists: config.yaml"
    fi
}

# Function to print usage instructions
print_usage() {
    print_header "Installation Complete!"

    echo ""
    echo -e "${CYAN}🎉 AI Infrastructure Agent has been successfully installed!${NC}"
    echo ""
    echo -e "${YELLOW}Next Steps:${NC}"
    echo ""
    echo "1. 📝 Configure your settings:"
    echo "   - Edit config.yaml to set your AWS region and preferences"
    echo "   - Set environment variables for your AI provider:"
    echo "     export OPENAI_API_KEY='your-key-here'     # For OpenAI"
    echo "     export GEMINI_API_KEY='your-key-here'     # For Google Gemini"
    echo "     export ANTHROPIC_API_KEY='your-key-here'  # For Anthropic"
    echo ""
    echo "2. 🔐 Configure AWS credentials:"
    echo "   aws configure"
    echo ""
    echo "3. 🚀 Run the application:"
    echo "   # Start Web UI:"
    echo "   ./scripts/run-web-ui.sh"
    echo ""
    echo "4. 🌐 Access the Web UI:"
    echo "   Open http://localhost:8080 in your browser"
    echo ""
    echo -e "${YELLOW}Troubleshooting:${NC}"
    echo "- Ensure your AWS credentials are properly configured"
    echo "- Verify your AI provider API key is set correctly"
    echo "- Check that port 8080 is available"
    echo ""
    echo -e "${CYAN}For more information, check the README.md file${NC}"
}

# Main installation function
main() {
    print_header "AI Infrastructure Agent Installation"

    # Check if we're in the right directory
    if [[ ! -f "api/app.py" ]]; then
        print_error "Please run this script from the AI Infrastructure Agent root directory"
        exit 1
    fi

    print_status "Starting installation process..."

    # Check Python version
    if ! check_python_version; then
        exit 1
    fi

    # Setup Python environment
    setup_python_env

    # Setup AWS CLI
    setup_aws_cli

    # Create necessary directories
    create_directories

    # Setup configuration
    setup_config

    # Make scripts executable
    if [[ -f "scripts/run-web-ui.sh" ]]; then
        chmod +x scripts/run-web-ui.sh
        print_success "Made scripts/run-web-ui.sh executable"
    fi

    # Print usage instructions
    print_usage
}

# Handle command line arguments
case "${1:-}" in
    --help|-h)
        echo "AI Infrastructure Agent Installation Script"
        echo ""
        echo "Usage: $0 [options]"
        echo ""
        echo "Options:"
        echo "  --help, -h     Show this help message"
        echo ""
        echo "This script will:"
        echo "- Check for Python 3.11+"
        echo "- Install Python dependencies"
        echo "- Setup AWS CLI"
        echo "- Create necessary directories"
        echo "- Setup configuration files"
        exit 0
        ;;
    *)
        main
        ;;
esac
