package user

import (
	"encoding/json"
	"net/http"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils"
	"github.com/Nokodoko/mkii_ddog_server/services/downtimes"
	"github.com/Nokodoko/mkii_ddog_server/services/events"
	"github.com/Nokodoko/mkii_ddog_server/services/hosts"
	"github.com/Nokodoko/mkii_ddog_server/services/pl"
)

type Handler struct {
	storage types.UserStorage
}

func NewHandler(storage types.UserStorage) *Handler {
	return &Handler{storage: storage}
}

func (h *Handler) RegisterRoutes(router *http.ServeMux) {
	// NOTE: USERS:
	utils.Endpoint(router, "POST", "/login", h.HandleLogin) //NOTE: LOGIN FOR BACKEND USAGE ONLY OTHERWISE FOR RUM NO LOGIN
	utils.Endpoint(router, "POST", "/register", h.HandleRegister)

	//NOTE: DOWNTIMES:
	utils.Endpoint(router, "GET", "/v1/downtimes", downtimes.GetDowntimes) //NOTE:-> only write to db if couple'd with webhook

	//NOTE: EVENTS:
	utils.Endpoint(router, "GET", "/v1/events", events.GetEvents)

	//NOTE:HOSTS:
	utils.Endpoint(router, "GET", "/v1/hosts/active", hosts.GetTotalActiveHosts)

	//NOTE:PRIVATELOCATIONSROTATION
	utils.EndpointWithPathParams(router, "GET", "/v1/pl/refresh/{name}", "name", pl.ImageRotation)

}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) (int, any) {
	var payload types.RegisterUserPayload

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body"}
	}

	user, err := h.storage.GetUserbyUUID(payload.UUID)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": "database error"}
	}

	if user.UUID == 0 {
		return http.StatusNotFound, map[string]string{"error": "user not found"}
	}

	return http.StatusOK, user
}

func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) (int, any) {
	var payload types.RegisterUserPayload

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body"}
	}

	// Check if user already exists
	existing, _ := h.storage.GetUserbyUUID(payload.UUID)
	if existing != nil && existing.UUID != 0 {
		return http.StatusConflict, map[string]string{"error": "user with UUID already exists"}
	}

	user, err := h.storage.CreateUser(payload.Name, payload.UUID)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": "failed to create user"}
	}

	return http.StatusCreated, user
}
