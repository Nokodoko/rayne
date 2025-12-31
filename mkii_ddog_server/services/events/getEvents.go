package events

import (
	"net/http"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/requests"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/urls"
)

func GetEvents(w http.ResponseWriter, r *http.Request) (int, any) {
	eventrequest, status, err := requests.Get[EventsResponse](w, r, urls.GetEvents)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	var events []string
	for _, v := range eventrequest.Data {
		events = append(events, v.Attributes.Message)
	}

	return status, events
}
