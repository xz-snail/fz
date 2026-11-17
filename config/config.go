package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是服务启动所需的完整配置。
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Log          LogConfig          `yaml:"log"`
	SFTP         SFTPConfig         `yaml:"sftp"`
	CleanLog     CleanLogConfig     `yaml:"clean_log"`
	StatusReport StatusReportConfig `yaml:"status_report"`
	RemoteAPIs   RemoteAPIsConfig   `yaml:"remote_apis"`
	Device       DeviceConfig       `yaml:"device"`
}

// ServerConfig 描述 HTTP 服务监听和超时时间。
type ServerConfig struct {
	Host             string        `yaml:"host"`
	Port             int           `yaml:"port"`
	ReadTimeout      time.Duration `yaml:"-"`
	WriteTimeout     time.Duration `yaml:"-"`
	ShutdownTimeout  time.Duration `yaml:"-"`
	MaxBodySizeBytes int64         `yaml:"-"`
	ReadTimeoutRaw   string        `yaml:"read_timeout"`
	WriteTimeoutRaw  string        `yaml:"write_timeout"`
	ShutdownRaw      string        `yaml:"shutdown_timeout"`
	MaxBodySizeMB    int64         `yaml:"max_body_size_mb"`
}

// LogConfig 描述服务日志输出行为。
type LogConfig struct {
	Directory    string            `yaml:"directory"`
	LogInConsole bool              `yaml:"logInConsole"`
	GinRequests  bool              `yaml:"ginRequests"`
	Level        string            `yaml:"level"`
	Rotation     LogRotationConfig `yaml:"rotation"`
}

type LogRotationConfig struct {
	MaxSizeMB int `yaml:"maxSizeMB"`
}

// SFTPConfig 描述规则文件下载目录、SFTP 连接行为和规则下载定时任务。
type SFTPConfig struct {
	DownloadDir              string             `yaml:"download_dir"`
	DownloadURLPrefix        string             `yaml:"download_url_prefix"`
	DirMode                  os.FileMode        `yaml:"-"`
	DirModeRaw               string             `yaml:"dir_mode"`
	Timeout                  time.Duration      `yaml:"-"`
	TimeoutRaw               string             `yaml:"timeout"`
	HostKeyFingerprint       string             `yaml:"host_key_fingerprint"`
	InsecureSkipHostKeyCheck bool               `yaml:"insecure_skip_host_key_check"`
	RuleDownload             RuleDownloadConfig `yaml:"rule_download"`
}

// CleanLogConfig 描述日志清理定时任务配置。
type CleanLogConfig struct {
	Enabled     bool             `yaml:"enabled"`
	Interval    string           `yaml:"interval"` // 每天固定执行时间，格式 HH:MM
	DailyHour   int              `yaml:"-"`
	DailyMinute int              `yaml:"-"`
	Targets     []CleanLogTarget `yaml:"targets"`
}

// CleanLogTarget 描述一个需要清理的日志目录。
type CleanLogTarget struct {
	Path      string        `yaml:"path"`
	MaxAge    time.Duration `yaml:"-"`
	MaxAgeRaw string        `yaml:"max_age"`
}

// RuleDownloadConfig 描述规则文件定时下载任务配置。
type RuleDownloadConfig struct {
	Enabled     bool          `yaml:"enabled"`
	Interval    time.Duration `yaml:"-"`
	IntervalRaw string        `yaml:"interval"`
}

// StatusReportConfig 描述默认状态上报定时任务配置。
type StatusReportConfig struct {
	Enabled     bool          `yaml:"enabled"`
	Interval    time.Duration `yaml:"-"`
	IntervalRaw string        `yaml:"interval"`
}

// RemoteAPIsConfig 描述上游服务地址配置。
type RemoteAPIsConfig struct {
	StatusReportURL    string        `yaml:"status_report_url"`
	RuleIncrementURL   string        `yaml:"rule_increment_url"`
	RuleUpgradeURL     string        `yaml:"rule_upgrade_url"`
	UpgradeReportURL   string        `yaml:"upgrade_report_url"`
	UpgradeRedirectURL string        `yaml:"upgrade_redirect_url"`
	Timeout            time.Duration `yaml:"-"`
	TimeoutRaw         string        `yaml:"timeout"`
}

// DeviceConfig 描述设备身份信息。
type DeviceConfig struct {
	SID string `yaml:"sid"`
}

// Load 读取 YAML 配置并完成默认值和类型转换。
func Load(path string) (*Config, error) {
	if path == "" {
		path = "config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := defaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.normalize(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8018,
			ReadTimeoutRaw:  "15s",
			WriteTimeoutRaw: "60s",
			ShutdownRaw:     "10s",
		},
		Log: LogConfig{
			Directory:    "logs",
			LogInConsole: true,
			GinRequests:  true,
			Level:        "info",
			Rotation: LogRotationConfig{
				MaxSizeMB: 10,
			},
		},
		SFTP: SFTPConfig{
			DownloadDir:       "/data/sftp/tmp/rule",
			DownloadURLPrefix: "sftp://sftpdx:dx2022sftp@10.144.114.115:22//data/sftp/tmp/rule",
			DirModeRaw:        "0755",
			TimeoutRaw:        "30s",
			RuleDownload: RuleDownloadConfig{
				Enabled:     true,
				IntervalRaw: "1m",
			},
		},
		CleanLog: CleanLogConfig{
			Enabled: true,
		},
		StatusReport: StatusReportConfig{
			Enabled:     true,
			IntervalRaw: "5s",
		},
		RemoteAPIs: RemoteAPIsConfig{
			StatusReportURL:    "http://127.0.0.1:8089/statusreport",
			RuleIncrementURL:   "http://127.0.0.1:8089/rule/increment",
			RuleUpgradeURL:     "http://127.0.0.1:8089/rule/upgrade",
			UpgradeReportURL:   "http://127.0.0.1:8089/upgrade/report",
			UpgradeRedirectURL: "http://127.0.0.1:8089/upgrade/redirect",
			TimeoutRaw:         "60s",
		},
		Device: DeviceConfig{
			SID: "310000AC001",
		},
	}
}

