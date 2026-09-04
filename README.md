# aibot-go-sdk (Go)

企业微信智能机器人 Go SDK —— 基于 WebSocket 长连接通道，提供消息收发、流式回复、模板卡片、事件回调、临时素材上传、文件下载解密等核心能力。

> 本项目是 [wecom-aibot-python-sdk](https://pypi.org/project/wecom-aibot-python-sdk/)（Python 版）的 Go 等价实现，协议依据[企业微信官方《智能机器人长连接》文档](https://developer.work.weixin.qq.com/document/path/101463)。

## ✨ 特性

- 🔗 **WebSocket 长连接** — 基于 `wss://openws.work.weixin.qq.com` 默认地址，开箱即用
- 🔐 **自动认证** — 连接建立后自动发送订阅帧（bot_id + secret）
- 💓 **心跳保活** — 自动维护 30s JSON 心跳帧，连续未收到 ack 时判定连接异常并重建
- 🔄 **断线重连** — 指数退避重连策略（1s → 2s → 4s → ... → 30s 上限），支持自定义最大重连次数（-1 无限）
- 📨 **消息分发** — 自动解析消息类型并触发对应事件（text / image / mixed / voice / file / video）
- 🌊 **流式回复** — 内置流式回复方法（stream.id + finish 机制），支持 Markdown 内容
- 🃏 **模板卡片** — 支持回复模板卡片、流式+卡片组合回复、更新卡片
- 📤 **主动推送** — 支持向指定会话主动发送 Markdown / 模板卡片 / 图片 / 文件等消息，无需依赖回调帧
- 📡 **事件回调** — 支持进入会话、模板卡片按钮点击、用户反馈、连接被踢等事件
- ⏩ **串行回复队列** — 同一 req_id 的回复自动串行发送并等待回执
- 📦 **临时素材上传** — 分片上传（≤512KB/片，≤100 片）自动完成 init/chunk/finish 全流程，获取 media_id
- 🔑 **文件下载解密** — 内置 AES-256-CBC 解密（IV 取密钥前 16 字节，PKCS#7 按 32 字节倍数填充），每个图片/文件消息自带独立 aeskey
- 🪵 **可插拔日志** — 实现 `Logger` 接口即可自定义日志，内置带时间戳的 `DefaultLogger`
- 🐹 **纯 goroutine 并发** — 基于 context 的异步架构，阻塞等待服务端回执

## 📦 安装

```bash
go get github.com/morningstars666/aibot-go-sdk
```

**依赖：**
- Go >= 1.24
- github.com/coder/websocket

## ⚙️ 准备凭证

1. 在企业微信管理后台进入智能机器人的配置页面
2. 开启「API 模式」并选择「**长连接**」方式
3. 获取 **BotID** 与 **Secret**（长连接专用密钥，与回调地址模式的 Token/EncodingAESKey 不同）

> 注意：每个机器人同一时间只能保持一个有效长连接，新连接完成订阅后会踢掉旧连接（旧连接会收到 `disconnected_event`）。

## 🚀 快速开始

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	aibot "github.com/morningstars666/aibot-go-sdk"
)

