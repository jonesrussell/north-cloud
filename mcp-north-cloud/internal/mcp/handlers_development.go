package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Development tool handlers

const taskTest = "test"

func (s *Server) handleLintFile(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		FilePath    string `json:"file_path"`
		ServiceName string `json:"service_name"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	projectRoot := s.detectProjectRoot()
	if projectRoot == "" {
		return s.errorResponse(id, InvalidParams, "Could not determine project root")
	}

	var serviceDir string
	var lintCommand *exec.Cmd
	var lintType string
	var setupErr error

	if args.ServiceName != "" {
		serviceDir, lintCommand, lintType, setupErr = s.setupServiceLint(ctx, projectRoot, args.ServiceName)
	} else if args.FilePath != "" {
		serviceDir, lintCommand, lintType, setupErr = s.setupFileLint(ctx, projectRoot, args.FilePath)
	} else {
		return s.errorResponse(id, InvalidParams, "Either file_path or service_name must be provided")
	}

	if setupErr != nil {
		return s.errorResponse(id, InvalidParams, setupErr.Error())
	}

	return s.executeLintCommand(id, lintCommand, lintType, serviceDir)
}

// detectProjectRoot determines the project root directory
func (s *Server) detectProjectRoot() string {
	projectRoot := "/home/jones/dev/north-cloud"
	if _, statErr := os.Stat(projectRoot); os.IsNotExist(statErr) {
		cwd, getwdErr := os.Getwd()
		if getwdErr != nil {
			return ""
		}
		for {
			if _, statErr2 := os.Stat(filepath.Join(cwd, ".cursor")); statErr2 == nil {
				return cwd
			}
			parent := filepath.Dir(cwd)
			if parent == cwd {
				break
			}
			cwd = parent
		}
		return ""
	}
	return projectRoot
}

// setupServiceLint configures linting for an entire service
func (s *Server) setupServiceLint(ctx context.Context, projectRoot, serviceName string) (
	serviceDir string, lintCommand *exec.Cmd, lintType string, err error,
) {
	serviceDir = filepath.Join(projectRoot, serviceName)
	if _, statErr := os.Stat(filepath.Join(serviceDir, "package.json")); statErr == nil {
		cmd := exec.CommandContext(ctx, "npm", "run", "lint")
		cmd.Dir = serviceDir
		return serviceDir, cmd, "frontend", nil
	}

	if _, statErr := os.Stat(filepath.Join(serviceDir, "Taskfile.yml")); statErr == nil {
		cmd := exec.CommandContext(ctx, "task", "lint")
		cmd.Dir = serviceDir
		return serviceDir, cmd, "go", nil
	}

	return "", nil, "", fmt.Errorf("service '%s' not found or doesn't have a lint configuration", serviceName)
}

// setupFileLint configures linting for a specific file
func (s *Server) setupFileLint(ctx context.Context, projectRoot, filePath string) (
	serviceDir string, lintCommand *exec.Cmd, lintType string, err error,
) {
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(projectRoot, filePath)
	}

	relPath, relErr := filepath.Rel(projectRoot, filePath)
	if relErr != nil {
		return "", nil, "", fmt.Errorf("file path '%s' is not within project root", filePath)
	}

	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) == 0 {
		return "", nil, "", errors.New("could not determine service from file path")
	}

	serviceDir = filepath.Join(projectRoot, parts[0])
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		return s.setupGoFileLint(ctx, serviceDir, parts[0])
	case ".vue", ".ts", ".js", ".tsx", ".jsx":
		return s.setupFrontendFileLint(ctx, serviceDir, parts[0])
	default:
		return "", nil, "", fmt.Errorf(
			"file type '%s' is not supported for linting. Supported: .go, .vue, .ts, .js, .tsx, .jsx",
			ext,
		)
	}
}

// setupGoFileLint configures linting for a Go file
func (s *Server) setupGoFileLint(ctx context.Context, serviceDir, serviceName string) (
	dir string, lintCommand *exec.Cmd, lintType string, err error,
) {
	if _, statErr := os.Stat(filepath.Join(serviceDir, "Taskfile.yml")); statErr == nil {
		cmd := exec.CommandContext(ctx, "task", "lint")
		cmd.Dir = serviceDir
		return serviceDir, cmd, "go", nil
	}
	return "", nil, "", fmt.Errorf("service directory '%s' doesn't have a Taskfile.yml for linting", serviceName)
}

// setupFrontendFileLint configures linting for a frontend file
func (s *Server) setupFrontendFileLint(ctx context.Context, serviceDir, serviceName string) (
	dir string, lintCommand *exec.Cmd, lintType string, err error,
) {
	if _, statErr := os.Stat(filepath.Join(serviceDir, "package.json")); statErr == nil {
		cmd := exec.CommandContext(ctx, "npm", "run", "lint")
		cmd.Dir = serviceDir
		return serviceDir, cmd, "frontend", nil
	}
	return "", nil, "", fmt.Errorf("service directory '%s' doesn't have a package.json for linting", serviceName)
}

// executeLintCommand runs the lint command and returns the response
func (s *Server) executeLintCommand(id any, lintCommand *exec.Cmd, lintType, serviceDir string) *Response {
	output, err := lintCommand.CombinedOutput()
	outputStr := string(output)
	displayOutput, outputTruncated, outputTotalBytes := truncateCommandOutput(outputStr, maxCommandOutputBytes)

	result := map[string]any{
		"lint_type":   lintType,
		"service_dir": serviceDir,
		"command":     strings.Join(lintCommand.Args, " "),
		"output":      displayOutput,
		"success":     err == nil,
	}
	if outputTruncated {
		result["output_truncated"] = true
		result["output_total_bytes"] = outputTotalBytes
	}
	if err != nil {
		result["error"] = err.Error()
		if lintCommand.ProcessState != nil {
			result["exit_code"] = lintCommand.ProcessState.ExitCode()
		} else {
			result["exit_code"] = -1
		}
	}

	return s.successResponse(id, result)
}

func (s *Server) handleBuildService(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		ServiceName string `json:"service_name"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}
	if args.ServiceName == "" {
		return s.errorResponse(id, InvalidParams, "service_name is required")
	}
	return s.executeBuildTestCommand(ctx, id, args.ServiceName, "build", false)
}