func (c *Config) normalize() error {
	c.normalizeLog()

	var err error
	c.Server.ReadTimeout, err = parseDuration(c.Server.ReadTimeoutRaw, "server.read_timeout")
	if err != nil {
		return err
	}
	c.Server.WriteTimeout, err = parseDuration(c.Server.WriteTimeoutRaw, "server.write_timeout")
	if err != nil {
		return err
	}
	c.Server.ShutdownTimeout, err = parseDuration(c.Server.ShutdownRaw, "server.shutdown_timeout")
	if err != nil {
		return err
	}
	c.SFTP.Timeout, err = parseDuration(c.SFTP.TimeoutRaw, "sftp.timeout")
	if err != nil {
		return err
	}
	if c.CleanLog.Enabled && c.CleanLog.Interval == "" {
		return errors.New("clean_log.interval is required when clean_log.enabled is true")
	}
	if c.CleanLog.Interval != "" {
		c.CleanLog.DailyHour, c.CleanLog.DailyMinute, err = parseDailyAt(c.CleanLog.Interval)
		if err != nil {
			return fmt.Errorf("invalid clean_log.interval: %w", err)
		}
	}
	c.SFTP.RuleDownload.Interval, err = parseDuration(c.SFTP.RuleDownload.IntervalRaw, "sftp.rule_download.interval")
	if err != nil {
		return err
	}
	c.StatusReport.Interval, err = parseDuration(c.StatusReport.IntervalRaw, "status_report.interval")
	if err != nil {
		return err
	}
	c.RemoteAPIs.Timeout, err = parseDuration(c.RemoteAPIs.TimeoutRaw, "remote_apis.timeout")
	if err != nil {
		return err
	}
	if err := c.RemoteAPIs.validate(); err != nil {
		return err
	}
	for i := range c.CleanLog.Targets {
		c.CleanLog.Targets[i].MaxAge, err = parseDuration(c.CleanLog.Targets[i].MaxAgeRaw, fmt.Sprintf("clean_log.targets[%d].max_age", i))
		if err != nil {
			return err
		}
	}

	if c.SFTP.DirModeRaw == "" {
		c.SFTP.DirModeRaw = "0755"
	}
	var mode uint64
	if _, err := fmt.Sscanf(c.SFTP.DirModeRaw, "%o", &mode); err != nil {
		return fmt.Errorf("invalid sftp.dir_mode: %w", err)
	}
	c.SFTP.DirMode = os.FileMode(mode)

	if c.Server.Port == 0 {
		return errors.New("server.port is required")
	}
	if c.Server.MaxBodySizeMB > 0 {
		c.Server.MaxBodySizeBytes = c.Server.MaxBodySizeMB * 1024 * 1024
	}
	if c.SFTP.DownloadDir == "" {
		return errors.New("sftp.download_dir is required")
	}
	if c.SFTP.DownloadURLPrefix == "" {
		c.SFTP.DownloadURLPrefix = c.SFTP.DownloadDir
	}
	if c.SFTP.HostKeyFingerprint == "" && !c.SFTP.InsecureSkipHostKeyCheck {
		return errors.New("sftp.host_key_fingerprint is required unless sftp.insecure_skip_host_key_check is true")
	}
	return nil
}

func (c RemoteAPIsConfig) validate() error {
	if c.StatusReportURL == "" {
		return errors.New("remote_apis.status_report_url is required")
	}
	if c.RuleIncrementURL == "" {
		return errors.New("remote_apis.rule_increment_url is required")
	}
	if c.RuleUpgradeURL == "" {
		return errors.New("remote_apis.rule_upgrade_url is required")
	}
	if c.UpgradeReportURL == "" {
		return errors.New("remote_apis.upgrade_report_url is required")
	}
	if c.UpgradeRedirectURL == "" {
		return errors.New("remote_apis.upgrade_redirect_url is required")
	}
	return nil
}

func (c *Config) normalizeLog() {
	if c.Log.Directory == "" {
		c.Log.Directory = "logs"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.Rotation.MaxSizeMB <= 0 {
		c.Log.Rotation.MaxSizeMB = 10
	}
}

func parseDuration(value, name string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return duration, nil
}

// parseDailyAt 解析 "HH:MM" 格式（24 小时制）的每天固定执行时间。
func parseDailyAt(value string) (int, int, error) {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time %q, want HH:MM", value)
	}
	return t.Hour(), t.Minute(), nil
}
