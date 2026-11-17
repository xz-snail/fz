// Command downstream 模拟下游服务，接收 fz 转发的规则接口：
//
//	POST /rule/increment     增量规则下发
//	POST /rule/upgrade       全量规则下发（携带 bathNo header）
//	POST /upgrade/redirect   重定向规则下发
//
// 配置默认读取 test/downstream/config.yaml，命令行参数优先。
// fail=true 时所有接口返回业务失败，用于验证 fz 的失败累计与重试逻辑。
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

// fileConfig 是下游 mock 的配置文件结构。
type fileConfig struct {
	Addr string `yaml:"addr"` // 监听地址
	Fail bool   `yaml:"fail"` // 是否返回业务失败
}

func main() {
	configPath := flag.String("config", "", "配置文件路径，留空自动查找 config.yaml")
	flag.String("addr", "", "监听地址")
	flag.Bool("fail", false, "为 true 时所有接口返回业务失败，用于验证失败重试")
	flag.Parse()

	cfg := fileConfig{Addr: ":8090"}
	configFile, err := resolveConfigPath(*configPath)
	if err != nil {
		log.Fatalf("读取配置失败：%v", err)
	}
	if err := loadConfig(configFile, &cfg); err != nil {
		log.Fatalf("读取配置失败：%v", err)
	}
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			cfg.Addr = f.Value.String()
		case "fail":
			cfg.Fail = f.Value.String() == "true"
		}
	})

	requestLog, closeLog, err := openRequestLog(configFile)
	if err != nil {
		log.Fatalf("初始化请求日志失败：%v", err)
	}
	defer closeLog()

	mux := http.NewServeMux()
	routes := []route{
		{"POST", "/rule/increment", "增量规则转发"},
		{"POST", "/rule/upgrade", "全量规则转发"},
		{"POST", "/upgrade/redirect", "重定向规则转发"},
	}
	for _, route := range routes {
		mux.HandleFunc(route.path, handle(cfg.Fail, requestLog))
	}

	printRoutes("[downstream]", routes)
	log.Printf("下游 mock 已启动：http://127.0.0.1%s（fail=%v），请求详情见 logs/requests.log", cfg.Addr, cfg.Fail)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
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

// resolveConfigPath 查找配置文件：显式参数优先，其次当前目录，最后仓库根相对路径。
func resolveConfigPath(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", err
		}
		return explicit, nil
	}
	for _, candidate := range []string{"config.yaml", "test/downstream/config.yaml"} {
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

// handle 终端只打印请求地址，完整请求内容和响应结果写入日志文件。
func handle(fail bool, requestLog *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s %s\n", r.Method, r.URL.RequestURI())

		body, err := io.ReadAll(r.Body)
		if err != nil {
			requestLog.Printf("[%s] 读取请求体失败：%v", r.URL.Path, err)
		} else {
			requestLog.Printf("[%s] %s bathNo=%q\n%s", r.URL.Path, r.Method, r.Header.Get("bathNo"), pretty(body))
		}

		code := 200
		message := "成功"
		if fail {
			code = 500
			message = "模拟下游失败"
		}
		requestLog.Printf("[%s] 响应 code=%d message=%s", r.URL.Path, code, message)

		w.Header().Set("Content-Type", "application/json;charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":      code,
			"message":   message,
			"body":      !fail,
			"timestamp": time.Now().Unix(),
		})
	}
}

// pretty 格式化 JSON，便于在控制台核对内容。
func pretty(body []byte) string {
	var out bytes.Buffer
	if err := json.Indent(&out, body, "", "  "); err != nil {
		return string(body)
	}
	return out.String()
}