func main() {
	// 1. 创建客户端实例
	client := aibot.NewWSClient(&aibot.Options{
		BotID:  os.Getenv("WECHAT_BOT_ID"),  // 企业微信后台获取的机器人 ID
		Secret: os.Getenv("WECHAT_BOT_SECRET"), // 企业微信后台获取的机器人 Secret
	})

	// 2. 监听认证成功
	client.OnAuthenticated(func() {
		fmt.Println("🔐 认证成功")
	})

	// 3. 监听文本消息并进行流式回复
	client.OnMessageText(func(frame *aibot.Frame) {
		var body aibot.MsgBody
		_ = frame.ParseBody(&body)
		content := body.Text.Content

		streamID := aibot.GenerateReqID("stream")
		ctx := context.Background()

		// 发送流式中间内容
		_, _ = client.ReplyStream(ctx, frame, streamID, "正在思考中...", false)

		// 发送最终结果（finish=true 结束流式消息）
		_, _ = client.ReplyStream(ctx, frame, streamID, fmt.Sprintf("你好！你说的是: %q", content), true)
	})

	// 4. 监听进入会话事件（发送欢迎语，需 5 秒内回复）
	client.OnEventEnterChat(func(frame *aibot.Frame) {
		_, _ = client.ReplyWelcome(context.Background(), frame, map[string]any{
			"msgtype": "text",
			"text":    map[string]any{"content": "您好！我是智能助手，有什么可以帮您的吗？"},
		})
	})

	// 5. 启动（阻塞运行，Ctrl+C 优雅退出，内部自动重连）
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := client.Run(ctx); err != nil {
		panic(err)
	}
}
```

完整可运行示例见 [examples/basic](examples/basic)（欢迎语 + 流式回复 + 图片下载解密）和 [examples/upload](examples/upload)（临时素材上传 + 主动推送图片）。

## 📖 API 文档

### `WSClient`

核心客户端结构体，提供连接管理、消息收发等功能。

#### 连接管理

| 方法 | 说明 |
| --- | --- |
| `NewWSClient(opts *Options)` | 创建客户端实例 |
| `Connect(ctx) error` | 建立连接并自动订阅认证（成功后自动开启心跳与断线重连） |
| `Run(ctx) error` | 便捷启动方法：连接（失败时自动重试）并阻塞运行，直到 ctx 取消或重连次数耗尽 |
| `Close()` | 主动断开连接并停止重连 |
| `IsConnected() bool` | 当前 WebSocket 连接状态 |
| `IsAuthenticated() bool` | 订阅认证状态 |

#### 回复与推送（均阻塞等待服务端回执，超时由 `RequestTimeout` 控制）

| 方法 | 说明 |
| --- | --- |
| `Reply(ctx, frame, body, cmd...)` | 通用回复：透传回调帧的 req_id 发送回复（默认 cmd 为 `aibot_respond_msg`） |
| `ReplyStream(ctx, frame, streamID, content, finish, opts...)` | 发送流式文本回复（支持 Markdown） |
| `ReplyWelcome(ctx, frame, body)` | 回复欢迎语（文本或模板卡片），需在收到 enter_chat 事件 5 秒内调用 |
| `ReplyTemplateCard(ctx, frame, card, feedback...)` | 回复模板卡片消息 |
| `ReplyStreamWithCard(ctx, frame, streamID, content, finish, opts)` | 发送流式消息 + 模板卡片组合回复 |
| `UpdateTemplateCard(ctx, frame, card, userids...)` | 更新模板卡片，需在收到 template_card_event 事件 5 秒内调用 |
| `SendMessage(ctx, chatid, body, chatType...)` | 主动推送消息（无需依赖回调帧），chatType：1 单聊 / 2 群聊 / 0 自动 |

#### 素材与文件

| 方法 | 说明 |
| --- | --- |
| `UploadMedia(ctx, mediaType, filename, r io.Reader)` | 上传临时素材（自动分片），返回 media_id（3 天有效）。类型：`image` ≤10MB、`voice` ≤2MB、`video` ≤10MB、`file` ≤20MB |
| `DownloadFile(ctx, url, aesKey)` | 下载文件并使用 AES-256-CBC 密钥解密（aesKey 取自消息体，URL 5 分钟内有效） |
| `ExtractFilenameFromURL(url)` | 从下载 URL 中解析文件名 |

#### 使用示例

**流式回复详细说明**

```go
// streamID 建议用 GenerateReqID("stream") 生成
// 首次使用某 streamID 回复 → 创建新的流式消息
// 继续用相同 streamID 推送 → 刷新该流式消息内容
// finish=true → 结束流式消息（10 分钟内必须完成）
_, _ = client.ReplyStream(ctx, frame, streamID, "正在为您查询天气信息...", false)

