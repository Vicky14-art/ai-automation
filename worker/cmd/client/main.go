package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
)

type AITaskPayload struct {
	TaskID string `json:"task_id"`
	Prompt string `json:"prompt"`
}

func main() {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: "localhost:6380"})
	defer client.Close()

	payload, err := json.Marshal(AITaskPayload{
		TaskID: fmt.Sprintf("task-%d", time.Now().Unix()),
		Prompt: "Jelaskan dengan singkat apa itu Artificial Intelligence dalam 1 paragraf.",
	})
	if err != nil {
		log.Fatal(err)
	}

	task := asynq.NewTask("ai:generate", payload)
	info, err := client.Enqueue(task)
	if err != nil {
		log.Fatalf("Gagal mengirim task ke queue: %v", err)
	}
	log.Printf("Sukses mengirim task! Task ID: %s, Queue: %s", info.ID, info.Queue)
}
