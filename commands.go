package main

import (
	"fmt"
	"math/rand"
	"strings"

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

func checkMessage(msg slack.Msg) command {
	for k := range cmds {
		if strings.Contains(msg.Text, string(k)) {
			return k
		}
	}

	return unknownCommand
}

func rank(db DB, rtm *slack.RTM, msg slack.Msg) error {
	u, err := db.getUser(msg.User, msg.Channel)
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

func challenge(db DB, rtm *slack.RTM, msg slack.Msg) error {
	challenger, err := db.getUser(msg.User, msg.Channel)
	if err != nil {
		return err
	}

	challenged, err := db.getUserAbove(msg.Channel, challenger.Rank)
	if err != nil {
		if _, ok := errors.Cause(err).(errNotFound); ok {
			return nil
		}
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

func won(db DB, rtm *slack.RTM, msg slack.Msg) error {
	winner, err := db.getUser(msg.User, msg.Channel)
	if err != nil {
		return err
	}

	// already the champion
	if winner.Rank == 0 {
		return nil
	}

	loser, err := db.getUserAbove(msg.Channel, winner.Rank)
	if err != nil {
		return err
	}

	winner.Rank, loser.Rank = loser.Rank, winner.Rank

	if err := db.insertOrUpdate(*winner); err != nil {
		return err
	}

	if err := db.insertOrUpdate(*loser); err != nil {
		return err
	}

	message := fmt.Sprintf("New Rank %d", winner.Rank+1)
	return sendMessage(rtm, msg.Channel, message)
}

func board(db DB, rtm *slack.RTM, msg slack.Msg) error {
	ladder, err := db.getLadder(msg.Channel)
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

func shuffle(db DB, rtm *slack.RTM, msg slack.Msg) error {
	if err := db.clearLadder(msg.Channel); err != nil {
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
				if err := db.insertOrUpdate(ranks[r]); err != nil {
					return err
				}
				break
			}
		}
	}

	return board(db, rtm, msg)
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

func userJoined(db DB, rtm *slack.RTM, msg slack.Msg) error {
	last, err := db.getLastUser(msg.Channel)
	if err != nil {
		return err
	}

	return db.insertOrUpdate(ladder{
		UserID:    msg.User,
		ChannelID: msg.Channel,
		Rank:      last.Rank + 1,
	})
}

func userLeft(db DB, rtm *slack.RTM, msg slack.Msg) error {
	users, err := db.getLadder(msg.Channel)
	if err != nil {
		return err
	}

	newUsers := make([]ladder, len(users)-1)

	rank := int64(0)
	for i := range users {
		if users[i].UserID == msg.User {
			if err := db.removeUser(users[i]); err != nil {
				return err
			}
			continue
		}

		newUsers[rank] = users[i]
		newUsers[rank].Rank = rank
		rank++
	}

	return db.updateLadder(newUsers)
}

func runCommand(db DB, cmd command, rtm *slack.RTM, evt *slack.MessageEvent) error {
	switch cmd {
	case rankCommand:
		return rank(db, rtm, evt.Msg)
	case challengeCommand:
		return challenge(db, rtm, evt.Msg)
	case wonCommand:
		return won(db, rtm, evt.Msg)
	case boardCommand:
		return board(db, rtm, evt.Msg)
	case shuffleCommand:
		return shuffle(db, rtm, evt.Msg)
	case helpCommand:
		return sendMessage(rtm, evt.Channel, cmds.Print())
	}

	switch evt.Msg.SubType {
	case "channel_join":
		return userJoined(db, rtm, evt.Msg)
	case "channel_leave":
		return userLeft(db, rtm, evt.Msg)
	}

	return nil
}
