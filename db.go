package main

import (
	"encoding/json"
	"github.com/HouzuoGuo/tiedot/db"
	"github.com/phayes/hookserve/hookserve"
	"sync"
)

var (
	Dir string = "/tmp/MyDatabase"
	DB  *db.DB
	Col *db.Col
	Mux sync.Mutex
)

// Bootstrap database
func DBInit() {
	DB, err := db.OpenDB(Dir)
	if err != nil {
		panic(err)
	}
	Col = DB.Use("events")
	if Col == nil {
		err := DB.Create("events")
		if err != nil {
			panic(err)
		}
		Col = DB.Use("events")
		if Col == nil {
			panic("Could not connect to newly created 'events' collection")
		}
		if err := Col.Index([]string{"domain", "owner", "repo", "branch", "commit"}); err != nil {
			panic(err)
		}
		if err := Col.Index([]string{"status"}); err != nil {
			panic(err)
		}
		if err := Col.Index([]string{"domain"}); err != nil {
			panic(err)
		}
		if err := Col.Index([]string{"owner"}); err != nil {
			panic(err)
		}
		if err := Col.Index([]string{"repo"}); err != nil {
			panic(err)
		}
		if err := Col.Index([]string{"branch"}); err != nil {
			panic(err)
		}
		if err := Col.Index([]string{"commit"}); err != nil {
			panic(err)
		}
	}
}

// Get a pending event, mark it as running
func PopEvent() (*Event, error) {
	Mux.Lock()
	defer Mux.Unlock()

	var query interface{}
	json.Unmarshal([]byte(`[{"eq": "pending", "in": ["status"]}]`), &query)
	queryResult := make(map[int]struct{}) // query result (document IDs) goes into map keys
	if err := db.EvalQuery(query, Col, &queryResult); err != nil {
		return nil, err
	}
	// Query result are document IDs
	for id := range queryResult {
		readBack, err := Col.Read(id)
		if err != nil {
			return nil, err
		}
		e := &Event{
			ID: id,
			Event: hookserve.Event{
				Owner:  readBack["owner"].(string),
				Repo:   readBack["repo"].(string),
				Branch: readBack["branch"].(string),
				Commit: readBack["commit"].(string),
			},
			Status: StatusRunning,
			Log:    readBack["log"].([]byte),
		}

		// Save it back to the database marked as running
		err = Col.Update(e.ID, e.DBItem())
		if err != nil {
			return nil, err
		}

		return e, nil
	}

	// Nothing in the queue
	return nil, nil
}

func GetEvent(owner, repo, branch, commit string) (*Event, error) {
	Mux.Lock()
	defer Mux.Unlock()

	var query interface{}
	json.Unmarshal([]byte(`[{"eq": "`+owner+`", "in": ["owner"]}, {"eq": "`+repo+`", "in": ["repo"]}, {"eq": "`+branch+`", "in": ["branch"]}, {"eq": "`+commit+`", "in": ["commit"]}]`), &query)
	queryResult := make(map[int]struct{}) // query result (document IDs) goes into map keys
	if err := db.EvalQuery(query, Col, &queryResult); err != nil {
		return nil, err
	}
	// Query result are document IDs
	for id := range queryResult {
		readBack, err := Col.Read(id)
		if err != nil {
			return nil, err
		}
		return &Event{
			ID: id,
			Event: hookserve.Event{
				Owner:  readBack["owner"].(string),
				Repo:   readBack["repo"].(string),
				Branch: readBack["branch"].(string),
				Commit: readBack["commit"].(string),
			},
			Status: readBack["status"].(EventStatus),
			Log:    readBack["log"].([]byte),
		}, nil
	}
	panic("unreachable")
}

func (e *Event) DBItem() map[string]interface{} {
	return map[string]interface{}{
		"owner":  e.Owner,
		"repo":   e.Repo,
		"branch": e.Branch,
		"commit": e.Commit,
		"status": string(e.Status),
		"log":    string(e.Log),
	}
}

func (e *Event) Insert() error {
	id, err := Col.Insert(e.DBItem())
	e.ID = id
	return err
}

func (e *Event) Update() error {
	Mux.Lock()
	defer Mux.Unlock()

	// If we dont' know the ID, then get it from the DB
	if e.ID == 0 {
		var query interface{}
		json.Unmarshal([]byte(`[{"eq": "`+e.Owner+`", "in": ["owner"]}, {"eq": "`+e.Repo+`", "in": ["repo"]}, {"eq": "`+e.Branch+`", "in": ["branch"]}, {"eq": "`+e.Commit+`", "in": ["commit"]}]`), &query)
		queryResult := make(map[int]struct{}) // query result (document IDs) goes into map keys
		if err := db.EvalQuery(query, Col, &queryResult); err != nil {
			return err
		}
		// Query result are document IDs
		for id := range queryResult {
			e.ID = id
			break
		}
	}

	return Col.Update(e.ID, e.DBItem())
}
