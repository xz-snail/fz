# CTAF SFTP Go 服务

该服务使用 Gin 重构原 FZ/Flow 规则下发链路，保留现有接口路径和 legacy 响应格式。当前版本不使用数据库，规则下发状态保存在进程内存缓存中。

## 目录结构

```text
cmd/server/                 启动入口
config/                     配置结构和 YAML 加载
core/                       HTTP Server 和后台任务启动
initialize/                 配置、缓存、依赖和路由初始化
global/                     全局运行对象
router/                     Gin 路由分组
api/v1/                     API 分组入口
api/v1/fz/                  FZ Gin 接口处理
service/                    Service 分组入口
service/fz/                 FZ 业务逻辑和内存缓存
model/common/               公共转换方法
model/common/response/      统一响应封装
model/fz/                   FZ 业务结构体
model/fz/request/           FZ 请求读取和字段校验
jobs/                       定时任务
jobs/rule/                  规则下载定时任务和 SFTP 下载
```

API 和 Service 分组参考 `go-demo`：

- `api/v1/enter.go` 暴露 `ApiGroupApp`
- `api/v1/fz/enter.go` 通过 `ApiGroup` 嵌入 `FlowApi`
- `service/enter.go` 暴露 `ServiceGroupApp`
- `service/fz/enter.go` 通过 `ServiceGroup` 嵌入 `FlowService`

## 接口

当前注册在根路径，不增加 `/api/v1` 前缀：

| 方法 | 路径                | 说明                 |
| ---- | ------------------- | -------------------- |
| GET  | `/healthz`          | 健康检查             |
| POST | `/statusreport`     | 接收外部设备状态上报 |
| POST | `/rule/increment`   | 转发增量规则下发     |
| POST | `/rule/upgrade`     | 保存全量规则下发信息 |
| POST | `/upgrade/redirect` | 转发导航规则配置     |

`/statusreport` 的空 body 不再触发默认上报，默认状态上报由后台定时任务在 service 中执行。

## 后台任务

服务启动后会启动以下任务：

- `CleanLogJob`：按 `clean_log` 配置清理日志目录。
- `RuleDownloadJob`：按 `sftp.rule_download` 配置检查缓存中的待下载全量规则。
- `StatusReportJob`：按 `status_report` 配置上报默认设备状态，启动后立即执行一次。

默认状态上报 payload 参考原 PHP `// Me` 逻辑，包含：

- `sid`
- `type`
- `timestamp`
- `vendorid`
- `config`
- `db`
- `sysinfor`

## 规则包下载链路

1. 上游调用 `POST /rule/upgrade` 下发全量规则。
2. API 校验请求字段和 `bathNo` header。
3. `FlowService.SaveRuleRecord` 将 `sid/type/bathNo/ruleType/version/path/md5` 保存到规则下载信息缓存，并把下载任务标记设为 `todo_download`。
4. `RuleDownloadJob` 按 `sftp.rule_download.interval` 定时按创建时间读取默认 `sid/type` 的下一条下载任务，按当前标记执行对应步骤。
5. 标记为 `todo_download` 时，读取规则下载信息中的 `path`（格式如 `sftp://user:password@host:port/upload/RULE_xxx.txt.gz`）并下载文件到 `sftp.download_dir`，下载成功后标记更新为 `todo_feedback`。
6. 标记为 `todo_feedback` 时，调用 `ForwardRuleUpgrade` 将本地下载路径下发给下游，再调用 `UpgradeReport` 向上游同步消费反馈；两步都成功后才删除任务标记和下发记录，任一步失败按任务失败累计并等待下一轮重试。
7. 任一步骤失败会累计失败次数，超过 5 次即删除该任务（标记和下发记录一起删除）；同批次号重新下发会重建任务。

缓存 key 规则仍兼容旧逻辑：

```text
sid + "_" + type + "_sidlog" + "_" + bathNo
```

下载任务标记：

- `todo_download`：待下载；保存下发记录后的标记
- `todo_feedback`：待反馈；规则文件下载完成后的标记

没有待执行任务时缓存中无对应条目：反馈成功或失败超过 5 次后，任务标记和下发记录一起删除；同批次号重新下发会重建任务。

## 请求和响应封装

- 请求读取和字段校验放在 `model/fz/request/flow.go`。
- FZ 业务结构体放在 `model/fz/flow.go`。
- 公共转换方法放在 `model/common/convert.go`。
- 响应封装放在 `model/common/response/response.go`。

