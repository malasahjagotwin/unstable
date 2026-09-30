# unstable

这项服务也正在迁移至中国。

Go 编写的单端点 REST API，通过 WebSocket 把任务分发给已连接的机器人执行。

## 结构

```
backend/
  主程序.go                 服务器：启动参数、路由、优雅退出
  任务分发.go                /fetch 处理逻辑
  机器人中枢.go                /rcon WebSocket 机器人集散器
  槽位.go                   并发槽位计数与到期回收
  配置加载.go                加载 users/methods/blacklist
  internal/arguments/     查询参数解析与校验
    参数.go                  Parse、ValidateHost、ValidateTime、ParseCmdTemplate
    访问控制.go               ValidateIP（支持 IP 与 CIDR）
    协议探测.go               协议探测
    参数_test.go
  internal/cli/          命令行帮助的中文输出
  internal/protocol/     REST API 与机器人共用的协议类型
  bot/机器人.go            机器人（独立 package main，只依赖 internal/protocol）
  配置/                   配置
  bin/                    编译产物（已 gitignore）
```

包目录 `internal/arguments`、`internal/cli`、`internal/protocol` 和 `bot` 保留 ASCII 名称：Go 的导入路径只接受 ASCII 字符，目录一旦用汉字就会报 `malformed import path`。文件名不受此限制，因此已全部改为汉字。

## 接口

### `GET|POST /fetch?key=&host=&time=&method=`

| 参数 | 规则 |
| --- | --- |
| `key` | 必须存在于 `配置/users.json` |
| `host` | 裸域名或带 `http://`、`https://` 前缀；禁止端口、路径、userinfo；仅允许 `[A-Za-z0-9.:/-]`。裸域名会先探测 443 端口的 TLS，失败降级为 HTTP |
| `time` | 仅数字，不得超过该 `key` 的上限 |
| `method` | 必须存在于 `配置/methods.json` 且 `status` 为 `true` |

校验顺序：`key` → IP 白名单 → `host` 格式 → 黑名单 → `time` → `method` → 槽位 → 广播。

状态码：`401` key 无效、`403` IP/黑名单/方法被禁用、`400` 参数非法、`429` 槽位占满、`503` 无可用机器人、`200` 成功。

### `/rcon`

隐藏的 WebSocket 端点，机器人接入。要求请求头携带 `X-Bot-Token`（由 `-bot-token` 或环境变量 `BOT_TOKEN` 指定）。任务广播给**所有**已连接机器人。

## 配置

`配置/users.json` — 每个 `key` 的时长上限、并发槽位（`slot`）、IP 白名单。

`配置/methods.json` — 可用方法。`cmd` 模板中的 `{host}` 与 `{time}` 会被替换。

`配置/blacklist.json` — 禁止访问的域名，按主机名匹配，忽略协议与大小写。

## 构建与运行

```bash
cd backend
go test ./...
go build -o bin/api ./
go build -o bin/bot ./bot

./bin/api -addr :8080 -json-dir 配置 -bot-token <secret>
./bin/bot -endpoint ws://127.0.0.1:8080/rcon -token <secret> -work-dir /path/to/lui
```

参数 `-addr`、`-json-dir`、`-bot-token`、`-probe-timeout`、`-slot-reap-interval` 详见 `./bin/api -h`。

## 已知缺口

`配置/methods.json` 指向的 `./liu` 尚未实现。机器人按模板执行 `argv[0]`，因此该文件必须存在于机器人的 `-work-dir` 下，否则任务会以 `status=failed` 上报。命令以参数数组直接执行，**不经过 shell**。
