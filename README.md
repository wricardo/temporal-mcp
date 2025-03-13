# MCP Temporal

This project is a **Model Context Protocol (MCP) server** designed to interact with Temporal.io services using the official Temporal Go SDK. It allows users to **list workflows** (filtered by running, completed, or failed statuses) and **retrieve detailed workflow execution information**.

---

## 🚀 Features

✅ **List Workflows**: Retrieve a list of Temporal workflows filtered by status (running, completed, or failed).  
✅ **Describe Workflow**: Get detailed information about a specific workflow execution, including **ID, Run ID, Type, Status, and Timestamps**.

---

## 📋 Requirements

- **Go** 1.23.0 or later
- **Access to a running Temporal server**

---

## ⚙️ Setup

### 1️⃣ Install the Package
```bash
go install github.com/wricardo/temporal-mcp@latest
```

### 2️⃣ Configure Environment Variables
Set the following environment variables to connect to your Temporal server:
```bash
export TEMPORAL_ADDRESS="localhost:7233"
export TEMPORAL_NAMESPACE="default"
```

### 3️⃣ Configure MCP Client Settings
Add the following configuration to your MCP settings:
```json
"temporal-mcp": {
  "command": "temporal-mcp",
  "env": {
    "TEMPORAL_ADDRESS": "localhost:7233",
    "TEMPORAL_NAMESPACE": "default"
  },
  "disabled": false,
  "autoApprove": []
}
```

---

## ▶️ Usage
Run the MCP server:
```bash
temporal-mcp
```

---

## 🛠️ Tools

### 🔹 **list_workflows**
Retrieve a list of workflows from the Temporal server filtered by status.

#### 📌 Parameters:
- `status` (**required**): Filter workflows by status (`running`, `completed`, `failed`).

### 🔹 **describe_workflow**
Retrieve detailed information about a specific workflow execution.

#### 📌 Parameters:
- `workflow_id` (**required**): The ID of the workflow to describe.
- `run_id` (**optional**): The run ID of the workflow. If omitted, the latest run is used.

---

## 📖 Notes
This README outlines the project’s purpose, key features, setup instructions, and usage details. It should help users quickly understand how to install, configure, and interact with your **temporal-mcp** server.

Let me know if you’d like any adjustments or further details! 🚀

