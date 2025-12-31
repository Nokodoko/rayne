package pl

type ContainerConfig struct {
	AutoRemove   bool              `json:"AutoRemove"`
	Command      []string          `json:"Command"`
	CreatedAt    string            `json:"CreatedAt"`
	CIDFile      string            `json:"CIDFile"`
	Exited       bool              `json:"Exited"`
	ExitedAt     int64             `json:"ExitedAt"`
	ExitCode     int               `json:"ExitCode"`
	ExposedPorts any               `json:"ExposedPorts"` // use interface{} for null
	Id           string            `json:"Id"`
	Image        string            `json:"Image"`
	ImageID      string            `json:"ImageID"`
	IsInfra      bool              `json:"IsInfra"`
	Labels       any               `json:"Labels"` // use interface{} for null
	Mounts       []any             `json:"Mounts"` // can be further defined if structure known
	Names        []string          `json:"Names"`
	Namespaces   map[string]string `json:"Namespaces"` // empty object -> empty map
	Networks     []any             `json:"Networks"`   // can be defined if structure known
	Pid          int               `json:"Pid"`
	Pod          string            `json:"Pod"`
	PodName      string            `json:"PodName"`
	Ports        any               `json:"Ports"` // use interface{} for null
	Restarts     int               `json:"Restarts"`
	Size         any               `json:"Size"` // use interface{} for null
	StartedAt    int64             `json:"StartedAt"`
	State        string            `json:"State"`
	Status       string            `json:"Status"`
	Created      int64             `json:"Created"`
}
