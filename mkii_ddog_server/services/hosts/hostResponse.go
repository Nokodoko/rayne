package hosts

type HostListResponse struct {
	HostList []struct {
		Aliases          []string `json:"aliases"`
		Apps             []string `json:"apps"`
		AWSName          string   `json:"aws_name"`
		HostName         string   `json:"host_name"`
		ID               int      `json:"id"`
		IsMuted          bool     `json:"is_muted"`
		LastReportedTime int64    `json:"last_reported_time"`
		Meta             struct {
			AgentChecks   []string `json:"agent_checks"`
			AgentVersion  string   `json:"agent_version"`
			CpuCores      int      `json:"cpuCores"`
			FbsdV         []string `json:"fbsdV"`
			Gohai         string   `json:"gohai"`
			InstallMethod struct {
				InstallerVersion string `json:"installer_version"`
				Tool             string `json:"tool"`
				ToolVersion      string `json:"tool_version"`
			} `json:"install_method"`
			MacV           []string `json:"macV"`
			Machine        string   `json:"machine"`
			NixV           []string `json:"nixV"`
			Platform       string   `json:"platform"`
			Processor      string   `json:"processor"`
			PythonV        string   `json:"pythonV"`
			SocketFQDN     string   `json:"socket-fqdn"`
			SocketHostname string   `json:"socket-hostname"`
			WinV           []string `json:"winV"`
		} `json:"meta"`
		Metrics struct {
			CPU    int     `json:"cpu"`
			IOWait float64 `json:"iowait"`
			Load   float64 `json:"load"`
		} `json:"metrics"`
		MuteTimeout  string              `json:"mute_timeout"`
		Name         string              `json:"name"`
		Sources      []string            `json:"sources"`
		TagsBySource map[string][]string `json:"tags_by_source"`
		Up           bool                `json:"up"`
	} `json:"host_list"`
	TotalMatching int `json:"total_matching"`
	TotalReturned int `json:"total_returned"`
}
