package xsuportal

import (
	"github.com/go-redis/redis/v8"
	_ "github.com/go-redis/redis/v8"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/isucon/isucon10-final/webapp/golang/util"
)

func GetDB() (*sqlx.DB, error) {
	mysqlConfig := mysql.NewConfig()
	mysqlConfig.Net = "tcp"
	mysqlConfig.Addr = util.GetEnv("MYSQL_HOSTNAME", "127.0.0.1") + ":" + util.GetEnv("MYSQL_PORT", "3306")
	mysqlConfig.User = util.GetEnv("MYSQL_USER", "isucon")
	mysqlConfig.Passwd = util.GetEnv("MYSQL_PASS", "isucon")
	mysqlConfig.DBName = util.GetEnv("MYSQL_DATABASE", "xsuportal")
	mysqlConfig.Params = map[string]string{
		"time_zone": "'+00:00'",
	}
	mysqlConfig.ParseTime = true
	mysqlConfig.InterpolateParams = true

	return sqlx.Open("mysql", mysqlConfig.FormatDSN())
}

func GetRDB () (*redis.Client) {
	return redis.NewClient(&redis.Options{
		Addr:     "isu1.t.isucon.dev:6379",
		Password: "",
		DB:       0,
	})
}
