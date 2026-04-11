package config

import (
	"fmt"
	"time"
)

type Config struct {
	AppPort    uint32  `koanf:"APP_PORT"`
	AppName    string  `koanf:"APP_NAME"`
	AppEnv     string  `koanf:"APP_ENV"`
	AppURL     string  `koanf:"APP_URL"`
	RootDir    string  `koanf:"ROOTDIR"`
	AppVersion float32 `koanf:"APP_VERSION"`

	DBHost         string `koanf:"MYSQL_HOST"`
	DBUserName     string `koanf:"MYSQL_USER"`
	DBUserPassword string `koanf:"MYSQL_PASSWORD"`
	DBName         string `koanf:"MYSQL_DB"`
	DBPort         string `koanf:"MYSQL_PORT"`
	DBTimeZone     string `koanf:"DB_TIME_ZONE"`

	RedisAddr string `koanf:"REDIS_HOST"`
	RedisPass string `koanf:"REDIS_PASSWORD"`
	RedisDB   int    `koanf:"REDIS_DB"`

	JwtSecret                string        `koanf:"JWT_SECRET"`
	JwtAccessTokenExpiresIn  time.Duration `koanf:"JWT_ACCESS_TOKEN_EXPIRED_IN"`
	JwtRefreshTokenExpiresIn time.Duration `koanf:"JWT_REFRESH_TOKEN_EXPIRED_IN"`
	LogFile                  string        `koanf:"LOGFILE"`

	InstanceID string `koanf:"INSTANCE_ID"`

	VAPIDPublicKey  string `koanf:"VAPID_PUBLIC_KEY"`
	VAPIDPrivateKey string `koanf:"VAPID_PRIVATE_KEY"`
}

func (c *Config) GetGormAddress() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Asia%%2FJakarta",
		c.DBUserName,
		c.DBUserPassword,
		c.DBHost,
		c.DBPort,
		c.DBName,
	)
}
