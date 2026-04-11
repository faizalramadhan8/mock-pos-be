package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
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
