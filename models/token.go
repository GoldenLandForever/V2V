package models

type UserToken struct {
	UserID    uint64 `db:"user_id"`
	Tokens    int64  `db:"tokens"`
	VIPLevel  uint8  `db:"vip_level"`
	CreatedAt string `db:"created_at"`
	UpdatedAt string `db:"updated_at"`
}
