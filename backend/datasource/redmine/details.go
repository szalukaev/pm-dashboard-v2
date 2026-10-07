package redmine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// IssueDetails is the part of an issue that is read live from Redmine for
// the task card: the raw description markup, files and comment history.
type IssueDetails struct {
	Description string       `json:"description"`
	Attachments []Attachment `json:"attachments"`
	Comments    []Comment    `json:"comments"`
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
		} `json:"journals"`
	} `json:"issue"`
}

// GetIssueDetails loads description, attachments and comments of an issue.
func (c *Client) GetIssueDetails(issueID int) (*IssueDetails, error) {
	data, err := c.doRequest(fmt.Sprintf("/issues/%d.json?include=attachments,journals", issueID))
	if err != nil {
		return nil, err
	}
	var resp issueDetailsResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	d := &IssueDetails{
		Description: resp.Issue.Description,
		Attachments: []Attachment{},
		Comments:    []Comment{},
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
	return d, nil
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
