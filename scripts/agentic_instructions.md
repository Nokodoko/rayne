# agentic_instructions.md

## Purpose
Development and deployment scripts for the Rayne ecosystem: minikube setup, traffic generation, LLM monitoring, Cloudflare DNS, notification server, and project context generation.

## Technology
Bash, Python, Node.js, gum (interactive CLI)

## Contents
- `minikube-setup.sh` -- Interactive gum-based minikube setup with Terraform, kubectl, and secret management
- `notify-server.py` -- Python HTTP server for desktop notifications (receives webhook alerts)
- `cloudflare-dns-setup.sh` -- Cloudflare DNS record creation via API
- `frontend-traffic-generator.sh` -- Generates HTTP traffic to frontend for RUM testing
- `traffic-generator.sh` -- Generates HTTP traffic to Rayne API endpoints
- `headless-traffic-generator.js` -- Node.js Puppeteer-based headless browser traffic generator
- `deploy-llm-monitoring.sh` -- Deploys Datadog LLM monitoring configuration
- `test-llm-monitoring.sh` -- Tests LLM monitoring setup
- `ddtrace-llm-wrapper.py` -- Python wrapper for ddtrace LLM observability
- `ingest-project-readmes.py` -- Ingests project READMEs for context
- `project-context.md` -- Generated project context document
- `HEADLESS_FIX_PLAN.md` -- Fix plan for headless traffic generator
- `requirements.txt` -- Python dependencies
- `package.json` / `package-lock.json` -- Node.js dependencies for headless generator

## Key Functions
- `minikube-setup.sh`: Interactive prompts for cluster configuration, secret creation, deployment
- `notify-server.py`: HTTP POST handler that triggers desktop notifications
- `traffic-generator.sh`: Curl-based load generation for API endpoints

## Data Types
N/A

## Logging
Varies by script (echo, print, console.log)

## CRUD Entry Points
- **Create**: Add new .sh/.py/.js scripts for automation tasks
- **Read**: Execute scripts directly: `./scripts/minikube-setup.sh`
- **Update**: Modify scripts for different environments/configurations
- **Delete**: Remove unused scripts

## Style Guide
- Bash scripts use gum for interactive prompts where applicable
- Python scripts use argparse or env vars for configuration
- Node.js scripts use package.json for dependency management