func (s *Server) handleTestService(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		ServiceName  string `json:"service_name"`
		WithCoverage bool   `json:"with_coverage"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}
	if args.ServiceName == "" {
		return s.errorResponse(id, InvalidParams, "service_name is required")
	}
	taskName := taskTest
	if args.WithCoverage {
		taskName = "test:coverage"
	}
	return s.executeBuildTestCommand(ctx, id, args.ServiceName, taskName, true)
}

// maxCommandOutputBytes caps the output field size in build/test/lint responses to avoid large MCP payloads.
const maxCommandOutputBytes = 6000

// truncateCommandOutput returns output truncated to the last maxBytes characters when over limit.
// It preserves the tail so failure messages and stack traces remain visible.
// truncationNoteReserve is the number of bytes reserved for the truncation note prefix.
const truncationNoteReserve = 60

// Returns (possibly truncated output, wasTruncated, original length in bytes).
func truncateCommandOutput(s string, maxBytes int) (out string, truncated bool, totalBytes int) {
	totalBytes = len(s)
	if totalBytes <= maxBytes {
		return s, false, totalBytes
	}
	tailStart := totalBytes - (maxBytes - truncationNoteReserve)
	if tailStart < 0 {
		tailStart = 0
	}
	note := fmt.Sprintf("... (output truncated, showing last %d of %d bytes)\n", totalBytes-tailStart, totalBytes)
	return note + s[tailStart:], true, totalBytes
}

// executeBuildTestCommand runs build or test for a service and returns structured output.
func (s *Server) executeBuildTestCommand(ctx context.Context, id any, serviceName, taskName string, isTest bool) *Response {
	projectRoot := s.detectProjectRoot()
	if projectRoot == "" {
		return s.errorResponse(id, InvalidParams, "Could not determine project root")
	}
	serviceDir := filepath.Join(projectRoot, serviceName)

	var cmd *exec.Cmd
	var isGo bool
	if _, statErr := os.Stat(filepath.Join(serviceDir, "Taskfile.yml")); statErr == nil {
		_, goModErr := os.Stat(filepath.Join(serviceDir, "go.mod"))
		isGo = (goModErr == nil)
		// Frontend services (dashboard, search-frontend) have Taskfile but no test:coverage
		runTask := taskName
		if taskName == "test:coverage" && !isGo {
			runTask = taskTest
		}
		cmd = exec.CommandContext(ctx, "task", runTask)
		cmd.Dir = serviceDir
	} else if _, pkgJSONErr := os.Stat(filepath.Join(serviceDir, "package.json")); pkgJSONErr == nil {
		isGo = false
		script := "build"
		if isTest {
			script = taskTest
		}
		cmd = exec.CommandContext(ctx, "npm", "run", script)
		cmd.Dir = serviceDir
	} else {
		return s.errorResponse(id, InvalidParams,
			fmt.Sprintf("service '%s' not found or doesn't have Taskfile.yml or package.json", serviceName))
	}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	displayOutput, outputTruncated, outputTotalBytes := truncateCommandOutput(outputStr, maxCommandOutputBytes)

	result := map[string]any{
		"success": err == nil,
		"service": serviceName,
		"command": strings.Join(cmd.Args, " "),
		"output":  displayOutput,
	}
	if outputTruncated {
		result["output_truncated"] = true
		result["output_total_bytes"] = outputTotalBytes
	}
	populateBuildTestErrorResult(result, cmd, err, outputStr, isGo)

	return s.successResponse(id, result)
}

func populateBuildTestErrorResult(result map[string]any, cmd *exec.Cmd, cmdErr error, outputStr string, isGo bool) {
	if cmdErr == nil {
		return
	}
	result["error"] = cmdErr.Error()
	if cmd.ProcessState != nil {
		result["exit_code"] = cmd.ProcessState.ExitCode()
	} else {
		result["exit_code"] = -1
	}
	if isGo {
		if parsed := parseGoErrors(outputStr); len(parsed) > 0 {
			result["errors"] = parsed
		}
	}
}

// goError represents a parsed Go compiler or test error.
type goError struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column,omitempty"`
	Message string `json:"message"`
}

// parseGoErrors extracts file:line:column: message patterns from Go build/test output.
func parseGoErrors(output string) []goError {
	// Matches: path/to/file.go:42:5: message or path/to/file.go:42: message
	re := regexp.MustCompile(`(?m)^([^:]+):(\d+)(?::(\d+))?:\s*(.+)$`)
	matches := re.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}
	errs := make([]goError, 0, len(matches))
	for _, m := range matches {
		e := goError{File: m[1], Message: m[4]}
		if line, err := strconv.Atoi(m[2]); err == nil {
			e.Line = line
		}
		if m[3] != "" {
			if col, colErr := strconv.Atoi(m[3]); colErr == nil {
				e.Column = col
			}
		}
		errs = append(errs, e)
	}
	return errs
}
