package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type SpecificLevelWriter struct {
	io.Writer
	Levels []zerolog.Level
}

func (w SpecificLevelWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	for _, l := range w.Levels {
		if l == level {
			return w.Write(p)
		}
	}
	return len(p), nil
}

func NewLogger(output string) (*zerolog.Logger, error) {
	// Create the logs directory if it doesn't exist
	if err := os.MkdirAll(output, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Construct the full log file path
	logFilePath := filepath.Join(output, fmt.Sprintf("debug-%s.log", time.Now().Format("2006-01-02")))

	debugFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	writer := zerolog.ConsoleWriter{
		Out:        debugFile,
		TimeFormat: "2006-01-02 15:04:05",
		NoColor:    true,
	}

	writer.FormatLevel = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
	}
	writer.FormatFieldName = func(i interface{}) string {
		return fmt.Sprintf("%s:", i)
	}
	writer.FormatFieldValue = func(i interface{}) string {
		return fmt.Sprintf("%s", i)
	}

	logs := zerolog.New(writer).With().Timestamp().Caller().Logger()

	return &logs, nil
}
