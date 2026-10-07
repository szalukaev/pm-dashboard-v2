package redmine

import (
	"encoding/json"
	"strconv"
	"strings"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// pageRetryDelay is the pause before retrying a failed page (grows per attempt).
var pageRetryDelay = time.Second

type Client struct {
	BaseURL    string
	APIKey     string
	BasicLogin string
	BasicPass  string
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
		CustomFields []customFieldValue `json:"custom_fields"`
	} `json:"issues"`
	TotalCount int `json:"total_count"`
}

type customFieldValue struct {
	ID    int         `json:"id"`
	Value interface{} `json:"value"`
}

// developerEstimateFieldID is the Redmine custom field "Оценка разработчика".
const developerEstimateFieldID = 18

// issueEstimate prefers the developer estimate when it is set and positive,
// otherwise falls back to the standard estimated_hours.
func issueEstimate(estimated *float64, fields []customFieldValue) *float64 {
	if v := developerEstimate(fields); v != nil {
		return v
	}
	return estimated
}

// developerEstimate returns the developer estimate when it is set and positive.
func developerEstimate(fields []customFieldValue) *float64 {
	for _, f := range fields {
		if f.ID != developerEstimateFieldID {
			continue
		}
		s, ok := f.Value.(string)
		if !ok {
			return nil
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(strings.Replace(s, ",", ".", 1)), 64)
		if err == nil && v > 0 {
			return &v
		}
		return nil
	}
	return nil
}

