#!/bin/bash
# Purpose    : Deployment script for pg-metako Docker environment
# Context    : Simplify Docker Compose operations and environment management
# Constraints: Must handle environment setup, service management, and error handling

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

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

# Function to check if Docker is running
check_docker() {
    if ! docker info >/dev/null 2>&1; then
        print_error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
}

# Function to check if docker-compose is available
check_docker_compose() {
    if ! command -v docker-compose >/dev/null 2>&1; then
        print_error "docker-compose is not installed. Please install docker-compose and try again."
        exit 1
    fi
}

# Function to setup environment file
setup_env() {
    cd "$PROJECT_DIR"
    
    if [ ! -f .env ]; then
        print_status "Creating .env file from .env.example..."
        cp .env.example .env
        print_warning "Please edit .env file to set your passwords and configuration!"
        print_warning "Default passwords are not secure for production use."
    else
        print_status ".env file already exists."
    fi
}

# Function to build images
build_images() {
    cd "$PROJECT_DIR"
    print_status "Building pg-metako Docker image..."
    docker-compose build --no-cache pg-metako
    print_success "Docker image built successfully!"
}

# Function to start services
start_services() {
    cd "$PROJECT_DIR"
    print_status "Starting pg-metako services..."
    
    # Start PostgreSQL services first
    print_status "Starting PostgreSQL master..."
    docker-compose up -d postgres-master
    
    # Wait for master to be healthy
    print_status "Waiting for PostgreSQL master to be ready..."
    timeout=60
    while [ $timeout -gt 0 ]; do
        if docker-compose exec -T postgres-master pg_isready -U postgres >/dev/null 2>&1; then
            break
        fi
        sleep 2
        timeout=$((timeout - 2))
    done
    
    if [ $timeout -le 0 ]; then
        print_error "PostgreSQL master failed to start within 60 seconds"
        exit 1
    fi
    
    print_success "PostgreSQL master is ready!"
    
    # Start slave services
    print_status "Starting PostgreSQL slaves..."
    docker-compose up -d postgres-slave1 postgres-slave2
    
    # Wait for slaves to be ready
    print_status "Waiting for PostgreSQL slaves to be ready..."
    sleep 10
    
    # Start pg-metako application
    print_status "Starting pg-metako application..."
    docker-compose up -d pg-metako
    
    print_success "All services started successfully!"
}

# Function to stop services
stop_services() {
    cd "$PROJECT_DIR"
    print_status "Stopping pg-metako services..."
    docker-compose down
    print_success "Services stopped successfully!"
}

# Function to show status
show_status() {
    cd "$PROJECT_DIR"
    print_status "Service status:"
    docker-compose ps
    
    print_status "Service logs (last 10 lines):"
    docker-compose logs --tail=10
}

# Function to show logs
show_logs() {
    cd "$PROJECT_DIR"
    if [ -n "$1" ]; then
        docker-compose logs -f "$1"
    else
        docker-compose logs -f
    fi
}

# Function to clean up
cleanup() {
    cd "$PROJECT_DIR"
    print_warning "This will remove all containers, volumes, and data!"
    read -p "Are you sure? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        print_status "Cleaning up Docker resources..."
        docker-compose down -v --remove-orphans
        docker system prune -f
        print_success "Cleanup completed!"
    else
        print_status "Cleanup cancelled."
    fi
}

# Function to run with pgAdmin
start_with_admin() {
    cd "$PROJECT_DIR"
    export COMPOSE_PROFILES=admin
    start_services
    docker-compose up -d pgadmin
    print_success "Services started with pgAdmin!"
    print_status "pgAdmin available at: http://localhost:8081"
}

# Function to show help
show_help() {
    echo "pg-metako Docker Deployment Script"
    echo ""
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  start       Start all services"
    echo "  stop        Stop all services"
    echo "  restart     Restart all services"
    echo "  status      Show service status"
    echo "  logs [svc]  Show logs (optionally for specific service)"
    echo "  build       Build Docker images"
    echo "  setup       Setup environment file"
    echo "  admin       Start services with pgAdmin"
    echo "  cleanup     Remove all containers and volumes"
    echo "  help        Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 setup     # Setup environment file"
    echo "  $0 start     # Start all services"
    echo "  $0 logs pg-metako  # Show pg-metako logs"
    echo "  $0 admin     # Start with pgAdmin"
}

# Main script logic
main() {
    check_docker
    check_docker_compose
    
    case "${1:-help}" in
        "setup")
            setup_env
            ;;
        "build")
            build_images
            ;;
        "start")
            setup_env
            build_images
            start_services
            ;;
        "stop")
            stop_services
            ;;
        "restart")
            stop_services
            setup_env
            build_images
            start_services
            ;;
        "status")
            show_status
            ;;
        "logs")
            show_logs "$2"
            ;;
        "admin")
            setup_env
            build_images
            start_with_admin
            ;;
        "cleanup")
            cleanup
            ;;
        "help"|*)
            show_help
            ;;
    esac
}

# Run main function with all arguments
main "$@"