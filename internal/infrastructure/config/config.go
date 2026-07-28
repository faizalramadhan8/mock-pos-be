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
	// Midtrans (deprecated 28 Jul 2026, pindah ke PG DOKU wrapper). Field
	// dipertahankan untuk backward-compat dev env yang masih pakai .env lama;
	// kode Midtrans di internal/infrastructure/midtrans + call site di-comment.
	MidtransServerKey string `koanf:"MIDTRANS_SERVER_KEY"`
	MidtransClientKey string `koanf:"MIDTRANS_CLIENT_KEY"`
	MidtransIsProd    bool   `koanf:"MIDTRANS_IS_PROD"`

	// Payment Gateway (DOKU via alifworks PG wrapper). BE proxy ke PG:
	// FE tidak pegang credentials (cegah expose sample-secret di devtools).
	// Kalau BaseURL/AppKey/AppSecret kosong = stub mode (fake payment_url).
	// Docs: postman collection Payment Gateway v2 - DOKU.
	PGBaseURL       string `koanf:"PG_BASE_URL"`       // e.g. https://api-pgsanbox.alifworks.net
	PGAppKey        string `koanf:"PG_APP_KEY"`        // Basic Auth username
	PGAppSecret     string `koanf:"PG_APP_SECRET"`     // Basic Auth password
	PGMerchantName  string `koanf:"PG_MERCHANT_NAME"`  // samaran, e.g. "Testing"
	// PGWebhookSecret — reserved untuk HMAC verify DOKU notification. Kalau
	// kosong = skip verify (dev/sandbox). Prod wajib set.
	PGWebhookSecret string `koanf:"PG_WEBHOOK_SECRET"`

	BiteshipAPIKey     string `koanf:"BITESHIP_API_KEY"`
	BiteshipOriginArea string `koanf:"BITESHIP_ORIGIN_AREA_ID"` // area ID toko Bu Santi
	// Fallback selain area_id — Biteship API accept postal_code juga
	// (docs: https://biteship.com/id/docs/api/rates). Kalau area_id kosong
	// tapi postal_code di-set, request tetap jalan.
	BiteshipOriginPostal string `koanf:"BITESHIP_ORIGIN_POSTAL_CODE"`
	BiteshipCouriers     string `koanf:"BITESHIP_COURIERS"` // comma-separated; default "jne,jnt,sicepat,anteraja"

	// Biteship Order API — untuk buat shipping order beneran + trigger pickup.
	// Origin (shipper) info di-attach ke setiap request Create Order.
	// Kalau salah satu kosong = FE tampil warning "setup dulu di admin".
	BiteshipOriginName    string `koanf:"BITESHIP_ORIGIN_NAME"`    // "Toko Bahan Kue Santi"
	BiteshipOriginPhone   string `koanf:"BITESHIP_ORIGIN_PHONE"`   // "628123..." E.164
	BiteshipOriginAddress string `koanf:"BITESHIP_ORIGIN_ADDRESS"` // full alamat toko
	BiteshipOriginNote    string `koanf:"BITESHIP_ORIGIN_NOTE"`    // "Ruko PG II no 29"
	// Webhook secret — Biteship kirim header signature; kita verify SHA256 HMAC.
	// Kosong = skip verify (dev mode). Set random 32-char string untuk prod.
	BiteshipWebhookSecret string `koanf:"BITESHIP_WEBHOOK_SECRET"`

	// Fallback shipping stub (kalau BITESHIP_API_KEY kosong).
	// Rate per kg flat untuk MVP validate demand.
	ShippingFlatBaseRate int `koanf:"SHIPPING_FLAT_BASE_RATE"` // default 15000

	// Brevo (formerly Sendinblue) — transactional email untuk OTP reset
	// password + order confirmation. Free tier: 300 email/hari.
	// Kalau kosong → OTP di-log ke console (dev mode fallback).
	BrevoAPIKey      string `koanf:"BREVO_API_KEY"`
	BrevoSenderEmail string `koanf:"BREVO_SENDER_EMAIL"` // e.g. noreply@tbksanti.id
	BrevoSenderName  string `koanf:"BREVO_SENDER_NAME"`  // e.g. "TBK Santi"
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