统一响应结构使用 `MessageResponse`：

```json
{
  "code": 200,
  "message": "成功",
  "body": true,
  "timestamp": 1710000000
}
```

部分 legacy 转发接口会直接透传上游响应体，以保持兼容。

## 配置

复制示例配置后填写真实参数：

```bash
cp config.example.yaml config.yaml
```

`config.yaml` 已加入 `.gitignore`，不要提交真实地址、账号或生产配置。

完整示例：

```yaml
server:
  host: "0.0.0.0"
  port: 8018
  read_timeout: "15s"
  write_timeout: "60s"
  shutdown_timeout: "10s"
  max_body_size_mb: 100

log:
  directory: "logs"
  logInConsole: false
  ginRequests: true
  level: "info"
  rotation:
    maxSizeMB: 10

sftp:
  download_dir: "/data/sftp/tmp/rule"
  download_url_prefix: "sftp://user:password@127.0.0.1:22/data/sftp/tmp/rule"
  dir_mode: "0755"
  timeout: "10m"
  host_key_fingerprint: ""
  insecure_skip_host_key_check: true
  rule_download:
    enabled: true
    interval: "1m"

clean_log:
  enabled: true
  interval: "24h"
  targets:
    - path: "/data/sftp/tmp/log"
      max_age: "48h"

status_report:
  enabled: true
  interval: "5s"

remote_apis:
  status_report_url: "http://172.16.103.36:8080/statusreport"
  rule_increment_url: "http://172.16.103.36:8080/rule/increment"
  rule_upgrade_url: "http://172.16.103.36:8080/rule/upgrade"
  upgrade_report_url: "http://172.16.103.36:8080/upgrade/report"
  upgrade_redirect_url: "http://172.16.103.36:8080/upgrade/redirect"
  timeout: "10m"

device:
  sid: "310000AC001"
```

配置项说明：

时间配置使用 Go duration 格式，常用示例：`"30s"` 表示 30 秒，`"5m"` 表示 5 分钟，`"2h"` 表示 2 小时，`"24h"` 表示 1 天，`"48h"` 表示 2 天，`"0s"` 表示不设置超时。

### server

- `host`：HTTP 服务监听地址。
- `port`：HTTP 服务监听端口。
- `read_timeout`：读取请求超时时间；配置为 `"0s"` 表示不设置读取超时。
- `write_timeout`：写响应超时时间；配置为 `"0s"` 表示不设置写入超时。
- `shutdown_timeout`：进程退出时的优雅关闭超时时间；建议保留有限时间。
- `max_body_size_mb`：请求体最大大小，单位 MB；配置为 `0` 或不配置表示不限制。

### log

- `directory`：日志文件保存目录；服务会按日期写入 `log.directory/YYYY-MM-DD/info.log` 和 `error.log`。
- `logInConsole`：是否同时输出日志到控制台。
- `ginRequests`：是否记录 Gin HTTP 请求日志。
- `level`：控制台日志级别，支持 `debug` 或默认 `info`。
- `rotation.maxSizeMB`：单个日志文件最大大小，单位 MB，超过后生成带时间后缀的历史文件。

### sftp

- `download_dir`：规则文件下载后的本地保存目录。
- `download_url_prefix`：规则下载完成后转发给下游的文件地址前缀，会与下载文件名拼接生成转发路径。
- `dir_mode`：下载目录不存在时创建目录使用的权限。
- `timeout`：SFTP 连接和下载超时时间；配置为 `"0s"` 表示不设置超时，但生产环境建议配置较大的有限时间，例如 `"10m"` 或 `"30m"`。
- `host_key_fingerprint`：SFTP 主机 key 的 SHA256 指纹，格式如 `SHA256:...`，必须与服务端主机 key 匹配。
- `insecure_skip_host_key_check`：是否跳过 SFTP 主机 key 校验；生产环境建议为 `false`。
- `rule_download.enabled`：是否启用规则文件定时下载任务。
- `rule_download.interval`：规则文件定时下载检查间隔；支持 `s`、`m`、`h` 等单位，可组合（如 `"1h30m"`），不支持 `d`/`w`，天需换算成小时（如 `"24h"`）。

### clean_log

