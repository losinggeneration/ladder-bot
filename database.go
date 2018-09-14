package main

type DB interface {
	Close() error
	getUser(userID, channelID string) (*user, error)
}
