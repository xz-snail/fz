package jobs

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"ctaf-fz/config"
	"ctaf-fz/global"

	"go.uber.org/zap"
)

// CleanLogJob 定时清理超过保留时间的日志文件。
type CleanLogJob struct {
	cfg config.CleanLogConfig
}

// NewCleanLogJob 创建日志清理任务。
func NewCleanLogJob(cfg config.CleanLogConfig) *CleanLogJob {
	return &CleanLogJob{cfg: cfg}
}

// Start 启动日志清理定时任务，每天在 interval 指定的本地时间执行。
func (j *CleanLogJob) Start(ctx context.Context) {
	if !j.cfg.Enabled || j.cfg.Interval == "" || len(j.cfg.Targets) == 0 {
		return
	}
	go j.scheduleDaily(ctx)
}

// scheduleDaily 每天在 interval 指定的本地时间执行清理。
func (j *CleanLogJob) scheduleDaily(ctx context.Context) {
	for {
		timer := time.NewTimer(time.Until(nextDailyTime(j.cfg.DailyHour, j.cfg.DailyMinute)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			j.run()
		}
	}
}

// nextDailyTime 计算今天或明天 hour:minute 的下一个执行时间点。
func nextDailyTime(hour, minute int) time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// run 按保留时间逐个清理配置目录下的过期日志文件，并移除清空后的空目录。
func (j *CleanLogJob) run() {
	now := time.Now()
	for _, target := range j.cfg.Targets {
		if target.Path == "" || target.MaxAge <= 0 {
			continue
		}
		scanned := 0
		removed := 0
		removeFailed := 0
		if err := filepath.WalkDir(target.Path, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			scanned++
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			if now.Sub(info.ModTime()) > target.MaxAge {
				if err := os.Remove(path); err != nil {
					removeFailed++
					global.Logger.Warn("clean_log remove failed", zap.String("path", path), zap.Error(err))
					return nil
				}
				removed++
			}
			return nil
		}); err != nil {
			global.Logger.Warn("clean_log walk failed", zap.String("path", target.Path), zap.Error(err))
			continue
		}
		removedDirs := j.removeEmptyDirs(target.Path)
		global.Logger.Info("clean_log completed", zap.String("path", target.Path), zap.Duration("maxAge", target.MaxAge), zap.Int("scanned", scanned), zap.Int("removed", removed), zap.Int("removeFailed", removeFailed), zap.Int("removedDirs", removedDirs))
	}
}

// removeEmptyDirs 移除 root 下清空后的空目录（如按日期归档的日志目录），返回移除数量。
func (j *CleanLogJob) removeEmptyDirs(root string) int {
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && entry.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	removed := 0
	// WalkDir 先访问父目录后访问子目录，倒序遍历即从最深层开始移除。
	for i := len(dirs) - 1; i >= 0; i-- {
		if os.Remove(dirs[i]) == nil {
			removed++
		}
	}
	return removed
}
