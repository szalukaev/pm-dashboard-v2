package manager

import (
	"sort"
	"time"
)

// События, которые уходят открытым страницам по WebSocket.
const (
	// EventSyncStatus: синхронизация началась или закончилась, см. SyncStatus.
	EventSyncStatus = "sync-status"
	// EventIssuesChanged: задачи добавлены, изменены или удалены, см. IssueChanges.
	EventIssuesChanged = "issues-changed"
	// EventCollectionComplete: синхронизация завершилась успешно.
	EventCollectionComplete = "collection-complete"
)

type SyncStatus struct {
	Status string    `json:"status"` // running | success | error
	At     time.Time `json:"at"`
}

// IssueChanges перечисляет только номера задач: каждая страница сама
// перечитывает то, что показывает, с правами своего пользователя.
type IssueChanges struct {
	Added   []int `json:"added"`
	Updated []int `json:"updated"`
	Removed []int `json:"removed"`
}

func (c IssueChanges) Empty() bool {
	return len(c.Added) == 0 && len(c.Updated) == 0 && len(c.Removed) == 0
}

// DiffIssues сравнивает два снимка (номер задачи → отпечаток).
func DiffIssues(before, after map[int]string) IssueChanges {
	changes := IssueChanges{Added: []int{}, Updated: []int{}, Removed: []int{}}
	for id, hash := range after {
		old, existed := before[id]
		switch {
		case !existed:
			changes.Added = append(changes.Added, id)
		case old != hash:
			changes.Updated = append(changes.Updated, id)
		}
	}
	for id := range before {
		if _, still := after[id]; !still {
			changes.Removed = append(changes.Removed, id)
		}
	}
	sort.Ints(changes.Added)
	sort.Ints(changes.Updated)
	sort.Ints(changes.Removed)
	return changes
}