// EstimateFields returns the Redmine fields that store an issue's estimate:
// the developer estimate when it is already set, otherwise the standard
// estimated_hours. A nil hours value clears the estimate.
func (c *Client) EstimateFields(issueID int, hours *float64) (map[string]interface{}, error) {
	data, err := c.doRequest(fmt.Sprintf("/issues/%d.json", issueID))
	if err != nil {
		return nil, err
	}
	var resp struct {
		Issue struct {
			CustomFields []customFieldValue `json:"custom_fields"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if developerEstimate(resp.Issue.CustomFields) != nil {
		value := ""
		if hours != nil {
			value = strconv.FormatFloat(*hours, 'f', -1, 64)
		}
		return map[string]interface{}{
			"custom_fields": []map[string]interface{}{{"id": developerEstimateFieldID, "value": value}},
		}, nil
	}
	if hours == nil {
		return map[string]interface{}{"estimated_hours": ""}, nil
	}
	return map[string]interface{}{"estimated_hours": *hours}, nil
}

type timeEntryResp struct {
	TimeEntries []struct {
		ID    int `json:"id"`
		Issue *struct {
			ID int `json:"id"`
		} `json:"issue"`
		Activity struct {
			Name string `json:"name"`
		} `json:"activity"`
		Hours float64 `json:"hours"`
	} `json:"time_entries"`
	TotalCount int `json:"total_count"`
}

// TimeEntry is a single spent-time record linked to an issue.
type TimeEntry struct {
	ID           int
	IssueID      int
	ActivityName string
	Hours        float64
}

// GetTimeEntries returns the time entries of a project that are linked to
// issues, spent on or after `from` (YYYY-MM-DD; empty = all). Subprojects
// are excluded: they are requested on their own.
func (c *Client) GetTimeEntries(projectID int, from string) ([]TimeEntry, error) {
	path := fmt.Sprintf("/time_entries.json?project_id=%d&subproject_id=!*", projectID)
	if from != "" {
		path += "&from=" + from
	}
	pages, err := c.getPages(path)
	if err != nil {
		return nil, err
	}
	var all []TimeEntry
	for _, data := range pages {
		var resp timeEntryResp
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		for _, e := range resp.TimeEntries {
			if e.Issue == nil {
				continue
			}
			all = append(all, TimeEntry{
				ID:           e.ID,
				IssueID:      e.Issue.ID,
				ActivityName: e.Activity.Name,
				Hours:        e.Hours,
			})
		}
	}
	return all, nil
}

type statusResp struct {
	Statuses []struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		IsClosed bool  `json:"is_closed"`
	} `json:"issue_statuses"`
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

func NewClient(baseURL, apiKey, basicLogin, basicPass string) *Client {
	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		BasicLogin: basicLogin,
		BasicPass:  basicPass,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) doRequest(path string) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.APIKey != "" {
		req.Header.Set("X-Redmine-API-Key", c.APIKey)
	}
	if c.BasicLogin != "" {
		req.SetBasicAuth(c.BasicLogin, c.BasicPass)
	}
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

const (
	pageSize = 100
	// Redmine answers 500 when hit with many parallel list requests.
	pageWorkers  = 2
	pageAttempts = 3
)

// getPages loads every page of a paginated list. The first page gives
// total_count, the rest are requested concurrently and returned in order.
// A failed page is retried a few times before the whole list fails.
func (c *Client) getPages(path string) ([][]byte, error) {
	page := func(offset int) ([]byte, error) {
		var data []byte
		var err error
		for attempt := 1; attempt <= pageAttempts; attempt++ {
			data, err = c.doRequest(fmt.Sprintf("%s&limit=%d&offset=%d", path, pageSize, offset))
			if err == nil {
				return data, nil
			}
			if attempt < pageAttempts {
				time.Sleep(time.Duration(attempt) * pageRetryDelay)
			}
		}
		return nil, err
	}
	first, err := page(0)
	if err != nil {
		return nil, err
	}
	var meta struct {
		TotalCount int `json:"total_count"`
	}
	if err := json.Unmarshal(first, &meta); err != nil {
		return nil, err
	}
	pages := (meta.TotalCount + pageSize - 1) / pageSize
	if pages < 1 {
		pages = 1
	}
	out := make([][]byte, pages)
	out[0] = first

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		sem      = make(chan struct{}, pageWorkers)
	)
	for i := 1; i < pages; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			data, err := page(i * pageSize)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			out[i] = data
		}(i)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
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

// GetIssues returns the issues of a project without its subprojects (they
// are requested on their own); projectID 0 means all projects. A non-nil
// updatedSince limits the result to issues changed since that moment.
func (c *Client) GetIssues(projectID int, updatedSince *time.Time) ([]Issue, error) {
	path := "/issues.json?status_id=*"
	if projectID > 0 {
		path += fmt.Sprintf("&project_id=%d&subproject_id=!*", projectID)
	}
	if updatedSince != nil {
		path += "&updated_on=" + url.QueryEscape(">="+updatedSince.UTC().Format(time.RFC3339))
	}
	pages, err := c.getPages(path)
	if err != nil {
		return nil, err
	}
	var all []Issue
	for _, data := range pages {
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
				EstimatedHours: issueEstimate(i.EstimatedHours, i.CustomFields),
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
	}
	return all, nil
}

type Comment struct {
	ID        int    `json:"id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	CreatedOn string `json:"created_on"`
}

type commentsResp struct {
	Issue struct {
		Journals []struct {
			ID        int    `json:"id"`
			User      struct {
				Name string `json:"name"`
			} `json:"user"`
			Notes     string `json:"notes"`
			CreatedOn string `json:"created_on"`
		} `json:"journals"`
	} `json:"issue"`
}

func (c *Client) GetIssueComments(issueID int) ([]Comment, error) {
	data, err := c.doRequest(fmt.Sprintf("/issues/%d.json?include=journals", issueID))
	if err != nil {
		return nil, err
	}
	var resp commentsResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	var all []Comment
	for _, j := range resp.Issue.Journals {
		if j.Notes == "" {
			continue
		}
		all = append(all, Comment{
			ID:        j.ID,
			Author:    j.User.Name,
			Text:      j.Notes,
			CreatedOn: j.CreatedOn,
		})
	}
	return all, nil
}

func (c *Client) AddIssueComment(issueID int, text string) error {
	body := map[string]interface{}{
		"issue": map[string]interface{}{
			"notes": text,
		},
	}
	b, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/issues/%d.json", c.BaseURL, issueID)
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return err
	}
	if c.APIKey != "" {
		req.Header.Set("X-Redmine-API-Key", c.APIKey)
	}
	if c.BasicLogin != "" {
		req.SetBasicAuth(c.BasicLogin, c.BasicPass)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytesReader(b))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("redmine comment returned %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) UpdateIssue(id int, fields map[string]interface{}) error {
	issue := map[string]interface{}{"issue": fields}
	body, _ := json.Marshal(issue)
	url := fmt.Sprintf("%s/issues/%d.json", c.BaseURL, id)
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return err
	}
	if c.APIKey != "" {
		req.Header.Set("X-Redmine-API-Key", c.APIKey)
	}
	if c.BasicLogin != "" {
		req.SetBasicAuth(c.BasicLogin, c.BasicPass)
	}
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

type priorityResp struct {
	Priorities []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		IsDefault bool  `json:"is_default"`
	} `json:"issue_priorities"`
	TotalCount int `json:"total_count"`
}

func (c *Client) GetPriorities() ([]Priority, error) {
	data, err := c.doRequest("/enumerations/issue_priorities.json")
	if err != nil {
		return nil, err
	}
	var resp priorityResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	var all []Priority
	for _, p := range resp.Priorities {
		all = append(all, Priority{
			ExternalID: p.ID,
			Name:       p.Name,
		})
	}
	return all, nil
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

type userResp struct {
	Users []struct {
		ID        int    `json:"id"`
		FirstName string `json:"firstname"`
		LastName  string `json:"lastname"`
		Login     string `json:"login"`
		Status    int    `json:"status"`
	} `json:"users"`
	TotalCount int `json:"total_count"`
}

// GetUsers returns all active users from Redmine.
func (c *Client) GetUsers() ([]Member, error) {
	var all []Member
	offset := 0
	for {
		data, err := c.doRequest(fmt.Sprintf("/users.json?limit=100&offset=%d&status=1", offset))
		if err != nil {
			return nil, err
		}
		var resp userResp
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		for _, u := range resp.Users {
			name := strings.TrimSpace(u.FirstName + " " + u.LastName)
			all = append(all, Member{
				ExternalID: u.ID,
				Name:       name,
				Login:      u.Login,
			})
		}
		offset += len(resp.Users)
		if offset >= resp.TotalCount || len(resp.Users) == 0 {
			break
		}
	}
	return all, nil
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

type categoryResp struct {
	Categories []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"issue_categories"`
}

// Category is an issue category of a Redmine project.
type Category struct {
	ID   int
	Name string
}

// GetCategories returns all issue categories defined for a project.
func (c *Client) GetCategories(projectID int) ([]Category, error) {
	data, err := c.doRequest(fmt.Sprintf("/projects/%d/issue_categories.json", projectID))
	if err != nil {
		return nil, err
	}
	var resp categoryResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	all := make([]Category, 0, len(resp.Categories))
	for _, cat := range resp.Categories {
		all = append(all, Category{ID: cat.ID, Name: cat.Name})
	}
	return all, nil
}

// FindCategoryID resolves an issue category name to its id within a project.
func (c *Client) FindCategoryID(projectID int, name string) (int, error) {
	categories, err := c.GetCategories(projectID)
	if err != nil {
		return 0, err
	}
	for _, cat := range categories {
		if cat.Name == name {
			return cat.ID, nil
		}
	}
	return 0, fmt.Errorf("category %q not found in project %d", name, projectID)
}
