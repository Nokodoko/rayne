# agentic_instructions.md

## Purpose
Templates and prompts for incident report generation. Used by the Claude agent sidecar to structure RCA analysis into Datadog Notebooks and JSON incident reports.

## Technology
Markdown, JSON

## Contents
- `incident-report-cloned.md` -- Datadog Notebook incident report template with dd-widget placeholders for metric graphs, log timelines, and event timelines
- `incident_report.json` -- JSON template for structured RCA output with sections: summary, root_cause_analysis, impact_assessment, recommendations, similar_incidents
- `logs-analysis.md` -- Datadog Notebook template for log analysis with dd-widget definitions for log streams and timeseries
- `prompt.md` -- Brief prompt notes (ChatGPT link, CRUD features reference)

## Key Functions
N/A (static templates)

## Data Types
- `incident_report.json` structure: summary (title, severity, status, affected_services, timeline), root_cause_analysis (primary_cause, contributing_factors, evidence), impact_assessment, recommendations (immediate_actions, long_term_improvements), similar_incidents

## Logging
N/A

## CRUD Entry Points
- **Create**: Add new template files for different report formats
- **Read**: Templates are loaded by the Claude agent sidecar during /analyze
- **Update**: Modify templates to change report structure/sections
- **Delete**: Remove unused templates

## Style Guide
- Markdown templates use Datadog Notebook dd-widget syntax for embedded widgets
- JSON templates use placeholder values (e.g., "{{MONITOR_NAME}}", "{{TIMESTAMP}}")
