package hosts

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/requests"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/urls"
)

func GetTotalActiveHosts(w http.ResponseWriter, r *http.Request) (int, any) {
	activeHosts, _, err := requests.Get[ActiveHostsResponse](w, r, urls.GetTotalActiveHosts)
	if err != nil {
		status := http.StatusInternalServerError
		utils.WriteError(w, status, err)
	}

	active := activeHosts.Total_active
	up := activeHosts.Total_up
	status := http.StatusOK
	data := fmt.Sprintf("Active: %d, Up: %d", active, up)
	log.Printf(data)
	utils.WriteJson(w, status, data)
	return status, data
}
