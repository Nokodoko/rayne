# agentic_instructions.md

## Purpose
Datadog Service Catalog API proxy. Lists service definitions and creates new ones with both simple and advanced request formats.

## Technology
Go, net/http, encoding/json, fmt

## Contents
- `handlers.go` -- ListServices, CreateServiceDefinition, CreateServiceDefinitionAdvanced
- `types.go` -- ServiceDefinitionRequest, SimpleServiceRequest, response types (ServiceData, ServiceAttrs, etc.)

## Key Functions
- `ListServices(w, r) (int, any)` -- Retrieves all service definitions from Datadog catalog
- `CreateServiceDefinition(w, r) (int, any)` -- Simple service creation with auto-generated tags from fields
- `CreateServiceDefinitionAdvanced(w, r) (int, any)` -- Full control over Datadog service definition API

## Data Types
- `ServiceDefinitionRequest` -- struct: SchemaVersion, DDService, Type, Languages, Tags, Description, Team, Contacts, Links, Repos, Docs
- `SimpleServiceRequest` -- struct: Name, Language, Type, Platform, Serverless, Host, URL, Env, APM, RUM, Active, Version, Owner, CostCenter, Department, Team
- `ServiceDefinitionResponse` -- struct: Data []ServiceData
- `ServiceData` -- struct: ID, Type, Attributes (Schema, Meta)
- `Contact`, `Link`, `Repo`, `Doc` -- struct: supporting types for service metadata

## Logging
None

## CRUD Entry Points
- **Create**: POST with SimpleServiceRequest (auto-tagged) or ServiceDefinitionRequest (full control)
- **Read**: GET via ListServices
- **Update**: N/A (not yet implemented)
- **Delete**: N/A (not yet implemented)

## Style Guide
- Schema version defaults to "v2.2" for Datadog Service Catalog
- Tags auto-generated from SimpleServiceRequest fields (platform:X, serverless:true, env:X, etc.)
- Representative snippet:

```go
func CreateServiceDefinition(w http.ResponseWriter, r *http.Request) (int, any) {
	var req SimpleServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body: " + err.Error()}
	}

	tags := []string{}
	if req.Platform != "" {
		tags = append(tags, fmt.Sprintf("platform:%s", req.Platform))
	}
	// ... build tags from fields

	ddReq := ServiceDefinitionRequest{
		SchemaVersion: "v2.2",
		DDService:     req.Name,
		Type:          req.Type,
		Languages:     []string{req.Language},
		Tags:          tags,
	}
	result, status, err := requests.Post[ServiceDefinitionResponse](w, r, urls.ServiceDefinitions, ddReq)
	return status, result
}
```
