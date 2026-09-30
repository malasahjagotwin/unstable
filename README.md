# unstable

这项服务也正在迁移至中国。

Go 编写的单端点 REST API，通过 WebSocket 把任务分发给已连接的机器人执行。

## 结构

```
backend/
  main.go                 服务器：启动参数、路由、优雅退出
  fetch.go                /fetch 处理逻辑
  hub.go                  /rcon WebSocket 机器人集散器
  slots.go                并发槽位计数与到期回收
  config.go               加载 users/methods/blacklist
  arguments/              查询参数解析与校验
    arguments.go            Parse、ValidateHost、ValidateTime、ParseCmdTemplate
    access.go               ValidateIP（支持 IP 与 CIDR）
    probe.go                协议探测
    arguments_test.go
  internal/protocol/     REST API 与机器人共用的协议类型
  connections/bot.go      机器人（独立 package main，只依赖 internal/protocol）
  json/                   配置
  bin/                    编译产物（已 gitignore）
```

## 接口

### `GET|POST /fetch?key=&host=&time=&method=`

| 参数 | 规则 |
| --- | --- |
| `key` | 必须存在于 `json/users.json` |
| `host` | 裸域名或带 `http://`、`https://` 前缀；禁止端口、路径、userinfo；仅允许 `[A-Za-z0-9.:/-]`。裸域名会先探测 443 端口的 TLS，失败降级为 HTTP |
| `time` | 仅数字，不得超过该 `key` 的上限 |
| `method` | 必须存在于 `json/methods.json` 且 `status` 为 `true` |

校验顺序：`key` → IP 白名单 → `host` 格式 → 黑名单 → `time` → `method` → 槽位 → 广播。

状态码：`401` key 无效、`403` IP/黑名单/方法被禁用、`400` 参数非法、`429` 槽位占满、`503` 无可用机器人、`200` 成功。

### `/rcon`

隐藏的 WebSocket 端点，机器人接入。要求请求头携带 `X-Bot-Token`（由 `-bot-token` 或环境变量 `BOT_TOKEN` 指定）。任务广播给**所有**已连接机器人。

## 配置

`json/users.json` — 每个 `key` 的时长上限、并发槽位（`slot`）、IP 白名单。

`json/methods.json` — 可用方法。`cmd` 模板中的 `{host}` 与 `{time}` 会被替换。

`json/blacklist.json` — 禁止访问的域名，按主机名匹配，忽略协议与大小写。

## 构建与运行

```bash
cd backend
go test ./...
go build -o bin/api ./
go build -o bin/bot ./connections

./bin/api -addr :8080 -json-dir json -bot-token <secret>
./bin/bot -endpoint ws://127.0.0.1:8080/rcon -token <secret> -work-dir /path/to/lui
```

参数 `-addr`、`-json-dir`、`-bot-token`、`-probe-timeout`、`-slot-reap-interval` 详见 `./bin/api -h`。

## 已知缺口

`json/methods.json` 指向的 `./liu` 尚未实现。机器人按模板执行 `argv[0]`，因此该文件必须存在于机器人的 `-work-dir` 下，否则任务会以 `status=failed` 上报。命令以参数数组直接执行，**不经过 shell**。
