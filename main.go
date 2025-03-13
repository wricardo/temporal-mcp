package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Configure logging format (include date, time, source file)
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC | log.Lshortfile)

	// Read Temporal connection settings from environment
	temporalAddress := os.Getenv("TEMPORAL_ADDRESS")
	temporalNamespace := os.Getenv("TEMPORAL_NAMESPACE")
	if temporalAddress == "" {
		temporalAddress = "localhost:7233"
	}
	if temporalNamespace == "" {
		temporalNamespace = "default"
	}

	// Connect to Temporal server
	c, err := client.Dial(client.Options{
		HostPort:  temporalAddress,
		Namespace: temporalNamespace,
	})
	if err != nil {
		log.Fatalf("Unable to connect to Temporal at %s (namespace %s): %v", temporalAddress, temporalNamespace, err)
	}
	defer c.Close()
	log.Printf("Connected to Temporal at %s (namespace: %s)", temporalAddress, temporalNamespace)

	// Create the MCP server instance
	mcpServer := server.NewMCPServer("temporal-mcp", "1.0.0")

	// Define the "list_workflows" tool
	listWorkflowsTool := mcp.NewTool(
		"list_workflows",
		mcp.WithDescription("List Temporal workflows filtered by status (running, completed, or failed)"),
		mcp.WithString("status",
			mcp.Required(),
			mcp.Description("Workflow status to filter by (running, completed, failed)"),
		),
	)

	// Define the "describe_workflow" tool
	describeWorkflowTool := mcp.NewTool(
		"describe_workflow",
		mcp.WithDescription("Retrieve detailed information about a specific workflow execution"),
		mcp.WithString("workflow_id",
			mcp.Required(),
			mcp.Description("Workflow ID of the execution to describe"),
		),
		mcp.WithString("run_id",
			mcp.Description("Optional Run ID (if not provided, the latest run is used)"),
		),
	)

	// Register the "list_workflows" tool with its handler
	mcpServer.AddTool(listWorkflowsTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Validate and retrieve the status parameter
		statusVal, ok := req.Params.Arguments["status"].(string)
		if !ok || statusVal == "" {
			return mcp.NewToolResultError("Missing or invalid 'status' parameter"), nil
		}
		statusFilter := strings.ToLower(statusVal)

		// Prepare list request based on the status filter
		var executions []*workflowpb.WorkflowExecutionInfo
		if statusFilter == "running" {
			// List open (running) workflows
			resp, err := c.ListOpenWorkflow(ctx, &workflowservice.ListOpenWorkflowExecutionsRequest{
				Namespace:       temporalNamespace,
				MaximumPageSize: 100, // limit results for performance
			})
			if err != nil {
				log.Printf("Error listing running workflows: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to list running workflows: %v", err)), nil
			}
			executions = resp.GetExecutions()
		} else if statusFilter == "completed" || statusFilter == "failed" {
			// List closed workflows filtered by close status (Completed or Failed)
			// var closeStatus enumspb.WorkflowExecutionStatus
			// if statusFilter == "completed" {
			// 	closeStatus = enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED
			// } else {
			// 	closeStatus = enumspb.WORKFLOW_EXECUTION_STATUS_FAILED
			// }
			resp, err := c.ListClosedWorkflow(ctx, &workflowservice.ListClosedWorkflowExecutionsRequest{
				Namespace:       temporalNamespace,
				MaximumPageSize: 100,
				Filters:         &workflowservice.ListClosedWorkflowExecutionsRequest_StatusFilter{
					// StatusFilter: &filterpb.WorkflowExecutionCloseStatusFilter{Status: closeStatus},
				},
			})
			if err != nil {
				log.Printf("Error listing %s workflows: %v", statusFilter, err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to list %s workflows: %v", statusFilter, err)), nil
			}
			executions = resp.GetExecutions()
		} else {
			// Unsupported status filter
			return mcp.NewToolResultError(fmt.Sprintf("Unsupported status '%s' (use running, completed, or failed)", statusVal)), nil
		}

		// Build the output based on retrieved executions
		if len(executions) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No %s workflows found.", statusFilter)), nil
		}
		var outputBuilder strings.Builder
		outputBuilder.WriteString(fmt.Sprintf("Found %d %s workflow(s):\n", len(executions), statusFilter))
		for _, info := range executions {
			id := info.GetExecution().GetWorkflowId()
			runID := info.GetExecution().GetRunId()
			wfType := info.GetType().GetName()
			start := info.GetStartTime().AsTime().UTC().Format(time.RFC3339)
			statusStr := workflowStatusToString(info.GetStatus())
			if info.GetCloseTime() != nil {
				end := info.GetCloseTime().AsTime().UTC().Format(time.RFC3339)
				outputBuilder.WriteString(
					fmt.Sprintf("- ID: %s | Run: %s | Type: %s | Status: %s | Start: %s | End: %s\n",
						id, runID, wfType, statusStr, start, end),
				)
			} else {
				outputBuilder.WriteString(
					fmt.Sprintf("- ID: %s | Run: %s | Type: %s | Status: %s | Start: %s\n",
						id, runID, wfType, statusStr, start),
				)
			}
		}
		return mcp.NewToolResultText(outputBuilder.String()), nil
	})

	// Register the "describe_workflow" tool with its handler
	mcpServer.AddTool(describeWorkflowTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Validate and get required workflow_id
		wfID, ok := req.Params.Arguments["workflow_id"].(string)
		if !ok || wfID == "" {
			return mcp.NewToolResultError("Missing or invalid 'workflow_id' parameter"), nil
		}
		// Get optional run_id (may be empty if not provided)
		runID, _ := req.Params.Arguments["run_id"].(string)

		// Describe the workflow execution via Temporal
		resp, err := c.DescribeWorkflowExecution(ctx, wfID, runID)
		if err != nil {
			log.Printf("Error describing workflow %q (run %q): %v", wfID, runID, err)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to describe workflow: %v", err)), nil
		}
		info := resp.GetWorkflowExecutionInfo()
		if info == nil {
			// This case is unlikely if no error, but handle defensively
			return mcp.NewToolResultError("No information available for the specified workflow"), nil
		}

		// Extract fields from WorkflowExecutionInfo
		id := info.GetExecution().GetWorkflowId()
		run := info.GetExecution().GetRunId()
		wfType := info.GetType().GetName()
		statusStr := workflowStatusToString(info.GetStatus())
		startTime := ""
		if info.GetStartTime() != nil {
			startTime = info.GetStartTime().AsTime().UTC().Format(time.RFC3339)
		}
		endTime := ""
		if info.GetCloseTime() != nil {
			endTime = info.GetCloseTime().AsTime().UTC().Format(time.RFC3339)
		}

		// Format the details into a multi-line output
		var outputBuilder strings.Builder
		outputBuilder.WriteString("Workflow Execution Details:\n")
		outputBuilder.WriteString(fmt.Sprintf("Workflow ID: %s\n", id))
		outputBuilder.WriteString(fmt.Sprintf("Run ID: %s\n", run))
		outputBuilder.WriteString(fmt.Sprintf("Type: %s\n", wfType))
		outputBuilder.WriteString(fmt.Sprintf("Status: %s\n", statusStr))
		outputBuilder.WriteString(fmt.Sprintf("Start Time: %s\n", startTime))
		if endTime != "" {
			outputBuilder.WriteString(fmt.Sprintf("End Time: %s\n", endTime))
		}
		return mcp.NewToolResultText(outputBuilder.String()), nil
	})

	// Start the MCP server (listening on STDIO for tool requests)
	log.Println("Starting temporal-mcp server...")
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}

// workflowStatusToString converts a WorkflowExecutionStatus enum to a readable string.
func workflowStatusToString(status enumspb.WorkflowExecutionStatus) string {
	switch status {
	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING:
		return "Running"
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		return "Completed"
	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED:
		return "Failed"
	case enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return "Canceled"
	case enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED:
		return "Terminated"
	case enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return "ContinuedAsNew"
	case enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return "TimedOut"
	default:
		return "Unknown"
	}
}
