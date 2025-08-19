package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

var BotToken string

var ratings = struct {
	sync.RWMutex
	m map[string]*RatingInfo
}{m: make(map[string]*RatingInfo)}

type RatingInfo struct {
	ItemName string
	Votes    map[string]int
}

var emojiToRating = map[string]int{
	"1️⃣": 1, "2️⃣": 2, "3️⃣": 3, "4️⃣": 4, "5️⃣": 5,
}

var ratingEmojis = []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣"}

var command = &discordgo.ApplicationCommand{
	Name:        "rate",
	Description: "Start a rating poll for an item.",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "item",
			Description: "The item you want users to rate.",
			Required:    true,
		},
	},
}

var commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"rate": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		var itemName string
		for _, opt := range options {
			if opt.Name == "item" {
				itemName = opt.StringValue()
			}
		}

		if itemName == "" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error: Item name was not provided.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
		if err != nil {
			log.Printf("Failed to send deferred response: %v", err)
			return
		}

		content := fmt.Sprintf("📊 **React to rate: %s**\n\nNo ratings yet.", itemName)
		ratingMsg, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		if err != nil {
			log.Printf("Failed to send message: %v", err)
			return
		}

		for _, emoji := range ratingEmojis {
			err := s.MessageReactionAdd(ratingMsg.ChannelID, ratingMsg.ID, emoji)
			if err != nil {
				log.Printf("Failed to add reaction: %v", err)
			}
		}

		ratings.Lock()
		ratings.m[ratingMsg.ID] = &RatingInfo{
			ItemName: itemName,
			Votes:    make(map[string]int),
		}
		ratings.Unlock()

		saveNewRating(ratingMsg.ID, itemName)
	},
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	BotToken = os.Getenv("BOT_TOKEN")
	if BotToken == "" {
		log.Fatal("BOT_TOKEN environment variable not set. Please set it to your Discord bot token.")
	}

	initDB()
	defer db.Close()

	loadRatings()

	dg, err := discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
	dg.AddHandler(reactionAdd)
	dg.AddHandler(reactionRemove)

	dg.Identify.Intents = discordgo.IntentsGuildMessageReactions

	err = dg.Open()
	if err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}

	log.Println("Adding commands...")
	registeredCommand, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", command)
	if err != nil {
		log.Fatalf("Cannot create '%v' command: %v", command.Name, err)
	}
	log.Printf("'%s' command created.", registeredCommand.Name)

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Removing commands and shutting down...")
	err = dg.ApplicationCommandDelete(dg.State.User.ID, "", registeredCommand.ID)
	if err != nil {
		log.Printf("Cannot delete '%v' command: %v", registeredCommand.Name, err)
	}
	dg.Close()
}

func reactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	// Ignore reactions added by the bot itself.
	if r.UserID == s.State.User.ID {
		return
	}

	ratings.Lock()
	defer ratings.Unlock()

	ratingInfo, ok := ratings.m[r.MessageID]
	if !ok {
		return
	}

	ratingValue, ok := emojiToRating[r.Emoji.Name]
	if !ok {
		return
	}

	for _, emoji := range ratingEmojis {
		if emoji != r.Emoji.Name {
			err := s.MessageReactionRemove(r.ChannelID, r.MessageID, emoji, r.UserID)
			if err != nil {
				log.Printf("Could not remove old reaction: %v", err)
			}
		}
	}

	ratingInfo.Votes[r.UserID] = ratingValue

	updateVotes(r.MessageID, ratingInfo.Votes)

	updateRatingMessage(s, r.ChannelID, r.MessageID)
}

func reactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	if r.UserID == s.State.User.ID {
		return
	}

	ratings.Lock()
	defer ratings.Unlock()

	ratingInfo, ok := ratings.m[r.MessageID]
	if !ok {
		return
	}

	if _, ok := emojiToRating[r.Emoji.Name]; !ok {
		return
	}

	delete(ratingInfo.Votes, r.UserID)

	updateVotes(r.MessageID, ratingInfo.Votes)

	updateRatingMessage(s, r.ChannelID, r.MessageID)
}

func updateRatingMessage(s *discordgo.Session, channelID, messageID string) {

	ratingInfo, ok := ratings.m[messageID]
	if !ok {
		return
	}

	var totalRating float64
	for _, rating := range ratingInfo.Votes {
		totalRating += float64(rating)
	}
	voteCount := len(ratingInfo.Votes)
	averageRating := 0.0
	if voteCount > 0 {
		averageRating = totalRating / float64(voteCount)
	}

	starString := ""
	for i := 1; i <= 5; i++ {
		if averageRating >= float64(i)-0.25 {
			starString += "⭐"
		} else if averageRating >= float64(i)-0.75 {
			starString += "🌟"
		} else {
			starString += "⚫"
		}
	}

	var updatedContent string
	if voteCount == 0 {
		updatedContent = fmt.Sprintf("📊 **React to rate: %s**\n\nNo ratings yet.", ratingInfo.ItemName)
	} else {
		updatedContent = fmt.Sprintf(
			"📊 **React to rate: %s**\n\n**Average Rating:** %.2f / 5.00 (%s)\n**Total Votes:** %d",
			ratingInfo.ItemName,
			averageRating,
			starString,
			voteCount,
		)
	}

	_, err := s.ChannelMessageEdit(channelID, messageID, updatedContent)
	if err != nil {
		log.Printf("Failed to edit message: %v", err)
	}
}
