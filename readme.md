# TODO
- this tanslator is over complex, so far, I think
youdao and sogou is fine for me even though they translate a little bit wrong.


# sync-email-boxing(LLM Generated code, if you don't trust llm, don't use)
基于 [bubbletea](https://github.com/charmbracelet/bubbletea) 的 QQ / 163 等邮箱**收件箱 TUI 客户端**：
多账号配置、收件箱列表、中文正文阅读、附件下载、剪贴板复制，全程支持调试日志与本地缓存（二次打开不用重新拉取）。

## 功能

- **多账号**：添加 / 切换 / 删除账号，配置长期存放在 `~/.config/sync-email-boxing/config.json`（0600 权限保护授权码），
  完整的配置结构示例见 [sample-config/](sample-config/)
- **外部配置导入**：`-c` 参数兼容 [go-sendemail](https://github.com/your/go-sendemail) 风格的
  `qq_email_config.json` / `163_email_config.json`（`sender_email/sender_key/smtp_server/smtp_port`），
  只填 SMTP 的会按邮箱域名自动推导 IMAP 服务器
- **收件箱列表**：IMAP TLS 只读连接，拉取最近邮件（新邮件在前），未读加粗 `●`，支持文件夹切换
- **中文渲染**：RFC2047 encoded-word、GBK/GB2312/GB18030/Big5 字符集、quoted-printable / base64
  传输编码全部正确解码；纯 HTML 邮件自动剥离标签
- **附件**：查看页列出附件名与大小，一键下载解码后的二进制内容
- **剪贴板**：复制正文、复制网页邮箱链接（粘贴到浏览器打开）；`wl-copy` → `xclip` → `xsel` →
  OSC52 终端回退（SSH 远程也可用）
- **翻译**：正文查看页按 `t` 把英文/其他语言的邮件正文翻译成中文（目标语言可配置），
  支持 sogou（默认）/ 有道智云 / 自定义 HTTP API；译文随正文缓存，重复查看不联网
- **搜索**：收件箱按 `/` 输入关键词（发件人/主题/日期，不分大小写）实时过滤，
  `Esc` 清除；过滤状态下 ↑/↓、Enter、g/G 都在匹配结果内导航
- **缓存**：正文、附件、账号均本地缓存，重新打开秒开；`r` 可强制刷新
- **调试日志**：`-v` 输出连接/登录/拉取耗时等详细日志，默认写入 `~/.config/sync-email-boxing/debug.log`

## 快速开始

```bash
make build
./sync-email-boxing    # 首次启动会进入账号添加表单
```

已有现成配置文件的话，直接导入：

```bash
./sync-email-boxing -c ../sync-between-linux-and-mobile/config/qq_email_config.json
./sync-email-boxing -c qq_email_config.json,163_email_config.json   # 一次导多个
./sync-email-boxing -c 139_email_config.json                        # 中国移动 139 邮箱
```

> QQ / 163 / 新浪邮箱需先在网页设置中开启 **IMAP/SMTP 服务**并生成**授权码**（不是登录密码）。
> 139 邮箱在「设置 → 收发信设置」开启 IMAP/SMTP 后，一般直接用登录密码即可。
> 联通（wo.cn）用「专用密码」，天翼（189.cn）用「客户端授权码」，均在网页版设置区生成。
> 完整脱敏示例见 [sample-config/config.json](sample-config/config.json)，覆盖
> 139 / QQ / 163 / 126 / 联通 wo.cn / 新浪 / 天翼 189.cn。

**内置服务器推导**（配置里不写 `smtp_server`/`imap_server` 时按域名自动填）：

| 域名 | IMAP | SMTP |
|------|------|------|
| `qq.com` / `foxmail.com` | `imap.qq.com:993` | `smtp.qq.com:465` |
| `163.com` / `126.com` | `imap.163.com:993` / `imap.126.com:993` | `smtp.163.com:465` / `smtp.126.com:465` |
| `139.com` | `imap.139.com:993` | `smtp.139.com:465` |
| `wo.cn` | `imap.wo.cn:993` | `smtp.wo.cn:465` |
| `sina.com` / `sina.cn` | `imap.sina.com:993` | `smtp.sina.com:465` |
| `189.cn` | `imap.189.cn:993` | `smtp.189.cn:465` |
| `gmail.com` | `imap.gmail.com:993` | `smtp.gmail.com:465` |

> 注意天翼邮箱是 `189.cn` 而不是 `189.com`（后者域名已废弃，登录会报 `domain is disable`）。

139 邮箱的 IMAP 网关（`imap.139.com`）只接受 **RSA 密钥交换**的 TLS 1.2 套件，而 Go 1.22+
默认已禁用这类套件。程序会自动检测握手失败并回退到 RSA 套件重试（日志里可见
「TLS 回退握手成功」），无需手动配置。

## 查看本地 .eml 文件

```bash
./sync-email-boxing view 邮件.eml        # 或 sync-email-boxing eml 邮件.eml
```

独立的 TUI 查看器，同样支持中文解码（RFC2047/GBK/Big5/QP/base64）与附件：

`↑/↓ PgUp/PgDn` 滚动 · `g/G` 顶/底 · `h` 精简/完整头信息切换 · `c` 复制正文 · `f` 复制发件人地址 · `d` 下载附件到 `~/Downloads/sync-email-boxing/eml-view/<文件名>/` · `q` 退出

## 启动参数

```
sync-email-boxing — QQ/163 等邮箱收件箱 TUI

用法:
  sync-email-boxing [flags]          启动收件箱 TUI
  sync-email-boxing accounts         列出已配置的邮箱账号
  sync-email-boxing view <file.eml>  查看本地 .eml 文件
  sync-email-boxing reset [目标]     清空缓存/配置（默认 all，见 reset -h）
```

## 列出已配置账号

```bash
sync-email-boxing accounts
```

输出（`●` 为当前账号，序号可直接给 `-u`）：

```
配置文件: /home/you/.config/sync-email-boxing/config.json
● 1. QQ工作号       1690544550@qq.com            imap imap.qq.com:993
  2. 139           13532932349@139.com           imap imap.139.com:993
```

## 重置（清空缓存 / 配置）

```bash
sync-email-boxing reset          # 清空整个 ~/.config/sync-email-boxing（配置+缓存+日志），恢复出厂
sync-email-boxing reset cache    # 只清正文/附件缓存（保留账号，下次重新拉取）
sync-email-boxing reset config   # 只删账号配置（保留缓存）
sync-email-boxing reset log      # 只删调试日志
sync-email-boxing reset -y       # 跳过确认直接执行
```

不带 `-y` 时会先显示将删除的路径和大小，输入 `y` 确认、其他任意输入取消。
重置后下次启动回到首次运行状态（进入账号添加表单）。


| 参数 | 说明 |
|------|------|
| `-c, --config <file>` | 从外部配置文件导入账号（可重复或逗号分隔），并存入长期配置 |
| `-u, --user <账号>` | 启动时切换到指定账号（邮箱地址或 1 起始序号，先 `accounts` 查看） |
| `-o, --output <dir>` | 附件 / `.eml` 下载目录（默认 `~/Downloads/sync-email-boxing`） |
| `-v, --debug` | 开启调试日志（开发者模式，**默认关闭**，不产生日志文件） |
| `-log <file>` | 调试日志文件路径（默认 `~/.config/sync-email-boxing/debug.log`，需 `-v` 开启） |

日常使用不需要任何参数；只有排查连接/登录/拉取耗时等问题时才加 `-v` 开发者模式，
日志（连接、登录、拉取、耗时统计）写入 `debug.log`，界面里按 `L` 也可查看当前日志开关与路径。

## 配置格式

### 长期配置 `~/.config/sync-email-boxing/config.json`（0600）

```json
{
  "accounts": [
    {
      "name": "QQ 工作邮箱",
      "email": "1690544550@qq.com",
      "auth_code": "你的授权码",
      "smtp_server": "smtp.qq.com",
      "smtp_port": 465,
      "imap_server": "imap.qq.com",
      "imap_port": 993
    },
    {
      "name": "163",
      "email": "me@163.com",
      "auth_code": "你的授权码",
      "smtp_server": "smtp.163.com",
      "smtp_port": 465,
      "imap_server": "imap.163.com",
      "imap_port": 993
    }
  ],
  "current": 0
}
```

| 字段 | 说明 |
|------|------|
| `accounts[].name` | 显示昵称，缺省用邮箱地址 |
| `accounts[].email` | 邮箱地址（唯一标识，去重依据） |
| `accounts[].auth_code` | SMTP/IMAP 共用的授权码（非登录密码），文件权限 0600 |
| `accounts[].smtp_server` / `smtp_port` | 发信服务器；SMTP 目前未使用，保留供后续扩展 |
| `accounts[].imap_server` / `imap_port` | 收信服务器（TLS），收件箱功能依赖此项 |
| `current` | 当前账号在 `accounts` 中的下标 |
| `translate_target` / `translate_source` | 已废弃，自动迁移到 `translators.json` |

## 翻译设置（translators.json）

翻译配置**不放在 config.json**，单独存 `~/.config/sync-email-boxing/translators.json`，
加多少翻译源都不会把账号配置撑大。旧版 `config.json` 里的 `translate_*` 字段会在启动时
自动迁移过来并从 config.json 删除。

```json
{
  "active": "youdao",
  "target": "zh-CN",
  "providers": {
    "youdao": {
      "type": "youdao",
      "app_key": "你的 appKey",
      "app_secret": "你的 appSecret"
    },
    "my-api": {
      "type": "http",
      "url": "https://api.example.com/translate",
      "method": "POST",
      "keys": { "api_key": "sk-xxxx" },
      "headers": { "Authorization": "Bearer {{.Keys.api_key}}" },
      "body": "{\"text\": \"{{.Text}}\", \"to\": \"{{.Target}}\"}",
      "result_path": "data.result"
    }
  }
}
```

- `active`：首选源名，失败自动回退 `sogou`（`sogou` 无需任何密钥，永远可用）
- `target`：目标语言，默认 `zh-CN`，支持 `en/ja/ko/ru/de/fr/es`
- `providers.<名字>.type`：`sogou` / `youdao` / `llm` / `http`；不写时按名字推断，其余按 `http`
- 内置 `youdao` 也可改用 `"keys": {"app_key": "...", "app_secret": "..."}`

### LLM 翻译（NOT test just an idea）

内置 `llm` 类型对接**所有 OpenAI 兼容的 `/chat/completions` 接口**，用大模型翻邮件
通常比网页逆向 API 更稳、译文更自然。三个字段就够：`url` + `model` + `keys.api_key`。

**DeepSeek：**
```json
"deepseek": {
  "type": "llm",
  "url": "https://api.deepseek.com/chat/completions",
  "model": "deepseek-chat",
  "keys": { "api_key": "sk-xxxx" }
}
```

**豆包（火山方舟 Ark）：**
```json
"doubao": {
  "type": "llm",
  "url": "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
  "model": "doubao-pro-32k",
  "keys": { "api_key": "你的 Ark API Key" }
}
```

**通义千问（DashScope 兼容模式）：**
```json
"qwen": {
  "type": "llm",
  "url": "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
  "model": "qwen-plus",
  "keys": { "api_key": "sk-xxxx" }
}
```

本地模型（Ollama 等）同样适用：`"url": "http://localhost:11434/v1/chat/completions"`,
`"model": "qwen2.5"`, api_key 随便填一个（如 `"ollama"`）。

- 默认系统提示词要求模型只输出译文；可用 `keys.system` 自定义提示词
- 自动去除模型偶尔输出的 markdown 围栏和引号
- 配好后 TUI 里按 `S` 即可切换到 LLM 源，`S` 循环所有已配置的源，并写回 translators.json

### 关于密钥（重要）

国内可用的翻译 API 基本都要密钥：有道、百度、腾讯、阿里都需要 `app_key + app_secret`
（或 `appid + key`），因为服务端要校验请求签名——**只有 sogou 这个网页端点不需要**。
所以自定义源必须能带凭据并计算签名，这不需要 JS 引擎，模板函数就够了：

| 模板变量 | 含义 |
|----------|------|
| `{{.Text}}` | 原文 |
| `{{.Target}}` / `{{.To}}` | 目标语言码 |
| `{{.From}}` | 源语言（固定 `auto`） |
| `{{.Keys.xxx}}` | 该 provider 的 `keys` 里的任意凭据 |
| `{{.APIKey}}` | 等价 `{{.Keys.api_key}}` |
| `{{.Salt}}` / `{{.CurTime}}` | 每次请求固定的随机串 / 秒级时间戳（参数与签名共用同一个值） |

| 模板函数 | 用途 |
|----------|------|
| `md5` / `sha1` / `sha256` | 十六进制摘要 |
| `hmacSHA256 <key> <msg>` | HMAC-SHA256（腾讯/阿里系签名常用） |
| `b64` / `b64url` | Base64 |
| `upper` / `lower` | 大小写 |
| `printf` / `randhex` / `now` | 字符串拼接、随机 hex、时间戳 |

`result_path` 是响应 JSON 的提取路径（gjson 语法，如 `trans_result.0.dst`），留空则整个响应当译文。

**例：百度翻译**（`sign = md5(appid + q + salt + key)`，完全不用写代码）：

```json
"baidu": {
  "type": "http",
  "url": "https://fanyi-api.baidu.com/api/trans/vip/translate",
  "method": "GET",
  "keys": { "appid": "你的appid", "key": "你的密钥" },
  "query": {
    "q": "{{.Text}}",
    "from": "{{.From}}",
    "to": "{{.Target}}",
    "appid": "{{.Keys.appid}}",
    "salt": "{{.Salt}}",
    "sign": "{{md5 (printf \"%s%s%s%s\" .Keys.appid .Text .Salt .Keys.key)}}"
  },
  "result_path": "trans_result.0.dst"
}
```

文件不存在或字段缺失时程序自动补默认值：按邮箱域名推导
qq/163/126/**139**/gmail 的服务器（见 `internal/config/config.go` 的 `DefaultServers`），
TUI 表单里填完邮箱按 `Tab` 也会自动补全。

### 外部导入格式 `-c`

兼容 go-sendemail 的极简格式（只有 SMTP 时 IMAP 按域名自动推导）：

```json
{
  "sender_email": "1690544550@qq.com",
  "sender_key": "你的授权码",
  "smtp_server": "smtp.qq.com",
  "smtp_port": 465
}
```

同时兼容上面的完整格式（`email/auth_code/imap_server/...`）。导入规则：
按 `email` 去重，已存在则覆盖更新（保留自定义昵称），最后导入的账号设为当前账号，
立即写回长期配置。

## 按键

### 收件箱列表
`↑/↓` 选择 · `Enter` 查看 · `/` 搜索过滤（输入即过滤，`Esc` 清除） · `r` 刷新 · `f` 切换文件夹 · `s` 账号列表 · `a` 添加账号 · `S` 切换翻译源 · `L` 日志/缓存信息 · `q` 退出

### 邮件正文
`↑/↓ PgUp/PgDn` 滚动 · `g/G` 顶/底 · `c` 复制正文 · `t` 翻译/切回原文 · `T` 忽略缓存用当前首选源重译 · `S` 切换翻译源 · `y` 复制网页邮箱链接 · `d` 下载附件 · `e` 导出整封 `.eml` · `Esc` 返回

正文页标题栏常驻显示 `翻译源 <当前首选> → <目标语言>`；译文分隔线也会标明实际使用的源，
状态栏在首选源失败回退到 sogou 时会明确提示。

### 账号列表
`Enter` 切换账号 · `a` 添加 · `d` 删除 · `Esc` 返回

### 文件夹
`Enter` 打开 · `r` 刷新 · `Esc` 返回

### 添加账号表单
`Tab/Shift+Tab` 切换字段 · `Enter` 下一项/保存 · 填完邮箱按 `Tab` 会按域名自动补全 qq/163/126/gmail 的默认服务器

## 构建

```bash
make build     # 当前平台
make release   # 交叉编译 linux/darwin/windows 到 dist/
make test      # 单元测试（中文解码、配置导入等）
make vet       # go vet
make install   # 安装到 /usr/local/bin
```

## 目录

```
sync-email-boxing/
├── main.go                     # 入口：启动参数、view 子命令、配置导入、TUI 启动
├── Makefile
└── internal/
    ├── config/                 # 多账号配置 + 外部文件导入 + 网页邮箱链接推导
    ├── mail/                   # IMAP 拉取、MIME/RFC2047/GBK 解析、附件提取、.eml 解析
    ├── cache/                  # 正文/附件磁盘缓存
    ├── save/                   # 附件与 .eml 导出（文件名清洗、重名处理）
    ├── clipboard/              # wl-copy/xclip/xsel/OSC52 剪贴板
    ├── translate/              # 邮件正文翻译（sogou / 有道智云 / 自定义 HTTP API）
    ├── debug/                  # 调试日志
    └── ui/                     # bubbletea 各视图（列表/正文/表单/账号/文件夹/.eml 查看器）
```

## 数据存放

| 路径 | 内容 |
|------|------|
| `~/.config/sync-email-boxing/config.json` | 账号配置（含授权码，0600） |
| `~/.config/sync-email-boxing/debug.log` | 调试日志（仅 `-v`/`--debug` 开启时产生） |
| `~/.config/sync-email-boxing/cache/` | 正文/附件缓存 |
| `~/Downloads/sync-email-boxing/` | 附件与 `.eml` 导出（可用 `-o` 改，不受 `reset` 影响） |

以上 `~/.config/sync-email-boxing` 下的内容都可用 `sync-email-boxing reset` 一键清理。


# ref links
- codebuddy: https://www.codebuddy.cn/
- youdao translate provider
- sogou translate provider
- anyway, thank you all.