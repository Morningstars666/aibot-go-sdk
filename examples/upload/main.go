package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	aibot "github.com/morningstars666/wecom-aibot-go-sdk"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "用法: %s <图片路径> <chatid>\nchatid 为群聊 ID，单聊时填用户 userid 并去掉 -group 参数\n", os.Args[0])
		os.Exit(1)
	}
	path := os.Args[1]
	chatID := os.Args[2]

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开文件失败: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	client := aibot.NewWSClient(&aibot.Options{
		BotID:  os.Getenv("WECHAT_BOT_ID"),
		Secret: os.Getenv("WECHAT_BOT_SECRET"),
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "连接失败: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	fmt.Println("正在上传图片...")
	result, err := client.UploadMedia(ctx, aibot.MediaTypeImage, filepath.Base(path), f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "上传失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("上传成功 media_id=%s created_at=%d\n", result.MediaID, result.CreatedAt)

	fmt.Println("正在主动推送图片消息...")
	if _, err := client.SendMessage(ctx, chatID, map[string]any{
		"msgtype": "image",
		"image":   map[string]any{"media_id": result.MediaID},
	}, aibot.ChatTypeSingle); err != nil {
		fmt.Fprintf(os.Stderr, "发送失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("发送成功")

	<-ctx.Done()
}
