package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type SavedTask struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

type SavedLog struct {
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

type TaskStore struct {
	Tasks []SavedTask `json:"tasks"`
	Log   []SavedLog  `json:"log"`
}

func tasksPath() string {
	dir, err := os.Getwd()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir,"data","tasks.json")
}

func loadTasks() ([]Task, []LogEntry, error) {
	data, err := os.ReadFile(tasksPath())
	if err != nil {
		return nil, nil, err // no file yet — fresh start
	}

	var store TaskStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, nil, err
	}

	var tasks []Task
	for _, t := range store.Tasks {
		tasks = append(tasks, Task{Text: t.Text, Done: t.Done})
	}

	var log []LogEntry
	for _, e := range store.Log {
		log = append(log, LogEntry{Text: e.Text, At: e.At})
	}

	return tasks, log, nil
}

func saveTasks(tasks []Task, log []LogEntry) error {
	path := tasksPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	var store TaskStore
	for _, t := range tasks {
		store.Tasks = append(store.Tasks, SavedTask{Text: t.Text, Done: t.Done})
	}
	for _, e := range log {
		store.Log = append(store.Log, SavedLog{Text: e.Text, At: e.At})
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}