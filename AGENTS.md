# 项目规则

## 语言

- 一律使用中文回复，包括解释、提问选项和总结。
- 面向用户的文案一律用中文：命令行参数说明、日志输出、启动与配置错误、HTTP 接口返回的错误信息。
- 不需要翻译的部分：标识符名、JSON 字段名、查询参数名（`key`/`host`/`time`/`method`）、协议字段名（`status`/`exit_code`/`stdout` 等）、协议头名（`X-Bot-Token`）、URL。
- Go 的 `flag` 包不做本地化，未知标志会输出英文。因此命令行帮助与解析错误由 `backend/internal/cli` 统一产出中文，新增标志时沿用 `cli.New` 与 `cli.Usage`，不要直接用全局 `flag`。

## 命名

- 提交到 GitHub 的文件名一律使用汉字，例如 `任务分发.go`、`槽位.go`、`参数/访问控制.go`。
- 包目录必须保留 ASCII 名称（`internal/arguments`、`internal/cli`、`internal/protocol`、`bot`）：Go 的导入路径只接受 ASCII 字符，目录名用汉字会报 `malformed import path`，项目无法构建。
- 以下文件名由工具链硬编码，不可改名：`go.mod`、`go.sum`、`.gitignore`、`README.md`、`AGENTS.md`。
- Go 标识符、包名与 module 路径保持 ASCII：`package main`、`package arguments` 以及 `unstablestress/backend/...`。
- 用 `git config core.quotepath false` 让 `git ls-files` 正常显示汉字路径。

## 代码规范

- 代码任何时候都必须整洁、可读、结构清晰。
- 优先写自解释的代码：命名有描述性、函数职责单一、用提前返回代替深层嵌套。

## 注释

- 禁止使用 `//` 注释，包括以下所有形式：
  - 独占一行的注释
  - 行尾注释
  - 文件顶部的注释块
  - 导出标识符的文档注释
  - JSON、JSONC、YAML 和配置文件内部的注释
- 确实需要说明时，使用 `/* ... */` 块注释，并单独成行放在被说明代码的上方。
- 如果加注释反而让文件比代码本身更难读，那就不要写注释，改为提取一个命名清晰的函数。
- 不要添加 merely 复述代码行为的注释。

## 验证

- 每次改动后，都要运行项目的 build、lint 和 test 命令。如果没有配置这些命令，就明确说明，不要报告成功。
- 除非真正编译或执行过，否则不要声称改动可用。
