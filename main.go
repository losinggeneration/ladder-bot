package main

import (
	"flag"
	"log"
	"os"

	"github.com/nlopes/slack"
)

var botID string

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

type user struct {
	ID        string `db:"id"`
	ChannelID string `db:"channel_id"`
	Rating    Rank   `db:"rating"`
}

func openDatabase(filename string) (DB, error) {
	return NewBoltDB(filename)
}

func main() {
	debug := flag.Bool("debug", false, "enable debugging")
	filename := flag.String("filename", "ladder.db", "filename for file based databases")
	token := flag.String("token", "", "slack access token")
	flag.Parse()

	setDebug(*debug)

	db, err := openDatabase(*filename)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	if *token != "" {
		accessToken = *token
	}

	api := slack.New(accessToken)

	rtm := api.NewRTM()
	go rtm.ManageConnection()

	auth, err := api.AuthTest()
	if err != nil {
		log.Fatal(err)
	}

	botID = auth.UserID

	log.Println("started ", os.Args[0])

	for {
		select {
		case e := <-rtm.IncomingEvents:
			switch evt := e.Data.(type) {
			case *slack.MessageEvent:
				Debugf("%#v", evt)
				if evt.BotID == "" {
					cmd := checkMessage(evt.Msg)
					if err := runCommand(db, cmd, rtm, evt); err != nil {
						log.Printf("%+v", err)
					}
				}
			default:
				Debugf("%#v", evt)
			}
		}
	}
}
