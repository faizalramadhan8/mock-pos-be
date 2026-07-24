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

	JwtSecret               string        `koanf:"JWT_SECRET"`
	JwtAccessTokenExpiresIn time.Duration `koanf:"JWT_ACCESS_TOKEN_EXPIRED_IN"`
	LogFile                 string        `koanf:"LOGFILE"`

	InstanceID string `koanf:"INSTANCE_ID"`

	VAPIDPublicKey  string `koanf:"VAPID_PUBLIC_KEY"`
	VAPIDPrivateKey string `koanf:"VAPID_PRIVATE_KEY"`

	WahaURL          string `koanf:"WAHA_URL"`
	WahaAPIKey       string `koanf:"WAHA_API_KEY"`
	WahaSession      string `koanf:"WAHA_SESSION"`
	WAReceiptEnabled bool   `koanf:"WA_RECEIPT_ENABLED"`

	// E-commerce Fase 3 integrations (Bu Santi 24 Jul 2026).
	// Semua optional — kalau kosong, fallback ke stub/manual mode.
	MidtransServerKey string `koanf:"MIDTRANS_SERVER_KEY"` // API key untuk create Snap token
	MidtransClientKey string `koanf:"MIDTRANS_CLIENT_KEY"` // untuk FE Snap.js
	MidtransIsProd    bool   `koanf:"MIDTRANS_IS_PROD"`    // false = sandbox

	BiteshipAPIKey     string `koanf:"BITESHIP_API_KEY"`
	BiteshipOriginArea string `koanf:"BITESHIP_ORIGIN_AREA_ID"` // area ID toko Bu Santi
	// Fallback selain area_id — Biteship API accept postal_code juga
	// (docs: https://biteship.com/id/docs/api/rates). Kalau area_id kosong
	// tapi postal_code di-set, request tetap jalan.
	BiteshipOriginPostal string `koanf:"BITESHIP_ORIGIN_POSTAL_CODE"`
	BiteshipCouriers     string `koanf:"BITESHIP_COURIERS"` // comma-separated; default "jne,jnt,sicepat,anteraja"

	// Fallback shipping stub (kalau BITESHIP_API_KEY kosong).
	// Rate per kg flat untuk MVP validate demand.
	ShippingFlatBaseRate int `koanf:"SHIPPING_FLAT_BASE_RATE"` // default 15000
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
