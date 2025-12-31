package types

type RegisterUserPayload struct {
	Name string `json:"Name"`
	UUID int    `json:"UUID"`
}

type User struct {
	Name string `json:"Name"`
	UUID int    `json:"UUID"`
}

type UserStorage interface {
	GetUserbyUUID(uuid int) (*User, error)
	CreateUser(name string, uuid int) (*User, error)
}
