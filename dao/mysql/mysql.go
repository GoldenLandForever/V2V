package mysql

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

var Db *sqlx.DB

// Init 初始化MySQL连接
func Init() (err error) {
	// "user:password@tcp(host:port)/dbname"
	dsn := fmt.Sprintf("root:123456@tcp(192.168.1.50:3306)/V2V?parseTime=true&loc=Local")
	Db, err = sqlx.Connect("mysql", dsn)
	if err != nil {
		return
	}
	Db.SetMaxOpenConns(32)
	Db.SetMaxIdleConns(16)
	return
}

// Close 关闭MySQL连接
func Close() {
	_ = Db.Close()
}
