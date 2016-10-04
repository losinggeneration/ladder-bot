package main

type DB interface {
	Close() error
	createLadderTable() error
	getUser(userID, channelID string) (*ladder, error)
	getUserAbove(channelID string, rank int64) (*ladder, error)
	getLastUser(channelID string) (*ladder, error)
	getLadders() ([]string, error)
	getLadder(channelID string) ([]ladder, error)
	clearLadder(channelID string) error
	removeUser(l ladder) error
	insertOrUpdate(l ladder) error
	updateLadder(l []ladder) error
}
