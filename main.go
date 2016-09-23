package main

import (
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

func init() {
	var err error
	db, err = sqlx.Connect("sqlite3", "database.sql")
	if err != nil {
		log.Fatalf("%+v", err)
	}

	if err := createLadderTable(); err != nil {
		log.Fatalf("%+v", err)
	}

	rand.Seed(time.Now().UnixNano())
}

func createLadderTable() error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS ladder (
		id INTEGER NOT NULL PRIMARY KEY,
		group_id TEXT,
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

type Ladder struct {
	ID      int64  `db:"id"`
	GroupID string `db:"group_id"`
	UserID  string `db:"user_id"`
	Rank    int64  `db:"rank"`
}

func getUser(userID, groupID string) (*Ladder, error) {
	l := []Ladder{}
	err := db.Select(&l, `SELECT id, group_id, user_id, rank FROM ladder WHERE user_id=? AND group_id=?`, userID, groupID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to select from ladder")
	}

	if len(l) == 0 {
		return nil, errNotFound{}
	}

	return &l[0], nil
}

func getUserAbove(groupID string, rank int64) (*Ladder, error) {
	l := []Ladder{}
	err := db.Select(&l, `SELECT id, group_id, user_id, rank FROM ladder WHERE group_id=? AND rank<=? ORDER BY rank DESC`, groupID, rank-1)
	if err != nil {
		return nil, errors.Wrap(err, "unable to select from ladder")
	}

	if len(l) == 0 {
		return nil, errNotFound{}
	}

	return &l[0], nil
}

func getLadder(groupID string) ([]Ladder, error) {
	l := []Ladder{}
	err := db.Select(&l, `SELECT id, group_id, user_id, rank FROM ladder WHERE group_id=? ORDER BY rank`, groupID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get ladder")
	}

	return l, nil
}

func clearLadder(groupID string) error {
	_, err := db.Exec(`DELETE FROM ladder WHERE group_id=?`, groupID)
	return errors.Wrap(err, "unable to delete ladder group")
}

func insertOrUpdate(l Ladder) error {
	existing, err := getUser(l.UserID, l.GroupID)
	if err != nil {
		if _, ok := err.(errNotFound); ok {
			_, err = db.NamedExec(`INSERT OR REPLACE INTO ladder (group_id, user_id, rank) VALUES(:group_id, :user_id, :rank)`, &l)
		} else {
			return errors.Wrap(err, "unable to select from ladder")
		}
	} else {
		existing.Rank = l.Rank
		_, err = db.NamedExec(`INSERT OR REPLACE INTO ladder (id, group_id, user_id, rank) VALUES(:id, :group_id, :user_id, :rank)`, existing)
	}

	return errors.Wrap(err, "unable to insert/update into ladder")
}

func rank(rtm *slack.RTM, user, group string) error {
	u, err := getUser(user, group)
	if err != nil {
		return err
	}

	g, err := rtm.GetGroupInfo(group)
	if err != nil {
		return err
	}

	us, err := rtm.GetUserInfo(user)
	if err != nil {
		return err
	}

	message := fmt.Sprintf("%s\t%d/%d", us.RealName, u.Rank+1, len(g.Members)-1)
	return sendMessage(rtm, group, message)
}

func challenge(rtm *slack.RTM, user, group string) error {
	challenger, err := getUser(user, group)
	if err != nil {
		return err
	}

	challenged, err := getUserAbove(group, challenger.Rank)
	if err != nil {
		return err
	}

	u, err := rtm.GetUserInfo(user)
	if err != nil {
		return errors.Wrap(err, "unable to get user info")
	}

	c, err := rtm.GetUserInfo(challenged.UserID)
	if err != nil {
		return errors.Wrap(err, "unable to get user info")
	}

	message := fmt.Sprintf("<@%s|%s> you've been challenged by %s (<@%s|%s>)\n", c.ID, c.Name, u.RealName, u.ID, u.Name)

	return sendMessage(rtm, group, message)
	msg := rtm.NewOutgoingMessage(message, group)
	rtm.SendMessage(msg)

	return nil
}

func won(rtm *slack.RTM, user, group string) error {
	winner, err := getUser(user, group)
	if err != nil {
		return err
	}

	// already the champion
	if winner.Rank == 0 {
		return nil
	}

	loser, err := getUserAbove(group, winner.Rank)
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
	return sendMessage(rtm, group, message)
}

func board(rtm *slack.RTM, group string) error {
	ladder, err := getLadder(group)
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

	return sendMessage(rtm, group, message)
	msg := rtm.NewOutgoingMessage(message, group)
	rtm.SendMessage(msg)

	return nil
}

func shuffle(rtm *slack.RTM, botID string, group *slack.Group) error {
	if err := clearLadder(group.ID); err != nil {
		return err
	}

	// length is one less because the bot is in the group
	ml := len(group.Members) - 1
	ranks := make(map[int64]Ladder)
	for _, member := range group.Members {
		if member == botID {
			continue
		}
		for {
			r := rand.Int63() % int64(ml)
			if _, ok := ranks[r]; !ok {
				ranks[r] = Ladder{
					GroupID: group.ID,
					UserID:  member,
					Rank:    r,
				}
				if err := insertOrUpdate(ranks[r]); err != nil {
					return err
				}
				break
			}
		}
	}

	return board(rtm, group.ID)
}

func sendMessage(rtm *slack.RTM, channel, text string) error {
	botID, err := getBotId(rtm)
	if err != nil {
		return err
	}

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

func runCommand(cmd command, botID string, rtm *slack.RTM, evt *slack.MessageEvent) error {
	group, err := rtm.GetGroupInfo(evt.Channel)
	if err != nil {
		return errors.Wrapf(err, "unable to get group info %q", evt.Channel)
	}

	switch cmd {
	case rankCommand:
		return rank(rtm, evt.Msg.User, group.ID)
	case challengeCommand:
		return challenge(rtm, evt.Msg.User, group.ID)
	case wonCommand:
		return won(rtm, evt.Msg.User, group.ID)
	case boardCommand:
		return board(rtm, group.ID)
	case shuffleCommand:
		return shuffle(rtm, botID, group)
	case helpCommand:
		if err := sendMessage(rtm, evt.Channel, cmds.Print()); err == nil {
			log.Printf("%+v", err)
		}
	}

	return nil
}

func getBotId(rtm *slack.RTM) (string, error) {
	auth, err := rtm.AuthTest()
	if err != nil {
		return "", errors.Wrap(err, "unable to get authenticated user")
	}

	return auth.UserID, nil
}

func main() {
	api := slack.New(accessToken)

	rtm := api.NewRTM()
	go rtm.ManageConnection()

	botID, err := getBotId(rtm)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("started ", os.Args[0])

	for {
		select {
		case e := <-rtm.IncomingEvents:
			switch evt := e.Data.(type) {
			case *slack.MessageEvent:
				if evt.Msg.User != botID {
					cmd := checkMessage(evt.Msg)
					if err := runCommand(cmd, botID, rtm, evt); err != nil {
						log.Printf("%+v", err)
					}
				}
			default:
				log.Printf("%#v", evt)
			}
		}
	}
}