// 高级选项（仅 finish=true 时 msg_item 有效；feedback 仅首次回复时设置）
_, _ = client.ReplyStream(ctx, frame, streamID, "最终内容", true, &aibot.StreamOptions{
	MsgItem:  nil,
	Feedback: &aibot.Feedback{ID: "FEEDBACKID"},
})
```

**主动推送详细说明**

```go
// 群聊：chatid 填回调事件中获取的 chatid
_, _ = client.SendMessage(ctx, chatID, map[string]any{
	"msgtype":  "markdown",
	"markdown": map[string]any{"content": "这是一条**主动推送**的消息"},
}, aibot.ChatTypeGroup)

// 单聊：chatid 填用户 userid
_, _ = client.SendMessage(ctx, userID, map[string]any{
	"msgtype": "template_card",
	"template_card": map[string]any{
		"card_type":  "text_notice",
		"main_title": map[string]any{"title": "通知"},
	},
}, aibot.ChatTypeSingle)
```

**文件下载解密使用示例**

```go
client.OnMessageImage(func(frame *aibot.Frame) {
	var body aibot.MsgBody
	_ = frame.ParseBody(&body)
	// aesKey 取自 image.aeskey，每个下载链接唯一
	data, err := client.DownloadFile(ctx, body.Image.URL, body.Image.AESKey)
	if err != nil {
		return
	}
	_ = os.WriteFile("image.png", data, 0o644)
})
```

## ⚙️ 配置选项

`Options` 完整配置（零值字段自动使用默认值）：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `BotID` | `string` | ✅ | — | 机器人 ID（企业微信后台获取） |
| `Secret` | `string` | ✅ | — | 机器人长连接专用 Secret |
| `WsURL` | `string` | — | `wss://openws.work.weixin.qq.com` | 自定义 WebSocket 连接地址 |
| `HeartbeatInterval` | `time.Duration` | — | `30s` | 心跳间隔（官方建议 30 秒） |
| `HeartbeatMaxMisses` | `int` | — | `3` | 连续心跳无 ack 次数上限，达到后强制重建连接 |
| `ReconnectBaseInterval` | `time.Duration` | — | `1s` | 重连基础延迟，实际按指数退避递增 |
| `ReconnectMaxInterval` | `time.Duration` | — | `30s` | 重连延迟上限 |
| `MaxReconnectAttempts` | `int` | — | `10` | 最大重连次数（`-1` 表示无限重连） |
| `RequestTimeout` | `time.Duration` | — | `10s` | 单次请求等待服务端回执的超时时间 |
| `HTTPTimeout` | `time.Duration` | — | `60s` | 文件下载 HTTP 超时 |
| `AutoReconnect` | `bool` | — | `true` | 断线后是否自动重连 |
| `Logger` | `Logger` | — | `&DefaultLogger{}` | 自定义日志实例 |

## 📡 事件列表

所有消息/事件通过回调注册方式监听：

| 方法 | 回调参数 | 说明 |
| --- | --- | --- |
| `OnConnected(func())` | — | WebSocket 连接建立 |
| `OnAuthenticated(func())` | — | 订阅认证成功 |
| `OnDisconnected(func(reason string))` | `reason` | 连接断开 |
| `OnReconnecting(func(attempt int))` | `attempt` | 正在重连（第 N 次） |
| `OnError(func(err error))` | `err` | 发生错误（如重连次数耗尽） |
| `OnMessage(func(*Frame))` | `frame` | 收到消息回调（所有类型） |
| `OnMessageText` / `OnMessageImage` / `OnMessageMixed` / `OnMessageVoice` / `OnMessageFile` / `OnMessageVideo` | `frame` | 收到对应类型消息 |
| `OnEvent(func(*Frame))` | `frame` | 收到事件回调（所有事件类型） |
| `OnEventEnterChat` | `frame` | 用户当天首次进入机器人单聊会话 |
| `OnEventTemplateCard` | `frame` | 用户点击模板卡片按钮 |
| `OnEventFeedback` | `frame` | 用户对机器人回复进行反馈 |
| `OnEventDisconnected` | `frame` | 有新连接建立，当前旧连接被系统断开 |

