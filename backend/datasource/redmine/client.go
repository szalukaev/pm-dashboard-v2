package redmine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// pageRetryDelay — пауза перед повтором неудавшейся страницы (растёт с каждой попыткой).
var pageRetryDelay = time.Second

type Client struct {
	BaseURL    string
	APIKey     string
	BasicLogin string
	BasicPass  string
	HTTPClient *http.Client

	// ctx, если задан, ограничивает запросы списков этого клиента:
	// остановленная синхронизация бросает то, чего ждёт.
	ctx context.Context
}

// withContext возвращает копию клиента, запросы которой обрываются вместе с ctx.
func (c *Client) withContext(ctx context.Context) *Client {
	bound := *c
	bound.ctx = ctx
	return &bound
}

func (c *Client) requestContext() context.Context {
	if c.ctx != nil {
		return c.ctx
	}
	return context.Background()
}

type projectResp struct {
	Projects []struct {
		ID     int               `json:"id"`
		Name   string            `json:"name"`
		Parent *struct{ ID int } `json:"parent"`
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
		StartDate      *string  `json:"start_date"`
		DueDate        *string  `json:"due_date"`
		DoneRatio      int      `json:"done_ratio"`
		EstimatedHours *float64 `json:"estimated_hours"`
		SpentHours     float64  `json:"spent_hours"`
		Tracker        struct {
			Name string `json:"name"`
		} `json:"tracker"`
		Author struct {
			Name string `json:"name"`
		} `json:"author"`
		FixedVersion *struct {
			Name string `json:"name"`
		} `json:"fixed_version"`
		CreatedOn    *string            `json:"created_on"`
		UpdatedOn    *string            `json:"updated_on"`
		ClosedOn     *string            `json:"closed_on"`
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

// issueEstimate берёт оценку разработчика, если она задана и больше нуля,
// иначе — стандартное поле estimated_hours.
func issueEstimate(estimated *float64, fields []customFieldValue) *float64 {
	if v := developerEstimate(fields); v != nil {
		return v
	}
	return estimated
}

// developerEstimate возвращает оценку разработчика, если она задана и больше нуля.
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

// EstimateFields возвращает поля Redmine, в которых хранится оценка задачи:
// оценку разработчика, если она уже задана, иначе стандартное
// estimated_hours. Значение hours, равное nil, очищает оценку.
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
		Project struct {
			ID int `json:"id"`
		} `json:"project"`
		Activity struct {
			Name string `json:"name"`
		} `json:"activity"`
		Hours float64 `json:"hours"`
	} `json:"time_entries"`
	TotalCount int `json:"total_count"`
}

// TimeEntry — одна запись трудозатрат, привязанная к задаче.
type TimeEntry struct {
	ID           int
	IssueID      int
	ProjectID    int
	ActivityName string
	Hours        float64
}

// GetTimeEntries возвращает трудозатраты проекта, привязанные к задачам, за
// дни начиная с `from` (YYYY-MM-DD; пусто — все). Подпроекты не входят: они
// запрашиваются отдельно.
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
				ProjectID:    e.Project.ID,
				ActivityName: e.Activity.Name,
				Hours:        e.Hours,
			})
		}
	}
	return all, nil
}

type statusResp struct {
	Statuses []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		IsClosed bool   `json:"is_closed"`
	} `json:"issue_statuses"`
}

type memberResp struct {
	Memberships []struct {
		User struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
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
	req, err := http.NewRequestWithContext(c.requestContext(), "GET", url, nil)
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

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w (redmine returned 401 for %s)", ErrUnauthorized, path)
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w (redmine returned 403 for %s)", ErrForbidden, path)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("redmine returned %d for %s", resp.StatusCode, path)
	}
	return io.ReadAll(resp.Body)
}

// ErrForbidden: у владельца ключа API нет права на запрошенное. В отличие
// от сбоя это само не пройдёт, поэтому запрос не повторяется.
var ErrForbidden = errors.New("redmine denies access")

// ErrUnauthorized: Redmine не принял ключ API.
var ErrUnauthorized = errors.New("redmine api key is not valid")

