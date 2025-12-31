package catalog

// ServiceDefinitionRequest represents a service definition creation request
type ServiceDefinitionRequest struct {
	SchemaVersion string   `json:"schema-version"`
	DDService     string   `json:"dd-service"`
	Type          string   `json:"type"`
	Languages     []string `json:"languages"`
	Tags          []string `json:"tags"`
	Description   string   `json:"description,omitempty"`
	Team          string   `json:"team,omitempty"`
	Contacts      []Contact `json:"contacts,omitempty"`
	Links         []Link    `json:"links,omitempty"`
	Repos         []Repo    `json:"repos,omitempty"`
	Docs          []Doc     `json:"docs,omitempty"`
}

// Contact represents a service contact
type Contact struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Contact string `json:"contact"`
}

// Link represents a service link
type Link struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Repo represents a service repository
type Repo struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	URL      string `json:"url"`
}

// Doc represents a service documentation link
type Doc struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	URL      string `json:"url"`
}

// ServiceDefinitionResponse represents the response from creating a service
type ServiceDefinitionResponse struct {
	Data []ServiceData `json:"data"`
}

// ServiceData represents service data in response
type ServiceData struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Attributes ServiceAttrs     `json:"attributes"`
}

// ServiceAttrs represents service attributes
type ServiceAttrs struct {
	Schema     ServiceSchema `json:"schema"`
	Meta       ServiceMeta   `json:"meta"`
}

// ServiceSchema represents the service schema
type ServiceSchema struct {
	SchemaVersion string   `json:"schema-version"`
	DDService     string   `json:"dd-service"`
	Type          string   `json:"type"`
	Languages     []string `json:"languages"`
	Tags          []string `json:"tags"`
}

// ServiceMeta represents service metadata
type ServiceMeta struct {
	LastModifiedTime string `json:"last-modified-time"`
	Origin           string `json:"origin"`
	OriginDetail     string `json:"origin-detail"`
}

// ServiceListResponse represents the list of services
type ServiceListResponse struct {
	Data []ServiceData `json:"data"`
}

// SimpleServiceRequest is a simplified request for creating services
type SimpleServiceRequest struct {
	Name       string   `json:"name"`
	Language   string   `json:"language"`
	Type       string   `json:"type,omitempty"`
	Platform   string   `json:"platform,omitempty"`
	Serverless bool     `json:"serverless,omitempty"`
	Host       string   `json:"host,omitempty"`
	URL        string   `json:"url,omitempty"`
	Env        string   `json:"env,omitempty"`
	APM        bool     `json:"apm,omitempty"`
	RUM        bool     `json:"rum,omitempty"`
	Active     bool     `json:"active,omitempty"`
	Version    string   `json:"version,omitempty"`
	Owner      string   `json:"owner,omitempty"`
	CostCenter string   `json:"cost_center,omitempty"`
	Department string   `json:"department,omitempty"`
	Team       string   `json:"team,omitempty"`
}
