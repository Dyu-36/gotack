package zalo

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveBotToken(t *testing.T) {
	if os.Getenv("ZALO_LIVE_TEST") != "1" {
		t.Skip("set ZALO_LIVE_TEST=1 and ZALO_BOT_TOKEN to verify a real bot")
	}
	token := os.Getenv("ZALO_BOT_TOKEN")
	if token == "" {
		t.Fatal("ZALO_BOT_TOKEN is required for the live test")
	}
	client, err := NewClient(token)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	bot, err := client.GetMe(ctx)
	if err != nil {
		t.Fatalf("getMe: %v", err)
	}
	if bot.Name == "" {
		t.Fatal("getMe returned an empty bot name")
	}
}
