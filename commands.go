package main

import (
	"fmt"
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
		if strings.Contains(strings.ToLower(msg.Text), string(k)) {
			return k
		}
	}

	return unknownCommand
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

func runCommand(db DB, cmd command, rtm *slack.RTM, evt *slack.MessageEvent) error {
	switch cmd {
	case helpCommand:
		return sendMessage(rtm, evt.Channel, cmds.Print())
	}

	return nil
}
