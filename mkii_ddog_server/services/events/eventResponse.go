package events

type EventsResponse struct {
	Data []struct {
		Attributes struct {
			Attributes struct {
				AggregationKey string `json:"aggregation_key"`
				DateHappened   int64  `json:"date_happened"`
				DeviceName     string `json:"device_name"`
				Duration       int64  `json:"duration"`
				EventObject    string `json:"event_object"`
				Evt            struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					SourceID int    `json:"source_id"`
					Type     string `json:"type"`
				} `json:"evt"`
				Hostname string `json:"hostname"`
				Monitor  struct {
					CreatedAt     int64    `json:"created_at"`
					GroupStatus   string   `json:"group_status"`
					Groups        []string `json:"groups"`
					ID            string   `json:"id"`
					Message       string   `json:"message"`
					Modified      string   `json:"modified"`
					Name          string   `json:"name"`
					Query         string   `json:"query"`
					Tags          []string `json:"tags"`
					TemplatedName string   `json:"templated_name"`
					Type          string   `json:"type"`
				} `json:"monitor"`
				MonitorGroups  []string `json:"monitor_groups"`
				MonitorID      string   `json:"monitor_id"`
				Priority       string   `json:"priority"`
				RelatedEventID string   `json:"related_event_id"`
				Service        string   `json:"service"`
				SourceTypeName string   `json:"source_type_name"`
				SourceCategory string   `json:"sourcecategory"`
				Status         string   `json:"status"`
				Tags           []string `json:"tags"`
				Timestamp      int64    `json:"timestamp"`
				Title          string   `json:"title"`
			} `json:"attributes"`
			Message   string   `json:"message"`
			Tags      []string `json:"tags"`
			Timestamp string   `json:"timestamp"`
		} `json:"attributes"`
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
	Meta struct {
		Elapsed int `json:"elapsed"`
		Page    struct {
			After string `json:"after"`
		} `json:"page"`
		RequestID string `json:"request_id"`
		Status    string `json:"status"`
		Warnings  []struct {
			Code   string `json:"code"`
			Detail string `json:"detail"`
			Title  string `json:"title"`
		} `json:"warnings"`
	} `json:"meta"`
}
