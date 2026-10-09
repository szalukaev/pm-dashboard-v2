package redmine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Attachment is a file attached to an issue.
type Attachment struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	Filesize    int64  `json:"filesize"`
	ContentType string `json:"content_type"`
	Description string `json:"description"`
	Author      string `json:"author"`
	CreatedOn   string `json:"created_on"`
}

// FieldChange is one changed field of a journal entry. Redmine reports ids
// for most fields (a status, a member); Old and New hold what it reported
// until the caller replaces the ids with names.
type FieldChange struct {
	// Field is what changed: "status", "priority", "assigned_to", "due_date",
	// "attachment", "relation", "cf_12" (a custom field) and so on.
	Field string `json:"field"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

// HistoryEntry is a journal entry of an issue: a note, changed fields or both.
type HistoryEntry struct {
	ID        int           `json:"id"`
	Author    string        `json:"author"`
	CreatedOn string        `json:"created_on"`
	Text      string        `json:"text"`
	Changes   []FieldChange `json:"changes"`
}

// Kinds of RelatedIssue besides the relation types of Redmine.
const (
	RelationParent = "parent"
	RelationChild  = "child"
)

// RelatedIssue is an issue tied to the one shown. Relation is told from the
// point of view of the shown issue: "blocks" — it blocks the related one,
// "blocked" — it is blocked by it. Subject, Status and Accessible are filled
// by the caller for the issues the user may see.
type RelatedIssue struct {
	Relation   string `json:"relation"`
	ID         int    `json:"id"`
	Subject    string `json:"subject"`
	Status     string `json:"status"`
	Accessible bool   `json:"accessible"`
}

// IssueDetails is the part of an issue that is read live from Redmine for
// the task card: the raw description markup, files, comments, the whole
// history of changes and the related issues.
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

// inverseRelations: how a relation reads from its other end.
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

// changedField names what a journal detail is about (see FieldChange.Field).
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

// GetIssueDetails loads description, attachments, history and related
// issues of an issue.
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
				// name is the relation type, the values are issue numbers
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
			// The relation is recorded from the other issue: turn it around
			related.ID = rel.IssueID
			if inverse, ok := inverseRelations[rel.RelationType]; ok {
				related.Relation = inverse
			}
		}
		d.Related = append(d.Related, related)
	}
	return d, nil
}

// issueRef shows an issue number the way issues are referred to: "#123".
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

// GetVersionNames returns the names of the versions of a project by id.
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

// DownloadAttachment streams the content of an attachment. The caller must
// close the returned body.
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
	// Attachments may be large: no overall client timeout for the body.
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
