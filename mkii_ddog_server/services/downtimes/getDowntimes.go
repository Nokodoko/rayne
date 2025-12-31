package downtimes

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/requests"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/urls"
)

// NOTE: TO ADD TO TEST (MAKESHIFT ASSERT FUNCTION)
func Must[T any](x T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return x
}

func GetDowntimes(w http.ResponseWriter, r *http.Request) (int, any) {
	// func GetDowntimes(w http.ResponseWriter, r *http.Request) {
	downtimes, _, err := requests.Get[DowntimesResponse](w, r, urls.GetDowntimesUrl)
	if err != nil {
		status := http.StatusInternalServerError
		utils.WriteError(w, status, err)
	}

	ids := []int{}
	scopes := []string{}

	for _, v := range downtimes.Data {
		id := v.Attributes.Monitor_identifier.MonitorID
		scope := v.Attributes.Scope

		ids = append(ids, id)
		scopes = append(scopes, scope)
		fmt.Println(ids, scopes)
	}

	data := combinedData{
		IDs:    ids,
		Scopes: scopes,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		status := http.StatusInternalServerError
		utils.WriteError(w, status, err)
	}

	utils.WriteJson(w, http.StatusOK, jsonData)
	return http.StatusOK, jsonData
}

// NOTE: remember to add mutex lock to make thread safe (rw mutexes)
