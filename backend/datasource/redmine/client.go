package redmine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL   string
	APIKey    string
	HTTPClient *http.Client
}

type projectResp struct {
	Projects []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Parent   *struct{ ID int } `json:"parent"`
	} `json:"projects"`
	TotalCount int `json:"total_count"`
}

type issueResp struct {
	Issues []struct {
		ID          int    `json:"id"`
		Subject     string `json:"subject"`
		Description string `json:"description"`
		Project     struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
		Status struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"status"`
		Priority struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"priority"`
		AssignedTo *struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"assigned_to"`
		Category *struct {
			Name string `json:"name"`
		} `json:"category"`
		StartDate    *string  `json:"start_date"`
		DueDate      *string  `json:"due_date"`
		DoneRatio    int      `json:"done_ratio"`
		EstimatedHours *float64 `json:"estimated_hours"`
		SpentHours   float64  `json:"spent_hours"`
		Tracker      struct {
			Name string `json:"name"`
		} `json:"tracker"`
		Author struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"issues"`
	TotalCount int `json:"total_count"`
}

type statusResp struct {
	Statuses []struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		IsClosed bool  `json:"is_closed"`
	} `json:"statuses"`
}

type memberResp struct {
	Memberships []struct {
		User struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Login string `json:"login"`
		} `json:"user"`
	} `json:"memberships"`
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) doRequest(path string) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Redmine-API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("redmine returned %d for %s", resp.StatusCode, path)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) TestConnection() error {
	_, err := c.doRequest("/projects.json?limit=1")
	return err
}

func (c *Client) GetProjects() ([]Project, error) {
	var all []Project
	offset := 0
	for {
		data, err := c.doRequest(fmt.Sprintf("/projects.json?limit=100&offset=%d", offset))
		if err != nil {
			return nil, err
		}
		var resp projectResp
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		for _, p := range resp.Projects {
			var parentID *int
			if p.Parent != nil {
				pid := p.Parent.ID
				parentID = &pid
			}
			all = append(all, Project{ExternalID: p.ID, Name: p.Name, ParentID: parentID})
		}
		offset += len(resp.Projects)
		if offset >= resp.TotalCount || len(resp.Projects) == 0 {
			break
		}
	}
	return all, nil
}

func (c *Client) GetIssues(projectID int) ([]Issue, error) {
	var all []Issue
	offset := 0
	for {
		path := fmt.Sprintf("/issues.json?limit=100&offset=%d&status_id=*", offset)
		if projectID > 0 {
			path += fmt.Sprintf("&project_id=%d", projectID)
		}
		data, err := c.doRequest(path)
		if err != nil {
			return nil, err
		}
		var resp issueResp
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		for _, i := range resp.Issues {
			iss := Issue{
				ExternalID:     i.ID,
				ProjectID:      i.Project.ID,
				ProjectName:    i.Project.Name,
				Subject:        i.Subject,
				Description:    i.Description,
				StatusName:     i.Status.Name,
				StatusID:       i.Status.ID,
				PriorityName:   i.Priority.Name,
				PriorityID:     i.Priority.ID,
				StartDate:      i.StartDate,
				DueDate:        i.DueDate,
				DoneRatio:      i.DoneRatio,
				EstimatedHours: i.EstimatedHours,
				SpentHours:     i.SpentHours,
				TrackerName:    i.Tracker.Name,
				AuthorName:     i.Author.Name,
			}
			if i.AssignedTo != nil {
				iss.AssignedToName = i.AssignedTo.Name
				aid := i.AssignedTo.ID
				iss.AssignedToID = &aid
			}
			if i.Category != nil {
				iss.CategoryName = i.Category.Name
			}
			all = append(all, iss)
		}
		offset += len(resp.Issues)
		if offset >= resp.TotalCount || len(resp.Issues) == 0 {
			break
		}
	}
	return all, nil
}

func (c *Client) UpdateIssue(id int, fields map[string]interface{}) error {
	issue := map[string]interface{}{"issue": fields}
	body, _ := json.Marshal(issue)
	url := fmt.Sprintf("%s/issues/%d.json", c.BaseURL, id)
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Redmine-API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytesReader(body))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("redmine update returned %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) GetStatuses() ([]Status, error) {
	data, err := c.doRequest("/issue_statuses.json")
	if err != nil {
		return nil, err
	}
	var resp statusResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	var statuses []Status
	for _, s := range resp.Statuses {
		statuses = append(statuses, Status{
			ExternalID: s.ID,
			Name:       s.Name,
			IsClosed:   s.IsClosed,
		})
	}
	return statuses, nil
}

func (c *Client) GetProjectMembers(projectID int) ([]Member, error) {
	var all []Member
	offset := 0
	for {
		data, err := c.doRequest(fmt.Sprintf("/projects/%d/memberships.json?limit=100&offset=%d", projectID, offset))
		if err != nil {
			return nil, err
		}
		var resp memberResp
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		for _, m := range resp.Memberships {
			all = append(all, Member{
				ExternalID: m.User.ID,
				Name:       m.User.Name,
				Login:      m.User.Login,
			})
		}
		offset += len(resp.Memberships)
		if len(resp.Memberships) == 0 {
			break
		}
	}
	return all, nil
}

// bytesReader wraps a byte slice into an io.ReadCloser for request bodies.
type bytesReaderData struct {
	data []byte
	pos  int
}

func (r *bytesReaderData) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *bytesReaderData) Close() error { return nil }

func bytesReader(data []byte) io.ReadCloser {
	return &bytesReaderData{data: data}
}
