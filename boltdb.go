package main

import (
	"encoding/json"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
)

type boltdb struct {
	db *bolt.DB
}

type boltLadder map[string]ladder

func NewBoltDB() (DB, error) {
	db, err := bolt.Open("database.db", 0600, nil)
	if err != nil {
		return nil, errors.Wrap(err, "unable to open database.db")
	}

	return &boltdb{db: db}, nil
}

func (b *boltdb) Close() error {
	return errors.Wrap(b.db.Close(), "unable to close database")
}

func (b *boltdb) createLadderTable() error {
	return nil
}

func (b *boltdb) getUser(userID, channelID string) (*ladder, error) {
	var l ladder
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(channelID))
		if b == nil {
			return errors.New("bucket does not exist yet")
		}
		u := b.Get([]byte(userID))
		if len(u) > 0 {
			if err := json.Unmarshal(u, &l); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &l, nil
}

func (b *boltdb) getUserAbove(channelID string, rank int64) (*ladder, error) {
	ladder, err := b.getLadder(channelID)
	if err != nil {
		return nil, err
	}

	for _, u := range ladder {
		if u.Rank == rank-1 {
			return &u, nil
		}
	}

	return nil, errors.Wrap(errNotFound{}, "unable to get user above")
}

func (b *boltdb) getLastUser(channelID string) (*ladder, error) {
	users, err := b.getLadder(channelID)
	if err != nil {
		return nil, err
	}

	var l ladder
	for _, u := range users {
		if u.Rank > l.Rank {
			l = u
		}
	}

	return &l, nil
}

func (b *boltdb) getLadder(channelID string) ([]ladder, error) {
	l := make([]ladder, 0)
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(channelID))
		if bucket == nil {
			return errors.Wrap(errNotFound{}, "unable to get bucket")
		}

		return errors.Wrap(bucket.ForEach(func(k, v []byte) error {
			var u ladder
			if err := json.Unmarshal(v, &u); err != nil {
				return errors.Wrap(err, "unable to unmarshal user")
			}
			l = append(l, u)
			return nil
		}), "unable to get bucket contents")
	})

	if err != nil {
		return nil, err
	}

	if len(l) == 0 {
		return nil, errors.Wrap(errNotFound{}, "ladder is empty")
	}

	return l, nil
}

func (b *boltdb) clearLadder(channelID string) error {
	err := b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(channelID))
		if b != nil {
			return errors.Wrap(tx.DeleteBucket([]byte(channelID)), "unable to delete bucket")
		}

		return nil
	})

	return errors.Wrap(err, "unable to clear the ladder")
}

func (b *boltdb) removeUser(l ladder) error {
	err := b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(l.ChannelID))
		if b == nil {
			return nil
		}
		return errors.Wrap(b.Delete([]byte(l.UserID)), "unable to delete user")
	})

	return errors.Wrap(err, "unable to remove user")
}

func (b *boltdb) insert(l ladder) func(*bolt.Tx) error {
	return func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(l.ChannelID))
		if err != nil {
			return errors.Wrap(err, "unable to create bucket")
		}

		data, err := json.Marshal(l)
		if err != nil {
			return errors.Wrap(err, "unable to marshal user into json")
		}

		err = b.Put([]byte(l.UserID), data)
		return errors.Wrap(err, "error puting user")
	}
}

func (b *boltdb) insertOrUpdate(l ladder) error {
	err := b.db.Update(b.insert(l))
	return errors.Wrap(err, "unable to insert user")
}

func (b *boltdb) updateLadder(l []ladder) error {
	tx, err := b.db.Begin(true)
	if err != nil {
		return errors.Wrap(err, "unable to begin transaction")
	}

	for _, u := range l {
		if err := b.insert(u)(tx); err != nil {
			return err
		}
	}

	return errors.Wrap(tx.Commit(), "unable to commit transaction")
}