- `enabled`：是否启用日志清理任务。
- `interval`：每天固定执行时间，格式 `HH:MM`（24 小时制、本地时区），如 `"02:00"` 表示每天凌晨 2 点执行；启用清理任务时必填。
- `targets[].path`：需要清理的日志目录。
- `targets[].max_age`：超过该时间的文件会被清理；支持 `s`、`m`、`h` 等单位，可组合（如 `"1h30m"`），不支持 `d`/`w`，天需换算成小时（如 `"48h"` 为 2 天）；配置为 `"0s"` 或不配置表示不清理该目录。

### status_report

- `enabled`：是否启用默认状态上报任务。
- `interval`：默认状态上报间隔。

### remote_apis

- `status_report_url`：状态上报转发地址。
- `rule_increment_url`：增量规则转发地址。
- `rule_upgrade_url`：全量规则下发转发地址。
- `upgrade_report_url`：全量规则消费反馈转发地址。
- `upgrade_redirect_url`：重定向规则转发地址。
- `timeout`：转发请求到上游服务的超时时间；配置为 `"0s"` 表示不设置超时。

### device

- `sid`：默认设备 SID。

## 构建

```bash
go mod tidy
go build -o ctaf-fz ./cmd/server
```

### 打包不同系统的二进制

Go 支持通过 `GOOS` 和 `GOARCH` 交叉编译不同系统和 CPU 架构的二进制文件。

```bash
# Linux x86_64
GOOS=linux GOARCH=amd64 go build -o dist/ctaf-fz-linux-amd64 ./cmd/server
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o dist/ctaf-fz-linux-amd64 ./cmd/server

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o dist/ctaf-fz-linux-arm64 ./cmd/server

# Windows x86_64
GOOS=windows GOARCH=amd64 go build -o dist/ctaf-fz-windows-amd64.exe ./cmd/server
$env:GOOS="windows"; $env:GOARCH="amd64"; go build -o dist/ctaf-fz-windows-amd64.exe ./cmd/server

# Windows ARM64
GOOS=windows GOARCH=arm64 go build -o dist/ctaf-fz-windows-arm64.exe ./cmd/server

# macOS x86_64
GOOS=darwin GOARCH=amd64 go build -o dist/ctaf-fz-darwin-amd64 ./cmd/server

# macOS ARM64 / Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o dist/ctaf-fz-darwin-arm64 ./cmd/server
```

如需先创建输出目录：

```bash
mkdir -p dist
```

常见取值：

- `GOOS`：`linux`、`windows`、`darwin`
- `GOARCH`：`amd64`、`arm64`

### 打包脚本

在项目根目录执行：

```bash
cd fz
bash build.sh
```

不传参数时按当前系统打包；也可以指定目标系统和架构：

```bash
bash build.sh linux amd64
bash build.sh linux arm64
bash build.sh windows amd64
bash build.sh darwin arm64
```

输出文件名为 `dist/ctaf-fz-<时间戳>`（如 `dist/ctaf-fz-20261116153045`），Windows 自动加 `.exe` 后缀。

将二进制文件和配置文件上传到 Linux 服务器：

```bash
scp dist/ctaf-fz-linux-amd64 config.yaml user@server:/opt/ctaf-fz/
```

在 Linux 服务器上运行：

```bash
cd /opt/ctaf-fz
chmod +x ctaf-fz-linux-amd64
./ctaf-fz-linux-amd64 -config config.yaml
```

后台运行示例：

```bash
nohup ./ctaf-fz-linux-amd64 -config config.yaml > output.log 2>&1 &
```

## 运行

```bash
./ctaf-fz -config config.yaml
```

后台运行示例：

```bash
nohup ./ctaf-fz -config config.yaml > output.log 2>&1 &
```

## 验证

```bash
gofmt -w .
go test ./...
go build -o ctaf-fz ./cmd/server
```

接口快速检查：

```bash
curl http://127.0.0.1:8018/healthz
```

## 注意事项

- 当前规则状态只保存在进程内存中，服务重启后缓存会丢失。
- `sftp.host_key_fingerprint` 需要填写服务端主机 key 的 SHA256 指纹；可用 `ssh-keyscan -p <port> <host> | ssh-keygen -lf - -E sha256` 获取。
- 原 PHP 中执行系统命令、远程写代码、打包日志、测试清表等高风险/维护接口没有迁移。
- 旧 `sftp.py` 仅作为历史实现参考。

### 上云规划

- 策略服务占用一台（包括策略包啥的）
- 日志推送占用一台
- DNS拦截 占用一台 (数据库一体)
