# 上游 Mock

模拟上游管理中心，用于验证 fz 与上游的交互。

## 接口

| 方法 | 接口 | 方向 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/statusreport` | fz → 本 mock | fz 定时上报设备状态（心跳） |
| `POST` | `/upgrade/report` | fz → 本 mock | 全量规则消费反馈，fz 下载并转发下游后上报 |

## 运行

```powershell
# server 模式：接收 fz 上报
go run ./test/upstream
```

配置文件为同目录下的 `config.yaml`，命令行参数优先于配置。
