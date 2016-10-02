package main

import (
	"flag"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/nlopes/slack"
)

var botID string

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

type ladder struct {
	ID        int64  `db:"id"`
	ChannelID string `db:"channel_id"`
	UserID    string `db:"user_id"`
	Rank      int64  `db:"rank"`
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	debug := flag.Bool("debug", false, "enable debugging")
	database := flag.String("database", "sqlite", "[memory, boltdb]")
	flag.Parse()

	var (
		db  DB
		err error
	)

	switch *database {
	case "sqlite":
		db, err = NewSqlite()
	case "boltdb":
		db, err = NewBoltDB()
	default:
		log.Fatal("invalid database argument")
	}

	if err != nil {
		log.Fatalf("%+v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	api := slack.New(accessToken)

	rtm := api.NewRTM()
	go rtm.ManageConnection()

	auth, err := api.AuthTest()
	if err != nil {
		log.Fatal(err)
	}

	botID = auth.UserID

	setDebug(*debug)
	log.Println("started ", os.Args[0])
	log.Println("using", *database, "for a database")

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
