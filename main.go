package main

import (
	"errors"
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

type ladders []ladder

func (l ladders) Less(i, j int) bool { return l[i].Rank < l[j].Rank }
func (l ladders) Len() int           { return len(l) }
func (l ladders) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

func init() {
	rand.Seed(time.Now().UnixNano())
}

func transferData(input, output DB) error {
	Debug("transfering data")
	ladders, err := input.getLadders()
	if err != nil {
		return err
	}

	Debugf("Got ladders: %#v", ladders)
	for _, ladder := range ladders {
		ranks, err := input.getLadder(ladder)
		if err != nil {
			return err
		}

		Debugf("Got ranks: %#v", ranks)
		if err := output.updateLadder(ranks); err != nil {
			return err
		}
	}

	return nil
}

func openDatabase(database, filename string) (DB, error) {
	switch database {
	case "sqlite":
		return NewSqlite(filename)
	case "boltdb":
		return NewBoltDB(filename)
	}

	return nil, errors.New("invalid database argument")
}

func main() {
	debug := flag.Bool("debug", false, "enable debugging")
	database := flag.String("database", "sqlite", "[sqlite, boltdb]")
	filename := flag.String("filename", "database.sql", "filename for file based databases")
	transfer := flag.String("transfer", "", "[sqlite, boltdb] database to transfer to")
	output := flag.String("output", "database.db", "filename for transfer to")
	flag.Parse()

	setDebug(*debug)

	db, err := openDatabase(*database, *filename)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	if *transfer != "" {
		out, err := openDatabase(*transfer, *output)
		if err != nil {
			log.Fatalf("%+v", err)
		}
		defer func() {
			if err := out.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		if err := transferData(db, out); err != nil {
			log.Fatalf("%+v", err)
		}

		return
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
