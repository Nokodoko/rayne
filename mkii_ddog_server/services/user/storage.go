package user

import (
	"database/sql"
	"log"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
)

type Storage struct {
	db *sql.DB
}

func NewStorage(db *sql.DB) *Storage {
	return &Storage{db: db}
}

func (s *Storage) GetUserbyUUID(uuid int) (*types.User, error) {
	rows, err := s.db.Query("SELECT * FROM users WHERE uuid = $1", uuid)
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

func ScanRowsIntoUser(rows *sql.Rows) (*types.User, error) {
	user := new(types.User)
	err := rows.Scan(
		&user.UUID,
		&user.Name,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Storage) CreateUser(name string, uuid int) (*types.User, error) {
	_, err := s.db.Exec("INSERT INTO users (name, uuid) VALUES ($1, $2)", name, uuid)
	if err != nil {
		return nil, err
	}
	user := &types.User{Name: name, UUID: uuid}
	return user, nil
}
