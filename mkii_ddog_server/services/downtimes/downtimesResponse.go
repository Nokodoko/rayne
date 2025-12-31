package downtimes

type DowntimesResponse struct {
	Data []struct {
		Attributes struct {
			Monitor_identifier struct {
				MonitorID int `json:"monitor_id"`
			} `json:"monitor_identifier"`
			Scope string `json:"scope"`
		} `json:"attributes"`
	} `json:"data"`
}

type combinedData struct {
	IDs    []int    `json:"ids"`
	Scopes []string `json:"scopes"`
}
