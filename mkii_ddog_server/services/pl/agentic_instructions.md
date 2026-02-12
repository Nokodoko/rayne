# agentic_instructions.md

## Purpose
Private Location container image management. Manages Datadog Synthetics private location worker containers via podman and systemd service actions (stop, pull, remove, restart).

## Technology
Go, os/exec, net/http, encoding/json

## Contents
- `imagerotate.go` -- ImageRotation HTTP handler (switch on action name)
- `utils.go` -- runner(), podmanPullImage(), ImageRemover(), serviceActions(), container helpers
- `containerResponse.go` -- ContainerConfig type (podman container metadata)
- `imageError.go` -- ImageError custom error type

## Key Functions
- `ImageRotation(w, r, name) (int, any)` -- HTTP handler dispatching stop/pull/remove/pp/restart actions
- `serviceActions(action, name) (int, any)` -- Executes systemd service commands (stop, restart)
- `podmanPullImage() (int, any, error)` -- Pulls and runs private location worker container
- `ImageRemover() (int, any, error)` -- Force removes stale private location images
- `runner(cmd, errMsg) (int, any, error)` -- Generic command executor with error wrapping

## Data Types
- `ContainerConfig` -- struct: detailed podman container metadata (27 fields including AutoRemove, Command, Image, State, etc.)
- `ImageError` -- struct: Msg, ReturnCode
- `IMAGE` -- package var: "gcr.io/datadoghq/synthetics-private-location-worker:latest"

## Logging
Uses `log.Fatalf`, `log.Println`, `fmt.Println` for command output

## CRUD Entry Points
- **Create**: N/A
- **Read**: N/A
- **Update**: Call ImageRotation with "pull", "pp" (push-pull), or "restart"
- **Delete**: Call ImageRotation with "stop" or "remove"

## Style Guide
- Shell command execution via os/exec
- Switch-case dispatch for action routing
- Dead code sections marked with `// NOTE: dead code` comments
- Representative snippet:

```go
func ImageRotation(w http.ResponseWriter, r *http.Request, name string) (int, any) {
	switch name {
	case "stop":
		_, err := serviceActions("stop ", name)
		if err != nil {
			return http.StatusInternalServerError, nil
		}
	case "pull":
		_, err := serviceActions("pull", IMAGE)
		if err != nil {
			return http.StatusInternalServerError, nil
		}
	}
	return http.StatusOK, nil
}
```
