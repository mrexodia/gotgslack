package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jinzhu/configor"
	"github.com/nlopes/slack"
)

type Config struct {
	Slack struct {
		Token   string `required:"true"`
		Channel string `required:"true"`
	}
	Telegram struct {
		Token   string `required:"true"`
		Admins  string `required:"true"`
		GroupId string `default:"0"`
	}
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func goTelegramSlack(conf Config) {
	//Slack init
	api := slack.New(conf.Slack.Token)
	logger := log.New(os.Stdout, "slack-bot: ", log.Lshortfile|log.LstdFlags)
	slack.SetLogger(logger)
	//api.SetDebug(true)

	rtm := api.NewRTM()
	go rtm.ManageConnection()

	//Telegram init
	bot, err := tgbotapi.NewBotAPI(conf.Telegram.Token)
	if err != nil {
		fmt.Printf("[Telegram] Error in NewBotAPI: %v...\n", err)
		return
	}
	fmt.Printf("[Telegram] Authorized on account %s\n", bot.Self.UserName)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		fmt.Printf("[Telegram] Error in GetUpdatesChan: %v...\n", err)
		return
	}
	groupId, err := strconv.ParseInt(conf.Telegram.GroupId, 10, 64)
	if err != nil {
		fmt.Printf("[Telegram] Error parsing GroupId: %v...\n", err)
		groupId = 0
	}
	fmt.Printf("[Telegram] GroupId: %v\n", groupId)

	//Slack loop
	go func() {
	Loop:
		for {
			select {
			case msg := <-rtm.IncomingEvents:
				switch ev := msg.Data.(type) {
				case *slack.HelloEvent:

				case *slack.ConnectedEvent:

				case *slack.MessageEvent:
					if len(ev.Msg.BotID) == 0 {
						user, err := api.GetUserInfo(ev.Msg.User)
						if err == nil {
							slackMsg := fmt.Sprintf("<%v> %v", user.Name, ev.Msg.Text)
							fmt.Printf("[Slack] %v\n", slackMsg)
							if groupId != 0 {
								bot.Send(tgbotapi.NewMessage(groupId, slackMsg))
							}
						}
					}

				case *slack.PresenceChangeEvent:

				case *slack.LatencyReport:

				case *slack.RTMError:
					fmt.Printf("[Slack] Error: %s\n", ev.Error())

				case *slack.InvalidAuthEvent:
					fmt.Printf("[Slack] Invalid credentials\n")
					break Loop

				default:
				}
			}
		}
	}()

	//Telegram loop
	for update := range updates {
		//copy variables
		message := update.Message
		if message == nil {
			fmt.Printf("[Telegram] message == nil\n%v\n", update)
			continue
		}
		chat := message.Chat
		if chat == nil {
			fmt.Printf("[Telegram] chat == nil\n%v\n", update)
			continue
		}
		name := message.From.UserName
		if len(name) == 0 {
			name = message.From.FirstName
		}
		//construct/log message
		telegramMsg := fmt.Sprintf("<%s> %s", name, message.Text)
		fmt.Printf("[Telegram] %s\n", telegramMsg)
		//check for admin commands
		if stringInSlice(message.From.UserName, strings.Split(conf.Telegram.Admins, " ")) && strings.HasPrefix(message.Text, "/") {
			if message.Text == "/start" && (chat.IsGroup() || chat.IsSuperGroup()) {
				groupId = chat.ID
			} else if message.Text == "/status" {
				bot.Send(tgbotapi.NewMessage(int64(message.From.ID), fmt.Sprintf("groupId: %v", groupId)))
			}
		} else if len(telegramMsg) > 0 {
			if groupId != 0 {
				//forward message to group
				if groupId != chat.ID {
					bot.Send(tgbotapi.NewMessage(groupId, telegramMsg))
				}
				//send to Slack
				params := slack.PostMessageParameters{}
				params.AsUser = true
				_, _, err := api.PostMessage(conf.Slack.Channel, telegramMsg, params)
				if err != nil {
					fmt.Printf("[Slack] error: %v\n", err)
				}
			} else {
				fmt.Println("[Telegam] Use /start to start the bot...")
			}
		}
	}
}

func main() {
	fmt.Println("Telegram/Slack Sync Bot, written in Go by mrexodia")
	var conf Config
	if err := configor.Load(&conf, "config.json"); err != nil {
		fmt.Printf("Error loading config: %v...\n", err)
		return
	}
	goTelegramSlack(conf)
}
