package user

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
)

type mockUserStorage struct {
	db *sql.DB
}

type mockUser interface {
	GetUserbyUUID(uuid int) (*types.User, error)
	CreateUser(name string, uuid int) (*types.User, error)
}

func (m *mockUserStorage) GetUserbyUUID(uuid int) (*types.User, error) {
	rows, err := m.db.Query("SELECT * FROM users where uuid = ?", uuid)
	if err != nil {
		return nil, err
	}
	user := new(types.User)
	for rows.Next() {
		user, err = ScanRowsIntoUser(rows)
		if err != nil {
			return nil, err
		}
	}

	if user.UUID == 0 {
		log.Printf("User Not Found")
	}
	return user, nil
}

func (m *mockUserStorage) CreateUser(name string, uuid int) (*types.User, error) {
	_, err := m.db.Exec("INSERT INTO users (name, uuid) VALUES (?, ?)", name, uuid)
	if err != nil {
		return nil, err
	}
	user := &types.User{Name: name, UUID: uuid}
	return user, nil
}

func TestUserServiceHandlers(t *testing.T) {
	userStorage := &mockUserStorage{}
	handler := NewHandler(userStorage)

	t.Run("should fail if payload is empty or invalid", func(t *testing.T) {
		payload := types.RegisterUserPayload{
			Name: "someone",
			UUID: 123,
		}

		marshalled, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %+v", err)
		}

		req, err := http.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(marshalled))
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		router := http.NewServeMux()

		router.HandleFunc("/register", handler.HandleRegister)
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("unexpected status code %d, got %d", http.StatusBadRequest, rr.Code)
		}

	})
}
