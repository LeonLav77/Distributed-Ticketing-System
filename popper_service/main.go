package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var (
	ctx = context.Background()
	rdb *redis.Client
)

func main() {
	godotenv.Load()

	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	redisAddr := redisHost + ":" + redisPort

	rdb = redis.NewClient(&redis.Options{
		Addr:            redisAddr,
		Password:        "", // no password set
		DB:              0,  // use default DB
		DisableIdentity: true,
	})

	tickerTimeInMilliseconds, err := strconv.Atoi(os.Getenv("TICKER_TIME_MS"))
	if err != nil {
		log.Fatalf("Invalid TICKER_TIME_MS: %v", err)
	}

	ticker := time.NewTicker(time.Duration(tickerTimeInMilliseconds) * time.Millisecond)
	defer ticker.Stop()

	queueKey := os.Getenv("QUEUE_WATCHER_KEY")
	amountToPop, err := strconv.Atoi(os.Getenv("AMOUNT_TO_POP"))
	if err != nil {
		log.Fatalf("Invalid AMOUNT_TO_POP: %v", err)
	}

	for range ticker.C {
		result, _ := rdb.ZPopMin(ctx, queueKey, int64(amountToPop)).Result()

		if len(result) > 0 {
			log.Printf("Popped from queue: %s", result[0].Member)
		} else {
			log.Printf("No members to pop from queue.")
		}
	}
}
