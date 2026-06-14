package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hibiken/asynq"
	tele "gopkg.in/telebot.v3"
)

type AITaskPayload struct {
	TaskID   string `json:"task_id"`
	Prompt   string `json:"prompt"`
	ChatID   int64  `json:"chat_id"`
	FileID   string `json:"file_id,omitempty"`
}

func main() {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6380" // Fallback local
	}

	// Inisialisasi Bot
	pref := tele.Settings{
		Token:  botToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	// Inisialisasi Asynq Client
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisURL})
	defer client.Close()

	// Handle pesan apa pun (Prompt)
	b.Handle(tele.OnText, func(c tele.Context) error {
		msg := c.Message().Text
		chatID := c.Chat().ID

		taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())

		// Masukkan ke antrean Asynq
		payload, err := json.Marshal(AITaskPayload{
			TaskID: taskID,
			Prompt: msg,
			ChatID: chatID,
		})
		if err != nil {
			return c.Send("Terjadi kesalahan pada sistem antrean.")
		}

		task := asynq.NewTask("ai:generate", payload)
		info, err := client.Enqueue(task)
		if err != nil {
			return c.Send("Gagal mengirim antrean ke Worker AI.")
		}

		log.Printf("Task %s antre untuk ChatID %d. Queue: %s", taskID, chatID, info.Queue)
		
		// Balas ke user
		return c.Send("⏳ Sedang memproses teks Anda...")
	})

	// Handle pesan gambar (Photo)
	b.Handle(tele.OnPhoto, func(c tele.Context) error {
		photo := c.Message().Photo
		caption := c.Message().Caption
		if caption == "" {
			caption = "Tolong jelaskan gambar ini."
		}
		chatID := c.Chat().ID
		fileID := photo.FileID

		taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())

		// Masukkan ke antrean Asynq
		payload, err := json.Marshal(AITaskPayload{
			TaskID: taskID,
			Prompt: caption,
			ChatID: chatID,
			FileID: fileID,
		})
		if err != nil {
			return c.Send("Terjadi kesalahan pada sistem antrean.")
		}

		task := asynq.NewTask("ai:generate", payload)
		info, err := client.Enqueue(task)
		if err != nil {
			return c.Send("Gagal mengirim antrean ke Worker AI.")
		}

		log.Printf("Task %s (Gambar) antre untuk ChatID %d. Queue: %s", taskID, chatID, info.Queue)
		
		// Balas ke user
		return c.Send("🖼 Sedang memproses dan menganalisis gambar Anda...")
	})

	log.Println("API Gateway (Telegram Bot) berjalan...")
	b.Start()
}
