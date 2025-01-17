package main

import (
	"crypto/tls"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"layeh.com/gumble/gumble"
	"layeh.com/gumble/gumbleutil"
)

type Bot struct {
	connectString        string
	defaultConnectString string
	channelTree          []string
}

func client(listeners ...gumble.EventListener) {
	server := os.Getenv("MUMBLE_SERVER")
	if server == "" {
		server = "localhost:64738"
	}

	username := os.Getenv("MUMBLE_USERNAME")
	if username == "" {
		username = "gumble-bot"
	}

	password := os.Getenv("MUMBLE_PASSWORD")

	insecure := os.Getenv("MUMBLE_INSECURE")
	insecureBool, _ := strconv.ParseBool(insecure)

	certificateFile := os.Getenv("MUMBLE_CERT_FILE")
	keyFile := os.Getenv("MUMBLE_KEY_FILE")

	host, port, err := net.SplitHostPort(server)
	if err != nil {
		host = server
		port = strconv.Itoa(gumble.DefaultPort)
	}

	keepAlive := make(chan bool)

	config := gumble.NewConfig()
	config.Username = username
	config.Password = password

	address := net.JoinHostPort(host, port)

	var tlsConfig tls.Config

	if insecureBool {
		tlsConfig.InsecureSkipVerify = true
	}

	if certificateFile != "" {
		if keyFile == "" {
			keyFile = certificateFile
		}
		if certificate, err := tls.LoadX509KeyPair(certificateFile, keyFile); err != nil {
			log.Printf("%s: %s\n", os.Args[0], err)
			os.Exit(1)
		} else {
			tlsConfig.Certificates = append(tlsConfig.Certificates, certificate)
		}
	}

	config.Attach(gumbleutil.AutoBitrate)
	for _, listener := range listeners {
		config.Attach(listener)
	}

	config.Attach(gumbleutil.Listener{
		Disconnect: func(e *gumble.DisconnectEvent) {
			keepAlive <- true
		},
	})

	_, err = gumble.DialWithDialer(new(net.Dialer), address, config, &tlsConfig)
	if err != nil {
		log.Printf("%s: %s\n", os.Args[0], err)
		os.Exit(1)
	}

	<-keepAlive
}

func runBot(bot Bot) {
	client(gumbleutil.Listener{
		Connect: func(e *gumble.ConnectEvent) {
			if len(bot.channelTree) > 0 {
				e.Client.Self.Move(e.Client.Channels.Find(bot.channelTree...))
				log.Println("Connected.")
			}
			bot.connectString = bot.defaultConnectString
			time.Sleep(time.Second * 1)
		},
		TextMessage: func(e *gumble.TextMessageEvent) {
			if strings.Contains(e.TextMessage.Message, "connect") {
				bot.connectString = e.TextMessage.Message
				e.Sender.Send("Connect received: " + bot.connectString)
				log.Printf("Connect: %v received from %v", bot.connectString, e.Sender.Name)
			}
		},
		UserChange: func(e *gumble.UserChangeEvent) {
			if e.Type.Has(gumble.UserChangeConnected) {
				if e.User.Name != "_ConnectBot" {
					if e.User.Channel.Name == e.Client.Self.Channel.Name {
						log.Printf("%v connected.\n", e.User.Name)
					}
				}

			}
			if e.Type.Has(gumble.UserChangeChannel) {
				log.Printf("%v changed channel to %v.\n", e.User.Name, e.User.Channel.Name)
				if len(e.Client.Self.Channel.Users) == 1 {
					bot.connectString = bot.defaultConnectString
				}
				if e.User.Name != "_ConnectBot" {
					if e.User.Channel.Name == e.Client.Self.Channel.Name {
						log.Println(bot.connectString)
						if len(bot.connectString) > 0 {
							e.User.Send(bot.connectString)
							log.Printf("Sent connect to %s", e.User.Name)
						}
					}
				}
			}
			if e.Type.Has(gumble.UserChangeDisconnected) {
				log.Printf("%v disconnected.\n", e.User.Name)
				log.Printf("Users: %v", len(e.Client.Self.Channel.Users))
				if len(e.Client.Self.Channel.Users) == 1 {
					bot.connectString = bot.defaultConnectString
				}
			}
		},
	})

}

func main() {
	channels := os.Getenv("MUMBLE_CHANNELS")
	splitChannels := strings.Split(channels, ",")

	bot := Bot{
		"",
		os.Getenv("MUMBLE_DEFAULT_STRING"),
		splitChannels,
	}
	runBot(bot)
}
