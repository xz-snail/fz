# 下游 Mock

模拟下游服务，接收 fz 转发的规则接口。

## 接口

| 方法 | 接口 | 方向 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/rule/increment` | fz → 本 mock | 增量规则转发 |
| `POST` | `/rule/upgrade` | fz → 本 mock | 全量规则转发，fz 下载规则包成功后发起 |
| `POST` | `/upgrade/redirect` | fz → 本 mock | 重定向规则转发 |

## 运行

```powershell
# 正常接收 fz 转发
go run ./test/downstream

# 模拟下游失败，验证 fz 的失败累计与重试
go run ./test/downstream -fail
```

配置文件为同目录下的 `config.yaml`，命令行参数优先于配置。
