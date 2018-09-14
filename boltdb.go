package main

import (
	"encoding/json"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
)

type boltdb struct {
	db *bolt.DB
}

func NewBoltDB(filename string) (DB, error) {
	db, err := bolt.Open(filename, 0600, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to open %v", filename)
	}

	return &boltdb{db: db}, nil
}

func (b *boltdb) Close() error {
	return errors.Wrap(b.db.Close(), "unable to close database")
}

func (b *boltdb) createLadderTable() error {
	return nil
}

func (b *boltdb) getUser(userID, channelID string) (*user, error) {
	var u user
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(channelID))
		if b == nil {
			return errors.New("bucket does not exist yet")
		}
		id := b.Get([]byte(userID))
		if len(id) > 0 {
			if err := json.Unmarshal(id, &u); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func (b *boltdb) getLastUser(channelID string) (*user, error) {
	users, err := b.getUsers(channelID)
	if err != nil {
		return nil, err
	}

	var u user
	for _, usr := range users {
		if usr.Rating > u.Rating {
			u = usr
		}
	}

	return &u, nil
}

func (b *boltdb) getBuckets() ([]string, error) {
	buckets := make([]string, 0)
	return buckets, errors.Wrap(b.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			buckets = append(buckets, string(name))
			return nil
		})
	}), "unable to get buckets")
}

func (b *boltdb) getUsers(channelID string) ([]user, error) {
	var users []user
	err := b.db.View(func(tx *bolt.Tx) error {
		//b := tx.Bucket([]byte(channelID))
		return nil
	})

	return users, err
}

func (b *boltdb) insert(u user) func(*bolt.Tx) error {
	return func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(u.ChannelID))
		if err != nil {
			return errors.Wrap(err, "unable to create bucket")
		}

		data, err := json.Marshal(u)
		if err != nil {
			return errors.Wrap(err, "unable to marshal user into json")
		}

		err = b.Put([]byte(u.ID), data)
		return errors.Wrap(err, "error puting user")
	}
}

func (b *boltdb) insertOrUpdate(u user) error {
	err := b.db.Update(b.insert(u))
	return errors.Wrap(err, "unable to insert user")
}

func (b *boltdb) updateUsers(users []user) error {
	tx, err := b.db.Begin(true)
	if err != nil {
		return errors.Wrap(err, "unable to begin transaction")
	}

	for _, user := range users {
		if err := b.insert(user)(tx); err != nil {
			return err
		}
	}

	return errors.Wrap(tx.Commit(), "unable to commit transaction")
}