也可以使用通用方法 `On(event string, h func(*Frame))` 注册事件，事件名格式为 `"message"`、`"message."+消息类型`、`"event"`、`"event."+事件类型`。

> 回调 handler 在独立 goroutine 中执行（已做 panic 隔离），不要阻塞过久；handler 内可安全调用 client 的回复方法。

## 📋 消息与事件类型

SDK 内置以下常量（`commands.go`）：

| 分类 | 常量 | 值 |
| --- | --- | --- |
| 消息类型 | `MsgTypeText` / `MsgTypeImage` / `MsgTypeMixed` / `MsgTypeVoice` / `MsgTypeFile` / `MsgTypeVideo` | `text` / `image` / `mixed` / `voice` / `file` / `video` |
| 回复消息类型 | `MsgTypeStream` / `MsgTypeMarkdown` / `MsgTypeTemplateCard` | `stream` / `markdown` / `template_card` |
| 事件类型 | `EventTypeEnterChat` / `EventTypeTemplateCard` / `EventTypeFeedback` / `EventTypeDisconnected` | `enter_chat` / `template_card_event` / `feedback_event` / `disconnected_event` |
| 发送会话类型 | `ChatTypeSingle` / `ChatTypeGroup` / `ChatTypeAuto` | `1` / `2` / `0` |
| 素材类型 | `MediaTypeImage` / `MediaTypeVoice` / `MediaTypeVideo` / `MediaTypeFile` | `image` / `voice` / `video` / `file` |

## 🪵 自定义日志

实现 `Logger` 接口即可自定义日志输出：

```go
type MyLogger struct{}

func (MyLogger) Debugf(format string, args ...any) {} // 静默 debug 日志

func (MyLogger) Infof(format string, args ...any) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func (MyLogger) Warnf(format string, args ...any) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}

func (MyLogger) Errorf(format string, args ...any) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}

client := aibot.NewWSClient(&aibot.Options{
	BotID:  "your-bot-id",
	Secret: "your-bot-secret",
	Logger: MyLogger{},
})
```

## ⏱️ 协议时效约束（SDK 已内置，业务需知晓）

| 约束 | 值 |
| --- | --- |
| 欢迎语 / 更新模板卡片回复窗口 | 收到事件后 **5 秒** 内 |
| 流式消息完成（finish=true） | 首次发送后 **10 分钟** 内 |
| 消息媒体下载 URL 有效期 | **5 分钟** |
| 临时素材 media_id 有效期 | **3 天** |
| 消息回复窗口 | 收到消息回调后 **24 小时** |
| 回复 + 主动推送频率 | 每会话 30 条/分钟、1000 条/小时 |
| 上传会话有效期 | **30 分钟**（断线重连后未过期可续传） |

## 📂 项目结构

```
aibot-go-sdk/
├── client.go            # WSClient 核心客户端（连接/订阅/心跳/重连/请求-响应关联）
├── dispatcher.go        # 消息解析与事件分发
├── reply.go             # 回复消息（流式/欢迎语/模板卡片/主动推送）
├── upload.go            # 临时素材分片上传
├── download.go          # 文件下载
├── crypto.go            # AES-256-CBC 文件解密
├── frame.go             # 协议帧与消息体类型定义
├── commands.go          # cmd 常量与类型枚举
├── options.go           # 配置选项
├── reqid.go             # req_id 生成（UUID v4）
├── logger.go            # 日志接口与默认实现
├── errors.go            # 错误定义
└── examples/
    ├── basic/           # 基础使用示例
    └── upload/          # 素材上传与主动推送示例
```

## 📄 License

MIT
