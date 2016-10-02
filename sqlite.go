package main

import (
	"github.com/Billups/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
)

type sqlite struct {
	db *sqlx.DB
}

func NewSqlite(filename string) (DB, error) {
	db, err := sqlx.Connect("sqlite3", filename)
	if err != nil {
		return nil, err
	}

	s := sqlite{db: db}

	if err := s.createLadderTable(); err != nil {
		return nil, err
	}

	return &s, nil
}

func (s *sqlite) Close() error {
	return errors.Wrap(s.db.Close(), "unable to close database")
	return nil
}

func (s *sqlite) createLadderTable() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS ladder (
		id INTEGER NOT NULL PRIMARY KEY,
		channel_id TEXT,
		user_id TEXT,
		rank INTEGER
	)`)

	return errors.Wrap(err, "unable to create table ladder")
}

func (s *sqlite) getUser(userID, channelID string) (*ladder, error) {
	l := []ladder{}
	err := s.db.Select(&l, `SELECT id, channel_id, user_id, rank FROM ladder WHERE user_id=? AND channel_id=?`, userID, channelID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to select from ladder")
	}

	if len(l) == 0 {
		return nil, errors.Wrap(errNotFound{}, "unable to get user")
	}

	return &l[0], nil
}

func (s *sqlite) getUserAbove(channelID string, rank int64) (*ladder, error) {
	l := []ladder{}
	err := s.db.Select(&l, `SELECT id, channel_id, user_id, rank FROM ladder WHERE channel_id=? AND rank<=? ORDER BY rank DESC`, channelID, rank-1)
	if err != nil {
		return nil, errors.Wrap(err, "unable to select from ladder")
	}

	if len(l) == 0 {
		return nil, errors.Wrap(errNotFound{}, "unable to get user above")
	}

	return &l[0], nil
}

func (s *sqlite) getLastUser(channelID string) (*ladder, error) {
	l, err := s.getLadder(channelID)
	if err != nil {
		return nil, err
	}

	if len(l) == 0 {
		return nil, errors.Wrap(errNotFound{}, "unable to get user below")
	}

	return &l[len(l)-1], nil
}

func (s *sqlite) getLadder(channelID string) ([]ladder, error) {
	l := []ladder{}
	err := s.db.Select(&l, `SELECT id, channel_id, user_id, rank FROM ladder WHERE channel_id=? ORDER BY rank`, channelID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get ladder")
	}

	return l, nil
}

func (s *sqlite) clearLadder(channelID string) error {
	_, err := s.db.Exec(`DELETE FROM ladder WHERE channel_id=?`, channelID)
	return errors.Wrap(err, "unable to delete ladder group")
}

func (s *sqlite) removeUser(l ladder) error {
	_, err := s.db.Exec(`DELETE FROM ladder WHERE channel_id=? AND user_id=?`, l.ChannelID, l.UserID)
	return errors.Wrap(err, "unable to delete user from ladder")
}

func (s *sqlite) insertOrUpdate(l ladder) error {
	existing, err := s.getUser(l.UserID, l.ChannelID)
	if err != nil {
		if _, ok := errors.Cause(err).(errNotFound); ok {
			_, err = s.db.NamedExec(`INSERT OR REPLACE INTO ladder (channel_id, user_id, rank) VALUES(:channel_id, :user_id, :rank)`, &l)
		} else {
			return errors.Wrap(err, "unable to select from ladder")
		}
	} else {
		existing.Rank = l.Rank
		_, err = s.db.NamedExec(`INSERT OR REPLACE INTO ladder (id, channel_id, user_id, rank) VALUES(:id, :channel_id, :user_id, :rank)`, existing)
	}

	return errors.Wrap(err, "unable to insert/update into ladder")
}

func (s *sqlite) updateLadder(l []ladder) error {
	for _, u := range l {
		if err := s.insertOrUpdate(u); err != nil {
			return err
		}
	}

	return nil
}
