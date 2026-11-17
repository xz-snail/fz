package initialize

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ctaf-fz/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger(cfg config.LogConfig) (*zap.Logger, error) {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	cores := make([]zapcore.Core, 0, 3)
	cores = append(cores, zapcore.NewCore(
		encoder,
		zapcore.AddSync(newRotatingDailyWriter(cfg.Directory, "info", cfg.Rotation.MaxSizeMB)),
		zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			return level == zapcore.InfoLevel
		}),
	))

	cores = append(cores, zapcore.NewCore(
		encoder,
		zapcore.AddSync(newRotatingDailyWriter(cfg.Directory, "error", cfg.Rotation.MaxSizeMB)),
		zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			return level >= zapcore.WarnLevel
		}),
	))

	if cfg.LogInConsole {
		consoleLevel := zapcore.InfoLevel
		if cfg.Level == "debug" {
			consoleLevel = zapcore.DebugLevel
		}
		cores = append(cores, zapcore.NewCore(
			encoder,
			zapcore.AddSync(os.Stdout),
			consoleLevel,
		))
	}

	if len(cores) == 0 {
		cores = append(cores, zapcore.NewCore(
			encoder,
			zapcore.AddSync(os.Stdout),
			zapcore.ErrorLevel,
		))
	}

	options := []zap.Option{zap.ErrorOutput(zapcore.AddSync(os.Stderr)), zap.AddCaller()}
	return zap.New(zapcore.NewTee(cores...), options...), nil
}

type rotatingDailyWriter struct {
	baseDir    string
	baseName   string
	maxSize    int64
	mu         sync.Mutex
	currentDay string
	file       *os.File
	fileSize   int64
}

func newRotatingDailyWriter(baseDir string, baseName string, maxSizeMB int) *rotatingDailyWriter {
	return &rotatingDailyWriter{
		baseDir:  baseDir,
		baseName: baseName,
		maxSize:  int64(maxSizeMB) * 1024 * 1024,
	}
}

func (w *rotatingDailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.prepare(len(p)); err != nil {
		return 0, err
	}

	n, err := w.file.Write(p)
	w.fileSize += int64(n)
	return n, err
}

func (w *rotatingDailyWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}

func (w *rotatingDailyWriter) prepare(incomingSize int) error {
	day := time.Now().Format("2006-01-02")
	if w.file == nil || w.currentDay != day {
		if err := w.open(day); err != nil {
			return err
		}
	}

	if w.maxSize > 0 && w.fileSize+int64(incomingSize) > w.maxSize {
		if err := w.rotate(); err != nil {
			return err
		}
	}

	return nil
}

func (w *rotatingDailyWriter) open(day string) error {
	if w.file != nil {
		_ = w.file.Close()
	}

	dir := filepath.Join(w.baseDir, day)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, w.baseName+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}

	w.file = file
	w.currentDay = day
	w.fileSize = info.Size()
	return nil
}

func (w *rotatingDailyWriter) rotate() error {
	if w.file == nil {
		return nil
	}

	currentPath := filepath.Join(w.baseDir, w.currentDay, w.baseName+".log")
	rotatedPath := filepath.Join(
		w.baseDir,
		w.currentDay,
		fmt.Sprintf("%s-%s.log", w.baseName, time.Now().Format("20060102-150405")),
	)

	if err := w.file.Close(); err != nil {
		return err
	}
	w.file = nil

	if err := os.Rename(currentPath, rotatedPath); err != nil {
		return err
	}

	return w.open(w.currentDay)
}
