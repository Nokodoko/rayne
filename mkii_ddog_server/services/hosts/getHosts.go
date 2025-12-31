package hosts

import (
	"net/http"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/requests"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/urls"
)

func GetHosts(w http.ResponseWriter, r *http.Request) (int, any) {
	hosts, status, err := requests.Get[HostListResponse](w, r, urls.GetHosts)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	var listHosts []string
	for _, v := range hosts.HostList {
		listHosts = append(listHosts, v.HostName)
	}

	return status, listHosts
}

func GetHostsHelper(w http.ResponseWriter, r *http.Request) []string {
	hostHelper, _, err := requests.Get[HostListResponse](w, r, urls.GetHosts)
	if err != nil {
		return nil
	}

	var listHosts []string
	for _, v := range hostHelper.HostList {
		listHosts = append(listHosts, v.HostName)
	}

	return listHosts
}
