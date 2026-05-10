package models

type User struct {
	Id    int64  `json:"id"`
	Email string `json:"email"`
	Hash  string `json:"hash"`
}

type Photo struct {
	Id       int64 `json:"id"`
	UserId   int64 `json:"user_id"`
	IsActive bool  `json:"is_active"`
}