// GetCurrentProjects возвращает проекты, в которых состоит владелец ключа
// API этого клиента, — то, на что у него есть права в Redmine.
func (c *Client) GetCurrentProjects() ([]int, error) {
	data, err := c.doRequest("/users/current.json?include=memberships")
	if err != nil {
		return nil, err
	}
	var resp struct {
		User struct {
			Memberships []struct {
				Project struct {
					ID int `json:"id"`
				} `json:"project"`
			} `json:"memberships"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	projects := make([]int, 0, len(resp.User.Memberships))
	for _, m := range resp.User.Memberships {
		if m.Project.ID > 0 {
			projects = append(projects, m.Project.ID)
		}
	}
	return projects, nil
}

// GetCurrentUser возвращает пользователя Redmine, которому принадлежит ключ
// API этого клиента. Личный ключ определяет своего владельца — так
// пользователь дашборда привязывается к своей учётной записи Redmine.
func (c *Client) GetCurrentUser() (*Member, error) {
	data, err := c.doRequest("/users/current.json")
	if err != nil {
		return nil, err
	}
	var resp struct {
		User struct {
			ID        int    `json:"id"`
			Login     string `json:"login"`
			FirstName string `json:"firstname"`
			LastName  string `json:"lastname"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.User.ID == 0 {
		return nil, fmt.Errorf("redmine returned no current user")
	}
	return &Member{
		ExternalID: resp.User.ID,
		Name:       strings.TrimSpace(resp.User.FirstName + " " + resp.User.LastName),
		Login:      resp.User.Login,
	}, nil
}

// GetCurrentUserMail возвращает адрес электронной почты владельца ключа API
// этого клиента — пусто, если в источнике он не задан.
func (c *Client) GetCurrentUserMail() (string, error) {
	data, err := c.doRequest("/users/current.json")
	if err != nil {
		return "", err
	}
	var resp struct {
		User struct {
			Mail string `json:"mail"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.User.Mail), nil
}

const (
	pageSize = 100
	// Сколько страниц одного списка запрашивается одновременно.
	pageWorkers  = 2
	pageAttempts = 3
	// maxParallelLists ограничивает число одновременных запросов списков по
	// всем спискам и проектам: на много запросов сразу Redmine отвечает 500.
	maxParallelLists = 4
)

// listSlots обеспечивает ограничение maxParallelLists.
var listSlots = make(chan struct{}, maxParallelLists)

// getPages загружает все страницы постраничного списка. Первая страница
// даёт total_count, остальные запрашиваются параллельно и возвращаются по
// порядку. Неудавшаяся страница повторяется несколько раз, и только потом
// ошибкой считается весь список.
func (c *Client) getPages(path string) ([][]byte, error) {
	page := func(offset int) ([]byte, error) {
		var data []byte
		var err error
		for attempt := 1; attempt <= pageAttempts; attempt++ {
			listSlots <- struct{}{}
			data, err = c.doRequest(fmt.Sprintf("%s&limit=%d&offset=%d", path, pageSize, offset))
			<-listSlots
			if err == nil {
				return data, nil
			}
			// Отказ — это ответ, а не сбой: повтор ничего не изменит. Как
			// и в случае, когда синхронизацию остановили.
			if errors.Is(err, ErrForbidden) || errors.Is(err, ErrUnauthorized) || c.requestContext().Err() != nil {
				return nil, err
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

// GetIssues возвращает задачи проекта без подпроектов (они запрашиваются
// отдельно); projectID 0 — все проекты. Заданный updatedSince оставляет
// только задачи, изменённые с этого момента.
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
				CreatedOn:      i.CreatedOn,
				UpdatedOn:      i.UpdatedOn,
				ClosedOn:       i.ClosedOn,
			}
			if i.FixedVersion != nil {
				iss.FixedVersionName = i.FixedVersion.Name
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
			ID   int `json:"id"`
			User struct {
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
		// При ошибке проверки Redmine объясняет причину: {"errors": [...]}
		var failure struct {
			Errors []string `json:"errors"`
		}
		if data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10)); json.Unmarshal(data, &failure) == nil && len(failure.Errors) > 0 {
			return fmt.Errorf("redmine update returned %d: %s", resp.StatusCode, strings.Join(failure.Errors, "; "))
		}
		return fmt.Errorf("redmine update returned %d", resp.StatusCode)
	}
	return nil
}

// IssueState — та часть задачи, которую меняет перенос на доске Kanban.
type IssueState struct {
	StatusID     int
	AssignedToID *int // nil — никто
}

// GetIssueState читает текущие статус и исполнителя задачи. Redmine молча
// пропускает изменение, которое не разрешает его workflow (запрос при этом
// успешен), поэтому перенос проверяется повторным чтением задачи.
func (c *Client) GetIssueState(id int) (*IssueState, error) {
	data, err := c.doRequest(fmt.Sprintf("/issues/%d.json", id))
	if err != nil {
		return nil, err
	}
	var resp struct {
		Issue struct {
			Status struct {
				ID int `json:"id"`
			} `json:"status"`
			AssignedTo *struct {
				ID int `json:"id"`
			} `json:"assigned_to"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	state := &IssueState{StatusID: resp.Issue.Status.ID}
	if resp.Issue.AssignedTo != nil {
		state.AssignedToID = &resp.Issue.AssignedTo.ID
	}
	return state, nil
}

type priorityResp struct {
	Priorities []struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		IsDefault bool   `json:"is_default"`
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

// GetUsers возвращает всех активных пользователей Redmine.
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

// bytesReader оборачивает срез байт в io.ReadCloser для тела запроса.
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

// Category — категория задач проекта Redmine.
type Category struct {
	ID   int
	Name string
}

// GetCategories возвращает все категории задач проекта.
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

// FindCategoryID находит идентификатор категории задач проекта по её названию.
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
