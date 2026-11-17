// Command upstream 模拟上游管理中心，接收 fz 主动发出的请求：
//
//	POST /statusreport     后台默认设备状态上报
//	POST /upgrade/report   全量规则消费反馈（携带 bathNo header）
//
// 配置默认读取 test/upstream/config.yaml，命令行参数优先。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// fileConfig 是上游 mock 的配置文件结构。
type fileConfig struct {
	Addr string `yaml:"addr"` // 监听地址
}

func main() {
	configPath := flag.String("config", "", "配置文件路径，留空自动查找 config.yaml")
	flag.String("addr", "", "监听地址")
	flag.Parse()

	configFile, err := resolveConfigPath(*configPath)
	if err != nil {
		log.Fatalf("读取配置失败：%v", err)
	}
	cfg := defaultFileConfig()
	if err := loadConfig(configFile, &cfg); err != nil {
		log.Fatalf("读取配置失败：%v", err)
	}
	applyFlags(&cfg)

	requestLog, closeLog, err := openRequestLog(configFile)
	if err != nil {
		log.Fatalf("初始化请求日志失败：%v", err)
	}
	defer closeLog()

	runServer(cfg.Addr, requestLog)
}

// defaultFileConfig 返回配置文件缺省值，保证缺少字段时仍可运行。
func defaultFileConfig() fileConfig {
	return fileConfig{
		Addr: ":8089",
	}
}

// resolveConfigPath 查找配置文件：显式参数优先，其次当前目录，最后仓库根相对路径。
func resolveConfigPath(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", err
		}
		return explicit, nil
	}
	for _, candidate := range []string{"config.yaml", "test/upstream/config.yaml"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到配置文件 config.yaml，可用 -config 指定")
}

// openRequestLog 在配置文件所在目录的 logs 下创建请求日志文件。
func openRequestLog(configFile string) (*log.Logger, func() error, error) {
	dir := filepath.Join(filepath.Dir(configFile), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "requests.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}
	return log.New(file, "", log.LstdFlags), file.Close, nil
}

// loadConfig 读取 YAML 配置，未出现的字段保留默认值。
func loadConfig(path string, cfg *fileConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

// applyFlags 用命令行显式传入的参数覆盖配置文件值。
func applyFlags(cfg *fileConfig) {
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			cfg.Addr = f.Value.String()
		}
	})
}

// runServer 启动上游 mock 接收 fz 上报，终端只显示请求地址，详情写入日志文件。
func runServer(addr string, requestLog *log.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/statusreport", func(w http.ResponseWriter, r *http.Request) {
		logReceived("statusreport", r, requestLog)
		writeSuccess(w)
	})
	mux.HandleFunc("/upgrade/report", func(w http.ResponseWriter, r *http.Request) {
		logReceived("upgrade/report", r, requestLog)
		writeSuccess(w)
	})

	printRoutes("[upstream]", []route{
		{"POST", "/statusreport", "接收 fz 心跳上报"},
		{"POST", "/upgrade/report", "接收全量规则消费反馈"},
	})
	log.Printf("上游 mock 已启动：http://127.0.0.1%s，请求详情见 logs/requests.log", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// route 描述一个已注册的 mock 接口。
type route struct {
	method string
	path   string
	desc   string
}

// printRoutes 启动时打印接口清单，格式对齐 gin 的路由注册输出。
func printRoutes(tag string, routes []route) {
	for _, r := range routes {
		fmt.Printf("%-14s %-6s %-20s --> %s\n", tag, r.method, r.path, r.desc)
	}
}

// logReceived 终端打印请求地址，路径、批次号和请求体写入日志文件。
func logReceived(name string, r *http.Request, requestLog *log.Logger) {
	fmt.Printf("%s %s\n", r.Method, r.URL.RequestURI())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		requestLog.Printf("[%s] 读取请求体失败：%v", name, err)
		return
	}
	requestLog.Printf("[%s] %s %s bathNo=%q\n%s", name, r.Method, r.URL.Path, r.Header.Get("bathNo"), pretty(body))
}

// writeSuccess 返回与 fz 一致的统一成功响应。
func writeSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":      200,
		"message":   "成功",
		"body":      true,
		"timestamp": time.Now().Unix(),
	})
}

// pretty 格式化 JSON，便于在控制台核对内容。
func pretty(body []byte) string {
	var out bytes.Buffer
	if err := json.Indent(&out, body, "", "  "); err != nil {
		return string(body)
	}
	return out.String()
}
