package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nlopes/slack"
	"github.com/pkg/errors"
)

type command string

const (
	helpCommand      command = "help"
	rankCommand              = "rank"
	wonCommand               = "won"
	challengeCommand         = "challenge"
	boardCommand             = "board"
	shuffleCommand           = "shuffle"
	unknownCommand           = "unknown"
)

type commands map[command]string

var cmds = commands{
	helpCommand:      "a list of available commands",
	rankCommand:      "what your current rank is",
	wonCommand:       "move yourself up the ladder",
	challengeCommand: "challenge another player to a match",
	boardCommand:     "show the board rankings",
	shuffleCommand:   "shuffle the board to random rankings",
}

func (c commands) Print() string {
	cmds := "Available commands\n"
	for k, v := range c {
		cmds += fmt.Sprintf("%q - %v\n", k, v)
	}

	return cmds
}

var db *sqlx.DB
var botID string

func setup() error {
	var err error
	db, err = sqlx.Connect("sqlite3", "database.sql")
	if err != nil {
		return err
	}

	if err := createLadderTable(); err != nil {
		return err
	}

	rand.Seed(time.Now().UnixNano())

	return nil
}

func createLadderTable() error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS ladder (
		id INTEGER NOT NULL PRIMARY KEY,
		channel_id TEXT,
		user_id TEXT,
		rank INTEGER
	)`)

	return errors.Wrap(err, "unable to create table ladder")
}

func checkMessage(msg slack.Msg) command {
	for k := range cmds {
		if strings.Contains(msg.Text, string(k)) {
			return k
		}
	}

	return unknownCommand
}

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

type ladder struct {
	ID        int64  `db:"id"`
	ChannelID string `db:"channel_id"`
	UserID    string `db:"user_id"`
	Rank      int64  `db:"rank"`
}

func getUser(userID, channelID string) (*ladder, error) {
	l := []ladder{}
	err := db.Select(&l, `SELECT id, channel_id, user_id, rank FROM ladder WHERE user_id=? AND channel_id=?`, userID, channelID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to select from ladder")
	}

	if len(l) == 0 {
		return nil, errNotFound{}
	}

	return &l[0], nil
}

func getUserAbove(channelID string, rank int64) (*ladder, error) {
	l := []ladder{}
	err := db.Select(&l, `SELECT id, channel_id, user_id, rank FROM ladder WHERE channel_id=? AND rank<=? ORDER BY rank DESC`, channelID, rank-1)
	if err != nil {
		return nil, errors.Wrap(err, "unable to select from ladder")
	}

	if len(l) == 0 {
		return nil, errNotFound{}
	}

	return &l[0], nil
}

func getLadder(channelID string) ([]ladder, error) {
	l := []ladder{}
	err := db.Select(&l, `SELECT id, channel_id, user_id, rank FROM ladder WHERE channel_id=? ORDER BY rank`, channelID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get ladder")
	}

	return l, nil
}

func clearLadder(channelID string) error {
	_, err := db.Exec(`DELETE FROM ladder WHERE channel_id=?`, channelID)
	return errors.Wrap(err, "unable to delete ladder group")
}

func insertOrUpdate(l ladder) error {
	existing, err := getUser(l.UserID, l.ChannelID)
	if err != nil {
		if _, ok := err.(errNotFound); ok {
			_, err = db.NamedExec(`INSERT OR REPLACE INTO ladder (channel_id, user_id, rank) VALUES(:channel_id, :user_id, :rank)`, &l)
		} else {
			return errors.Wrap(err, "unable to select from ladder")
		}
	} else {
		existing.Rank = l.Rank
		_, err = db.NamedExec(`INSERT OR REPLACE INTO ladder (id, channel_id, user_id, rank) VALUES(:id, :channel_id, :user_id, :rank)`, existing)
	}

	return errors.Wrap(err, "unable to insert/update into ladder")
}

func rank(rtm *slack.RTM, msg slack.Msg) error {
	u, err := getUser(msg.User, msg.Channel)
	if err != nil {
		return err
	}

	us, err := rtm.GetUserInfo(msg.User)
	if err != nil {
		return err
	}

	members, err := getMembers(rtm, msg.Channel)
	if err != nil {
		return err
	}

	message := fmt.Sprintf("%s\t%d/%d", us.RealName, u.Rank+1, len(members)-1)
	return sendMessage(rtm, msg.Channel, message)
}

func challenge(rtm *slack.RTM, msg slack.Msg) error {
	challenger, err := getUser(msg.User, msg.Channel)
	if err != nil {
		return err
	}

	challenged, err := getUserAbove(msg.Channel, challenger.Rank)
	if err != nil {
		return err
	}

	u, err := rtm.GetUserInfo(msg.User)
	if err != nil {
		return errors.Wrap(err, "unable to get user info")
	}

	c, err := rtm.GetUserInfo(challenged.UserID)
	if err != nil {
		return errors.Wrap(err, "unable to get user info")
	}

	message := fmt.Sprintf("<@%s|%s> you've been challenged by %s (<@%s|%s>)\n", c.ID, c.Name, u.RealName, u.ID, u.Name)

	return sendMessage(rtm, msg.Channel, message)
}

func won(rtm *slack.RTM, msg slack.Msg) error {
	winner, err := getUser(msg.User, msg.Channel)
	if err != nil {
		return err
	}

	// already the champion
	if winner.Rank == 0 {
		return nil
	}

	loser, err := getUserAbove(msg.Channel, winner.Rank)
	if err != nil {
		return err
	}

	winner.Rank, loser.Rank = loser.Rank, winner.Rank

	if err := insertOrUpdate(*winner); err != nil {
		return err
	}

	if err := insertOrUpdate(*loser); err != nil {
		return err
	}

	message := fmt.Sprintf("New Rank %d", winner.Rank+1)
	return sendMessage(rtm, msg.Channel, message)
}

func board(rtm *slack.RTM, msg slack.Msg) error {
	ladder, err := getLadder(msg.Channel)
	if err != nil {
		return err
	}

	var message string
	for _, u := range ladder {
		user, err := rtm.GetUserInfo(u.UserID)
		if err != nil {
			return errors.Wrap(err, "unable to get user info")
		}

		name := user.RealName
		if name == "" {
			name = user.Name
		}
		message += fmt.Sprintf("%s\t%v\n", name, u.Rank+1)
	}

	return sendMessage(rtm, msg.Channel, message)
}

func shuffle(rtm *slack.RTM, msg slack.Msg) error {
	if err := clearLadder(msg.Channel); err != nil {
		return err
	}

	members, err := getMembers(rtm, msg.Channel)
	if err != nil {
		return err
	}
	// length is one less because the bot is in the group
	ml := len(members) - 1
	ranks := make(map[int64]ladder)
	for _, member := range members {
		if member == botID {
			continue
		}
		for {
			r := rand.Int63() % int64(ml)
			if _, ok := ranks[r]; !ok {
				ranks[r] = ladder{
					ChannelID: msg.Channel,
					UserID:    member,
					Rank:      r,
				}
				if err := insertOrUpdate(ranks[r]); err != nil {
					return err
				}
				break
			}
		}
	}

	return board(rtm, msg)
}

func getMembers(rtm *slack.RTM, channel string) ([]string, error) {
	ch, cherr := rtm.GetChannelInfo(channel)
	if cherr == nil {
		return ch.Members, nil
	}

	gr, grerr := rtm.GetGroupInfo(channel)
	if grerr == nil {
		return gr.Members, nil
	}

	return nil, errors.Wrapf(grerr, "unable to get group or channel: %v", cherr)
}

func sendMessage(rtm *slack.RTM, channel, text string) error {
	var err error
	for i := 0; i < 5; i++ {
		params := slack.NewPostMessageParameters()
		params.EscapeText = false
		params.AsUser = true
		params.Username = botID

		_, _, err = rtm.PostMessage(channel, text, params)
		if err == nil {
			break
		}
	}

	return errors.Wrap(err, "unable to send message")
}

func runCommand(cmd command, rtm *slack.RTM, evt *slack.MessageEvent) error {
	switch cmd {
	case rankCommand:
		return rank(rtm, evt.Msg)
	case challengeCommand:
		return challenge(rtm, evt.Msg)
	case wonCommand:
		return won(rtm, evt.Msg)
	case boardCommand:
		return board(rtm, evt.Msg)
	case shuffleCommand:
		return shuffle(rtm, evt.Msg)
	case helpCommand:
		return sendMessage(rtm, evt.Channel, cmds.Print())
	}

	return nil
}

func main() {
	debug := flag.Bool("debug", false, "enable debugging")
	flag.Parse()

	if err := setup(); err != nil {
		log.Fatalf("%+v", err)
	}

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

	for {
		select {
		case e := <-rtm.IncomingEvents:
			switch evt := e.Data.(type) {
			case *slack.MessageEvent:
				Debugf("%#v", evt)
				if evt.BotID == "" {
					cmd := checkMessage(evt.Msg)
					if err := runCommand(cmd, rtm, evt); err != nil {
						log.Printf("%+v", err)
					}
				}
			default:
				Debugf("%#v", evt)
			}
		}
	}
}
