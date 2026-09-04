package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	aibot "github.com/morningstars666/wecom-aibot-go-sdk"
)

func main() {
	client := aibot.NewWSClient(&aibot.Options{
		BotID:  os.Getenv("WECHAT_BOT_ID"),
		Secret: os.Getenv("WECHAT_BOT_SECRET"),
		Logger: &aibot.DefaultLogger{Debug: false},
	})

	client.OnConnected(func() {
		fmt.Println("WebSocket 连接已建立")
	})

	client.OnAuthenticated(func() {
		fmt.Println("认证成功")
	})

	client.OnDisconnected(func(reason string) {
		fmt.Printf("连接断开: %s\n", reason)
	})

	client.OnReconnecting(func(attempt int) {
		fmt.Printf("正在重连（第 %d 次）\n", attempt)
	})

	client.OnError(func(err error) {
		fmt.Printf("发生错误: %v\n", err)
	})

	client.OnEventEnterChat(func(frame *aibot.Frame) {
		_, err := client.ReplyWelcome(context.Background(), frame, map[string]any{
			"msgtype": "text",
			"text": map[string]any{
				"content": "您好！我是智能助手，有什么可以帮您的吗？",
			},
		})
		if err != nil {
			fmt.Printf("回复欢迎语失败: %v\n", err)
		}
	})

	client.OnMessageText(func(frame *aibot.Frame) {
		var body aibot.MsgBody
		if err := frame.ParseBody(&body); err != nil {
			fmt.Printf("解析消息失败: %v\n", err)
			return
		}
		content := ""
		if body.Text != nil {
			content = body.Text.Content
		}
		fmt.Printf("收到文本: %s\n", content)

		streamID := aibot.GenerateReqID("stream")
		ctx := context.Background()

		if _, err := client.ReplyStream(ctx, frame, streamID, "正在思考中...", false); err != nil {
			fmt.Printf("流式回复失败: %v\n", err)
			return
		}

		time.Sleep(1 * time.Second)

		if _, err := client.ReplyStream(ctx, frame, streamID, fmt.Sprintf("你好！你说的是: %q", content), true); err != nil {
			fmt.Printf("流式回复失败: %v\n", err)
		}
	})

	client.OnMessageImage(func(frame *aibot.Frame) {
		var body aibot.MsgBody
		if err := frame.ParseBody(&body); err != nil || body.Image == nil {
			return
		}
		data, err := client.DownloadFile(context.Background(), body.Image.URL, body.Image.AESKey)
		if err != nil {
			fmt.Printf("下载图片失败: %v\n", err)
			return
		}
		fmt.Printf("收到图片，解密后大小: %d bytes\n", len(data))
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := client.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("已退出")
}
