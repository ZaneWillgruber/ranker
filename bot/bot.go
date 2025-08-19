package bot

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"

	"github.com/bwmarrin/discordgo"
)

var (
	BotToken      string
	AppId         string
	GuildId       string
	emojiToRating = map[string]int{"1️⃣": 1, "2️⃣": 2, "3️⃣": 3, "4️⃣": 4, "5️⃣": 5}
	ratingEmojis  = []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣"}
)

type RatingInfo struct {
	ItemName string
	Votes    map[string]int
}

var ratings = struct {
	sync.RWMutex
	m map[string]*RatingInfo
}{m: make(map[string]*RatingInfo)}

func checkNilError(e error) {
	if e != nil {
		log.Fatal(e.Error())
	}
}

func Run() {
	fmt.Println("Bot Running...")

	discord, err := discordgo.New("Bot " + BotToken)
	checkNilError(err)

	_, err = discord.ApplicationCommandBulkOverwrite(AppId, GuildId,
		[]*discordgo.ApplicationCommand{
			{
				Name:        "ping",
				Description: "get a response",
			},
			{
				Name:        "rate",
				Description: "have the public rate something",
			},
		})
	checkNilError(err)

	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		data := i.ApplicationCommandData()
		switch data.Name {
		case "ping":
			err := s.InteractionRespond(
				i.Interaction,
				&discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "pong!",
					},
				},
			)
			checkNilError(err)
		}
	})

	err = discord.Open()
	checkNilError(err)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop

	err = discord.Close()
	checkNilError(err)
}
