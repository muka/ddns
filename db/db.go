package db

import (
	"errors"
	"log"
	"time"

	"github.com/boltdb/bolt"
)

var bdb *bolt.DB

const rrBucket = "rr"

//Connect create a bucket
func Connect(dbPath string) error {
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{
		Timeout: 10 * time.Second,
	})
	if err != nil {
		return err
	}

	bdb = db
	// Create dns bucket if doesn't exist
	createBucket(rrBucket)

	return nil
}

//Disconnect create a bucket
func Disconnect() error {
	if bdb == nil {
		return nil
	}
	return bdb.Close()
}

//createBucket create bucket if not exists
func createBucket(bucket string) error {
	return bdb.Update(func(tx *bolt.Tx) error {
		_, err1 := tx.CreateBucketIfNotExists([]byte(bucket))
		if err1 != nil {
			e := errors.New("Create bucket:  " + bucket)
			log.Println(e.Error())

			return e
		}
		return nil
	})
}

//DeleteRecord create a bucket
func DeleteRecord(key string, rtype uint16) (err error) {
	err = bdb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(rrBucket))
		err1 := b.Delete([]byte(key))

		if err1 != nil {
			e := errors.New("Delete record failed for domain:  " + key)
			log.Println(e.Error())
			return e
		}

		return nil
	})

	return err
}

//StoreRecord save a new record
func StoreRecord(key string, record string) error {
	return bdb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(rrBucket))
		err := b.Put([]byte(key), []byte(record))

		if err != nil {
			e := errors.New("Store record failed:  " + record)
			log.Println(e.Error())
			return e
		}

		return nil
	})
}

//GetRecord return a stored record for a domain
func GetRecord(key string, rtype uint16) ([]byte, error) {
	var v []byte

	err := bdb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(rrBucket))
		v = b.Get([]byte(key))

		if len(v) == 0 {
			e := errors.New("Record not found, key:  " + key)
			log.Println(e.Error())
			return e
		}

		return nil
	})

	return v, err
}
