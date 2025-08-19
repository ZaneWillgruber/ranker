package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	bot "github.com/zanewillgruber/ranker/bot"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	bot.BotToken = os.Getenv("BOT_TOKEN")
	if bot.BotToken == "" {
		log.Fatal("BOT_TOKEN envirmonment variable is not set.")
	}
	bot.AppId = os.Getenv("APP_ID")
	if bot.AppId == "" {
		log.Fatal("APP_ID envirmonment variable is not set.")
	}
	bot.GuildId = os.Getenv("GUILD_ID")
	if bot.GuildId == "" {
		log.Fatal("GUILD_ID envirmonment variable is not set.")
	}

	bot.Run()
}
