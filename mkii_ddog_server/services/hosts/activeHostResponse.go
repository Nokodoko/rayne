package hosts

type ActiveHostsResponse struct {
	Total_active int `json:"total_active"`
	Total_up     int `json:"total_up"`
}
