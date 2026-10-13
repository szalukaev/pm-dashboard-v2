package redmine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Attachment — файл, приложенный к задаче.
type Attachment struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	Filesize    int64  `json:"filesize"`
	ContentType string `json:"content_type"`
	Description string `json:"description"`
	Author      string `json:"author"`
	CreatedOn   string `json:"created_on"`
}

// FieldChange — одно изменённое поле записи журнала. Для большинства полей
// (статус, сотрудник) Redmine сообщает идентификаторы; в Old и New лежит то,
// что он сообщил, пока вызывающий не заменит идентификаторы названиями.
type FieldChange struct {
	// Field — что изменилось: "status", "priority", "assigned_to", "due_date",
	// "attachment", "relation", "cf_12" (настраиваемое поле) и так далее.
	Field string `json:"field"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

// HistoryEntry — запись журнала задачи: комментарий, изменённые поля или и то и другое.
type HistoryEntry struct {
	ID        int           `json:"id"`
	Author    string        `json:"author"`
	CreatedOn string        `json:"created_on"`
	Text      string        `json:"text"`
	Changes   []FieldChange `json:"changes"`
}

// Виды RelatedIssue помимо типов связей самого Redmine.
const (
	RelationParent = "parent"
	RelationChild  = "child"
)

// RelatedIssue — задача, связанная с показанной. Relation описывает связь с
// точки зрения показанной задачи: "blocks" — она блокирует связанную,
// "blocked" — заблокирована ею. Subject, Status и Accessible заполняет
// вызывающий для задач, которые пользователь вправе видеть.
type RelatedIssue struct {
	Relation   string `json:"relation"`
	ID         int    `json:"id"`
	Subject    string `json:"subject"`
	Status     string `json:"status"`
	Accessible bool   `json:"accessible"`
}

// IssueDetails — часть задачи, которая для карточки читается из Redmine
// напрямую: исходная разметка описания, файлы, комментарии, вся история
// изменений и связанные задачи.
type IssueDetails struct {
	Description string         `json:"description"`
	Attachments []Attachment   `json:"attachments"`
	Comments    []Comment      `json:"comments"`
	History     []HistoryEntry `json:"history"`
	Related     []RelatedIssue `json:"related"`
}

type issueDetailsResp struct {
	Issue struct {
		Description string `json:"description"`
		Attachments []struct {
			ID          int    `json:"id"`
			Filename    string `json:"filename"`
			Filesize    int64  `json:"filesize"`
			ContentType string `json:"content_type"`
			Description string `json:"description"`
			Author      struct {
				Name string `json:"name"`
			} `json:"author"`
			CreatedOn string `json:"created_on"`
		} `json:"attachments"`
		Journals []struct {
			ID   int `json:"id"`
			User struct {
				Name string `json:"name"`
			} `json:"user"`
			Notes     string `json:"notes"`
			CreatedOn string `json:"created_on"`
			Details   []struct {
				Property string `json:"property"`
				Name     string `json:"name"`
				OldValue string `json:"old_value"`
				NewValue string `json:"new_value"`
			} `json:"details"`
		} `json:"journals"`
		Parent *struct {
			ID int `json:"id"`
		} `json:"parent"`
		Children []struct {
			ID int `json:"id"`
		} `json:"children"`
		Relations []struct {
			IssueID      int    `json:"issue_id"`
			IssueToID    int    `json:"issue_to_id"`
			RelationType string `json:"relation_type"`
		} `json:"relations"`
	} `json:"issue"`
}

// inverseRelations: как связь читается с другого её конца.
var inverseRelations = map[string]string{
	"relates":     "relates",
	"duplicates":  "duplicated",
	"duplicated":  "duplicates",
	"blocks":      "blocked",
	"blocked":     "blocks",
	"precedes":    "follows",
	"follows":     "precedes",
	"copied_to":   "copied_from",
	"copied_from": "copied_to",
}

// changedField называет, о чём запись журнала (см. FieldChange.Field).
func changedField(property, name string) string {
	switch property {
	case "attr":
		return strings.TrimSuffix(name, "_id")
	case "cf":
		return "cf_" + name
	default: // "attachment", "relation"
		return property
	}
}

// GetIssueDetails загружает описание, файлы, историю и связанные задачи
// одной задачи.
func (c *Client) GetIssueDetails(issueID int) (*IssueDetails, error) {
	data, err := c.doRequest(fmt.Sprintf("/issues/%d.json?include=attachments,journals,relations,children", issueID))
	if err != nil {
		return nil, err
	}
	return parseIssueDetails(data, issueID)
}

func parseIssueDetails(data []byte, issueID int) (*IssueDetails, error) {
	var resp issueDetailsResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	d := &IssueDetails{
		Description: resp.Issue.Description,
		Attachments: []Attachment{},
		Comments:    []Comment{},
		History:     []HistoryEntry{},
		Related:     []RelatedIssue{},
	}
	for _, a := range resp.Issue.Attachments {
		d.Attachments = append(d.Attachments, Attachment{
			ID:          a.ID,
			Filename:    a.Filename,
			Filesize:    a.Filesize,
			ContentType: a.ContentType,
			Description: a.Description,
			Author:      a.Author.Name,
			CreatedOn:   a.CreatedOn,
		})
	}
	for _, j := range resp.Issue.Journals {
		entry := HistoryEntry{ID: j.ID, Author: j.User.Name, CreatedOn: j.CreatedOn, Text: j.Notes, Changes: []FieldChange{}}
		for _, detail := range j.Details {
			change := FieldChange{Field: changedField(detail.Property, detail.Name), Old: detail.OldValue, New: detail.NewValue}
			if detail.Property == "relation" {
				// name — тип связи, значения — номера задач
				change.Old, change.New = issueRef(detail.OldValue), issueRef(detail.NewValue)
			}
			entry.Changes = append(entry.Changes, change)
		}
		if entry.Text != "" || len(entry.Changes) > 0 {
			d.History = append(d.History, entry)
		}
		if j.Notes == "" {
			continue
		}
		d.Comments = append(d.Comments, Comment{
			ID:        j.ID,
			Author:    j.User.Name,
			Text:      j.Notes,
			CreatedOn: j.CreatedOn,
		})
	}

	if resp.Issue.Parent != nil {
		d.Related = append(d.Related, RelatedIssue{Relation: RelationParent, ID: resp.Issue.Parent.ID})
	}
	for _, child := range resp.Issue.Children {
		d.Related = append(d.Related, RelatedIssue{Relation: RelationChild, ID: child.ID})
	}
	for _, rel := range resp.Issue.Relations {
		related := RelatedIssue{Relation: rel.RelationType, ID: rel.IssueToID}
		if rel.IssueToID == issueID {
			// Связь записана со стороны другой задачи: разворачиваем её
			related.ID = rel.IssueID
			if inverse, ok := inverseRelations[rel.RelationType]; ok {
				related.Relation = inverse
			}
		}
		d.Related = append(d.Related, related)
	}
	return d, nil
}

// issueRef записывает номер задачи так, как на задачи ссылаются: "#123".
func issueRef(value string) string {
	if _, err := strconv.Atoi(value); err != nil {
		return value
	}
	return "#" + value
}

type versionsResp struct {
	Versions []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"versions"`
}

// GetVersionNames возвращает названия версий проекта по их идентификаторам.
func (c *Client) GetVersionNames(projectID int) (map[int]string, error) {
	data, err := c.doRequest(fmt.Sprintf("/projects/%d/versions.json", projectID))
	if err != nil {
		return nil, err
	}
	var resp versionsResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	names := make(map[int]string, len(resp.Versions))
	for _, v := range resp.Versions {
		names[v.ID] = v.Name
	}
	return names, nil
}

// DownloadAttachment отдаёт содержимое файла потоком. Возвращённое тело
// должен закрыть вызывающий.
func (c *Client) DownloadAttachment(id int) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/attachments/download/%d", c.BaseURL, id), nil)
	if err != nil {
		return nil, err
	}
	if c.APIKey != "" {
		req.Header.Set("X-Redmine-API-Key", c.APIKey)
	}
	if c.BasicLogin != "" {
		req.SetBasicAuth(c.BasicLogin, c.BasicPass)
	}
	// Файлы бывают большими: общего тайм-аута клиента на чтение тела нет.
	client := &http.Client{Transport: c.HTTPClient.Transport}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("redmine returned %d for attachment %d", resp.StatusCode, id)
	}
	return resp.Body, nil
}
