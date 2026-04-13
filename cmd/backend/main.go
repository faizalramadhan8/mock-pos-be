package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/faizalramadhan/pos-be/internal/cron"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/router"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/database"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/logging"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/whatsapp"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

func init() {
	createDirIfNotExists := func(path string) error {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err = os.MkdirAll(path, os.ModePerm); err != nil {
				return err
			}

			log.Printf("%s directory created", path)
		}
		return nil
	}

	logsPath := "logs"
	if err := createDirIfNotExists(logsPath); err != nil {
		log.Fatalf("Error creating %s directory:%v", logsPath, err)
	}

	storagePath := "storage"
	if err := createDirIfNotExists(storagePath); err != nil {
		log.Fatalf("Error creating %s directory:%v", storagePath, err)
	}

}

func main() {
	// Set timezone to WIB (UTC+7)
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err == nil {
		time.Local = loc
	}

	// If .env file doesn't exist (e.g. running in Docker with env vars injected
	// via docker-compose), synthesize one from the process environment so koanf
	// can parse it with its existing dotenv provider.
	if _, err := os.Stat(".env"); errors.Is(err, os.ErrNotExist) {
		var lines []string
		for _, kv := range os.Environ() {
			lines = append(lines, kv)
		}
		if err := os.WriteFile(".env", []byte(strings.Join(lines, "\n")), 0600); err != nil {
			log.Fatalf("error writing synthesized .env file: %v", err)
		}
	}

	conf := koanf.New(".")
	if err := conf.Load(file.Provider(".env"), dotenv.Parser()); err != nil {
		log.Fatalf("error loading .env file: %v", err)
	}

	cfg := new(config.Config)
	if err := conf.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{FlatPaths: false}); err != nil {
		log.Fatalf("failed to read .env file: %v", err)
	}

	logfile, err := logging.NewLogger(cfg.LogFile)
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}

	db, err := gorm.Open(mysql.Open(cfg.GetGormAddress()), &gorm.Config{})
	if err != nil {
		log.Fatal(err.Error())
	}

	ctx := context.WithValue(context.Background(), enum.GormCtxKey, db)
	ctx = context.WithValue(ctx, enum.ConfigCtxKey, cfg)
	ctx = context.WithValue(ctx, enum.LoggerCtxKey, logfile)

	redisInstance := database.NewRedis(ctx, db)
	if err := redisInstance.ConnectRedis(ctx); err != nil {
		logfile.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	ctx = context.WithValue(ctx, enum.RedisCtxKey, redisInstance)

	waService := whatsapp.New(cfg.WahaURL, cfg.WahaAPIKey, cfg.WahaSession, cfg.WAReceiptEnabled, logfile)
	ctx = context.WithValue(ctx, enum.WhatsAppCtxKey, waService)

	app := fiber.New(fiber.Config{
		ProxyHeader: fiber.HeaderXForwardedFor,
		AppName:     cfg.AppName,
		JSONEncoder: json.Marshal,
		JSONDecoder: json.Unmarshal,
		BodyLimit:   10 * 1024 * 1024,
	})

	app.Use(fiberlogger.New())
	app.Use(recover.New())

	app.Use(cors.New(cors.Config{
		AllowOrigins:  "*",
		AllowMethods:  "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:  "Origin, Content-Type, Accept, Authorization",
		ExposeHeaders: "Content-Length",
		// AllowCredentials: true,
	}))

	app.Use("/storage", filesystem.New(filesystem.Config{
		Root:   http.Dir("./storage"),
		Browse: false,
	}))

	// Start push notification scheduler
	cronScheduler := cron.NewScheduler(ctx, db)
	cronScheduler.Start()

	router.UseRouter(ctx, app)

	if err = app.Listen(fmt.Sprintf(":%d", cfg.AppPort)); err != nil {
		log.Fatal(err.Error())
	}
}
