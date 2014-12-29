package main

import (
	"database/sql"
	"errors"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const tableDef = `(
	'id' INTEGER PRIMARY KEY AUTOINCREMENT,
	'time' timestamp default CURRENT_TIMESTAMP,
	'status' text NOT NULL,
	'type' text NOT NULL, 
	'domain' text NOT NULL,
	'owner' text NOT NULL,
	'repo' text NOT NULL,
	'branch' text NOT NULL,
	'commit' text NOT NULL,
	'baseowner' text NOT NULL, 
	'baserepo' text NOT NULL, 
	'basebranch' text NOT NULL, 
	'log' blob
)`

var (
	DB *sqlx.DB
)

// Bootstrap database
func InitDB() {
	DB = sqlx.MustConnect("sqlite3", Config.DataDir+"/deadci.sqlite")
	DB.MustExec("CREATE TABLE IF NOT EXISTS deadci " + tableDef)
	DB.MustExec("CREATE INDEX IF NOT EXISTS status_index on deadci (status)")
	DB.MustExec("CREATE INDEX IF NOT EXISTS domain_index on deadci (domain)")
	DB.MustExec("CREATE INDEX IF NOT EXISTS owner_index on deadci (domain, owner)")
	DB.MustExec("CREATE INDEX IF NOT EXISTS repo_index on deadci (domain, owner, repo)")
	DB.MustExec("CREATE INDEX IF NOT EXISTS branch_index on deadci (domain, owner, repo, branch)")
	DB.MustExec("CREATE UNIQUE INDEX IF NOT EXISTS combined_index on deadci (domain, owner, repo, branch, `commit`)")
}

// Get a pending event, mark it as running
func PopEvent() (*Event, error) {
	event := Event{}

	err := DB.Get(&event, "SELECT * FROM deadci WHERE status = 'pending' ORDER BY id ASC LIMIT 1")
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		} else {
			return nil, err
		}
	}
	// Mark as running and return
	event.Status = StatusRunning
	if len(event.Log) != 0 {
		event.Log = []byte("Retrying...\n")
	}
	e := &event
	err = e.Update()
	if err != nil {
		return nil, err
	}

	return e, nil
}

func GetEvent(domain, owner, repo, branch, commit string) (*Event, error) {
	event := Event{}

	err := DB.Get(&event, "SELECT * FROM deadci WHERE domain = ? AND owner = ? AND repo = ? AND branch = ? AND `commit` = ?", domain, owner, repo, branch, commit)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		} else {
			return nil, err
		}
	}
	return &event, nil
}

func GetEvents(args ...string) ([]Event, error) {
	events := []Event{}

	if len(args) > 5 {
		panic("too many arguments to GetEvents()")
	}

	query := "SELECT time,status,domain,owner, repo, branch, `commit` FROM deadci"
	dbargs := make([]interface{}, 0)
	if len(args) >= 1 {
		query += " WHERE domain = ?"
		dbargs = append(dbargs, args[0])
		if len(args) >= 2 {
			query += " AND owner = ?"
			dbargs = append(dbargs, args[1])
			if len(args) >= 3 {
				query += " AND repo = ?"
				dbargs = append(dbargs, args[2])
				if len(args) >= 4 {
					query += " AND branch = ?"
					dbargs = append(dbargs, args[3])
					if len(args) == 5 {
						query += " AND `commit` = ?"
						dbargs = append(dbargs, args[4])
					}
				}
			}
		}
	}
	query += " ORDER BY id DESC LIMIT 500"
	err := DB.Select(&events, query, dbargs...)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (e *Event) Insert() error {
	if e.ID != 0 {
		return errors.New("Cannot Insert event with an ID. Use Update()")
	}

	res, err := DB.NamedExec("INSERT INTO deadci (time,status,`type`,domain,owner, repo, branch, `commit`, baseowner, baserepo, basebranch, log) VALUES(:time, :status, :type, :domain, :owner, :repo, :branch, :commit, :baseowner, :baserepo, :basebranch, :log)", e)
	if err != nil {
		return err
	} else {
		id, err := res.LastInsertId()
		if err != nil {
			return err
		} else {
			e.ID = int(id)
			return nil
		}
	}
}

func (e *Event) Update() error {
	if e.ID == 0 {
		return errors.New("Cannot update event with no ID. Use Insert()")
	}
	_, err := DB.NamedExec("UPDATE deadci SET time = :time , status = :status, `type` = :type, domain = :domain, owner = :owner, repo = :repo, branch = :branch, `commit` = :commit, baseowner = :baseowner, baserepo = :baserepo, basebranch = :basebranch, log = :log WHERE id= :id", e)
	if err != nil {
		return err
	} else {
		return nil
	}
}
