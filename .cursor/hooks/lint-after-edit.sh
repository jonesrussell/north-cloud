#!/bin/bash

# lint-after-edit.sh - Cursor hook to lint files after they're edited
# This hook automatically runs linting on edited files based on their type

# Read JSON input from stdin
input=$(cat)

# Extract file path from the JSON input
file_path=$(echo "$input" | jq -r '.file_path // empty')

# If no file_path, exit silently (shouldn't happen, but be safe)
if [ -z "$file_path" ] || [ "$file_path" = "null" ]; then
    exit 0
fi

# Get the project root (assumes we're in /home/jones/dev/north-cloud)
project_root="/home/jones/dev/north-cloud"

# Convert absolute path to relative path from project root
relative_path="${file_path#$project_root/}"

# Skip if file is not in the project root
if [ "$relative_path" = "$file_path" ]; then
    exit 0
fi

# Determine which service directory this file belongs to
service_dir=""
if [[ "$relative_path" =~ ^(auth|classifier|crawler|index-manager|mcp-north-cloud|publisher|search|source-manager)/ ]]; then
    # Extract service name from path
    service_dir=$(echo "$relative_path" | cut -d'/' -f1)
elif [[ "$relative_path" =~ ^(dashboard|search-frontend)/ ]]; then
    service_dir=$(echo "$relative_path" | cut -d'/' -f1)
fi

# Skip if we couldn't determine the service
if [ -z "$service_dir" ]; then
    exit 0
fi

# Skip linting for certain file types that don't need linting
file_extension="${file_path##*.}"
case "$file_extension" in
    md|txt|json|yml|yaml|sql|sh|log|lock|sum)
        exit 0
        ;;
esac

# Skip if file is in certain directories
if [[ "$relative_path" =~ ^(node_modules|\.git|\.vscode|\.cursor|bin|tmp|vendor|dist|build|coverage|\.next)/ ]]; then
    exit 0
fi

# Determine file type and run appropriate linter
if [[ "$file_path" =~ \.(go)$ ]]; then
    # Go file - run golangci-lint for the specific service
    cd "$project_root/$service_dir" || exit 0
    
    # Only lint if Taskfile.yml exists (indicates it's a Go service with lint task)
    if [ -f "Taskfile.yml" ]; then
        # Run lint in background to avoid blocking, but capture output
        task lint > /tmp/cursor-lint-${service_dir}.log 2>&1 &
    fi
    
elif [[ "$file_path" =~ \.(vue|ts|js|tsx|jsx)$ ]] || [[ "$relative_path" =~ ^(dashboard|search-frontend)/ ]]; then
    # Frontend file - run npm lint for the specific service
    cd "$project_root/$service_dir" || exit 0
    
    # Only lint if package.json exists
    if [ -f "package.json" ]; then
        # Run lint in background to avoid blocking
        npm run lint > /tmp/cursor-lint-${service_dir}.log 2>&1 &
    fi
fi

# Always exit successfully to not block the agent
exit 0
