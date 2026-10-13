package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"pm-dashboard/access"
	"pm-dashboard/mail"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

// Как часто уходит сводка.
const (
	ReportDaily   = "daily"   // каждый день
	ReportWeekly  = "weekly"  // по понедельникам
	ReportMonthly = "monthly" // 1-го числа
)

// reportSections — разделы сводки в порядке письма.
var reportSections = []string{"stats", "team_load", "distribution", "deadlines"}

// reportListLimit — сколько задач показывает один список письма.
const reportListLimit = 30

// ReportSettings — настройка регулярной сводки пользователя.
type ReportSettings struct {
	Enabled bool `json:"enabled"`
	// Email: куда слать; пусто — на адрес из профиля.
	Email  string `json:"email"`
	Period string `json:"period"`
	// Time — время отправки по часам сервера, ЧЧ:ММ.
	Time     string   `json:"time"`
	Sections []string `json:"sections"`
	// OverdueList: добавить в письмо список просроченных задач.
	OverdueList bool `json:"overdue_list"`
}

// defaultReportSettings — сводка, которую ещё не настраивали: выключена,
// раз в неделю в 09:00, со всеми разделами.
func defaultReportSettings() ReportSettings {
	return ReportSettings{Period: ReportWeekly, Time: "09:00", Sections: append([]string{}, reportSections...), OverdueList: true}
}

// parseReportSettings читает сохранённую настройку поверх стандартной.
func parseReportSettings(raw []byte) ReportSettings {
	settings := defaultReportSettings()
	if len(raw) > 0 {
		saved := settings
		if json.Unmarshal(raw, &saved) == nil && saved.valid() {
			settings = saved
		}
	}
	if settings.Sections == nil {
		settings.Sections = []string{}
	}
	return settings
}

// valid проверяет настройку, присланную интерфейсом.
func (s ReportSettings) valid() bool {
	if s.Period != ReportDaily && s.Period != ReportWeekly && s.Period != ReportMonthly {
		return false
	}
	if _, err := time.Parse("15:04", s.Time); err != nil {
		return false
	}
	if s.Email != "" && !mail.ValidAddress(s.Email) {
		return false
	}
	if len(s.Sections) > len(reportSections) {
		return false
	}
	for _, section := range s.Sections {
		if !s.knownSection(section) {
			return false
		}
	}
	return true
}

func (s ReportSettings) knownSection(section string) bool {
	for _, known := range reportSections {
		if known == section {
			return true
		}
	}
	return false
}

func (s ReportSettings) has(section string) bool {
	for _, chosen := range s.Sections {
		if chosen == section {
			return true
		}
	}
	return false
}

// reportMoment возвращает последний момент не позже now, когда сводка должна
// была уйти по расписанию: сегодня или вчера в заданное время, последний
// понедельник, последнее 1-е число.
func reportMoment(period, timeOfDay string, now time.Time) time.Time {
	t, err := time.Parse("15:04", timeOfDay)
	if err != nil {
		t, _ = time.Parse("15:04", "09:00")
	}
	at := func(day time.Time) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	}
	switch period {
	case ReportWeekly:
		// time.Weekday считает с воскресенья; неделя здесь начинается с понедельника
		monday := now.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
		if moment := at(monday); !moment.After(now) {
			return moment
		}
		return at(monday.AddDate(0, 0, -7))
	case ReportMonthly:
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		if moment := at(first); !moment.After(now) {
			return moment
		}
		return at(first.AddDate(0, -1, 0))
	}
	if moment := at(now); !moment.After(now) {
		return moment
	}
	return at(now.AddDate(0, 0, -1))
}

// reportDue сообщает, пора ли отправлять сводку: с прошлой отправки (или с
// момента включения) наступил очередной срок по расписанию.
func reportDue(s ReportSettings, lastSent *time.Time, now time.Time) bool {
	return s.Enabled && lastSent != nil && lastSent.Before(reportMoment(s.Period, s.Time, now))
}

// ReportHandler отправляет регулярные сводки и пробное письмо.
type ReportHandler struct {
	DB     **sql.DB
	Mail   *mail.Store
	Scopes func(userID int) (*access.Scope, error)
	// Send отправляет одно письмо; по умолчанию mail.Send.
	Send func(cfg mail.Config, to, subject, html string) error
}

func (h *ReportHandler) send(cfg mail.Config, to, subject, body string) error {
	if h.Send != nil {
		return h.Send(cfg, to, subject, body)
	}
	return mail.Send(cfg, to, subject, body)
}

// SendDue отправляет сводки тем, у кого наступил срок, и возвращает число
// отправленных писем и первую ошибку. Вызывается рассылкой.
func (h *ReportHandler) SendDue(cfg mail.Config, now time.Time) (int, error) {
	if h.DB == nil || *h.DB == nil {
		return 0, nil
	}
	rows, err := (*h.DB).Query(`SELECT u.id, u.email, COALESCE(us.language, 'ru'), us.email_report, us.email_report_sent_at
		FROM users u JOIN user_settings us ON us.user_id = u.id
		WHERE NOT COALESCE(u.is_blocked, false) AND us.email_report->>'enabled' = 'true'`)
	if err != nil {
		return 0, err
	}
	type target struct {
		userID      int
		email, lang string
		settings    ReportSettings
	}
	var targets []target
	for rows.Next() {
		var t target
		var raw []byte
		var lastSent *time.Time
		if rows.Scan(&t.userID, &t.email, &t.lang, &raw, &lastSent) != nil {
			continue
		}
		t.settings = parseReportSettings(raw)
		if t.settings.Email != "" {
			t.email = t.settings.Email
		}
		if t.email != "" && reportDue(t.settings, lastSent, now) {
			targets = append(targets, t)
		}
	}
	rows.Close()

	sent := 0
	var firstErr error
	for _, t := range targets {
		if err := h.sendTo(cfg, t.userID, t.email, t.lang, t.settings, now); err != nil {
			slog.Warn("Could not send the regular report", "user", t.userID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		(*h.DB).Exec("UPDATE user_settings SET email_report_sent_at = $1 WHERE user_id = $2", now, t.userID)
		sent++
	}
	return sent, firstErr
}

func (h *ReportHandler) sendTo(cfg mail.Config, userID int, to, lang string, settings ReportSettings, now time.Time) error {
	scope, err := h.Scopes(userID)
	if err != nil {
		return err
	}
	subject, body, err := h.build(scope, settings, lang, now, cfg.TaskURL)
	if err != nil {
		return err
	}
	return h.send(cfg, to, subject, body)
}

// SendTest отправляет пользователю сводку прямо сейчас — проверить, как она
// выглядит. Настройка берётся из запроса, чтобы проверить можно было до
// сохранения.
func (h *ReportHandler) SendTest(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	settings := defaultReportSettings()
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil || !settings.valid() {
		utils.Error(w, http.StatusBadRequest, "INVALID_EMAIL_REPORT")
		return
	}
	userID := middleware.GetUserID(r)
	var email, lang string
	(*h.DB).QueryRow(`SELECT u.email, COALESCE(us.language, 'ru') FROM users u
		LEFT JOIN user_settings us ON us.user_id = u.id WHERE u.id = $1`, userID).Scan(&email, &lang)
	if settings.Email != "" {
		email = settings.Email
	}
	if email == "" {
		utils.Error(w, http.StatusBadRequest, "EMAIL_REQUIRED")
		return
	}
	cfg, err := h.Mail.Load()
	if err != nil || !cfg.Ready() {
		utils.Error(w, http.StatusBadRequest, "SMTP_NOT_CONFIGURED")
		return
	}
	subject, body, err := h.build(middleware.GetScope(r), settings, lang, time.Now(), cfg.TaskURL)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	if err := h.send(cfg, email, subject, body); err != nil {
		slog.Warn("Test report failed", "user", userID, "error", err)
		utils.Error(w, http.StatusBadGateway, "MAIL_SEND_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "to": email})
}

// reportWords — слова письма на двух языках.
var reportWords = map[string]map[string]string{
	"ru": {
		"subject": "PM Dashboard: сводка по задачам на %s", "title": "Сводка по задачам", "on": "на %s",
		"stats": "Статистика", "team_load": "Нагрузка на команду", "distribution": "Распределение открытых задач по проектам",
		"deadlines": "Ближайшие дедлайны", "overdue_list": "Просроченные задачи",
		"active_projects": "Активных проектов", "completion": "Выполнение", "open": "Открытых", "in_test": "В тесте",
		"overdue": "Просрочено", "bugs": "Bugs", "no_estimate": "Без оценки", "closed": "Закрытых", "high": "Высокий",
		"employee": "Сотрудник", "project": "Проект", "tasks": "Задач", "task": "Задача", "assignee": "Ответственный",
		"due": "Срок", "days_late": "Дней просрочки", "today": "Сегодня", "three_days": "3 дня", "week": "Неделя",
		"none": "Нет задач", "more": "…и ещё %d", "unassigned": "Не назначен",
		"footer": "Сводка настраивается в дашборде: Настройки → Уведомления → Email-рассылка.",
	},
	"en": {
		"subject": "PM Dashboard: task report for %s", "title": "Task report", "on": "as of %s",
		"stats": "Statistics", "team_load": "Team workload", "distribution": "Open tasks by project",
		"deadlines": "Upcoming deadlines", "overdue_list": "Overdue tasks",
		"active_projects": "Active projects", "completion": "Completion", "open": "Open", "in_test": "In test",
		"overdue": "Overdue", "bugs": "Bugs", "no_estimate": "No estimate", "closed": "Closed", "high": "High",
		"employee": "Employee", "project": "Project", "tasks": "Tasks", "task": "Task", "assignee": "Assignee",
		"due": "Due", "days_late": "Days overdue", "today": "Today", "three_days": "3 days", "week": "Week",
		"none": "No tasks", "more": "…and %d more", "unassigned": "Unassigned",
		"footer": "The report is set up in the dashboard: Settings → Notifications → Email report.",
	},
}

// Оформление письма: почтовые программы понимают только стили в самих тегах.
const (
	reportCell   = `padding:6px 10px;border-bottom:1px solid #e5e7eb;font-size:13px;`
	reportHead   = `padding:6px 10px;border-bottom:2px solid #d0d7de;font-size:12px;color:#57606a;text-align:left;`
	reportNumber = `text-align:right;white-space:nowrap;`
)

// reportBar рисует цветную полосу длиной value от max — вместо диаграммы.
func reportBar(value, max int, color string) string {
	width := 0
	if max > 0 {
		width = int(math.Round(float64(value) / float64(max) * 100))
	}
	if value > 0 && width < 2 {
		width = 2
	}
	return fmt.Sprintf(`<div style="background:#eef1f4;border-radius:4px;height:10px;width:160px">`+
		`<div style="background:%s;border-radius:4px;height:10px;width:%d%%"></div></div>`, color, width)
}

func reportDate(date string) string {
	if len(date) < 10 {
		return "—"
	}
	return date[8:10] + "." + date[5:7] + "." + date[0:4]
}

// build собирает письмо со сводкой: тему и текст в HTML. Данные считаются
// теми же расчётами, что показывает вкладка «Аналитика», в правах пользователя.
// taskURL даёт ссылку на карточку задачи в дашборде; с пустой ссылкой задача
// остаётся текстом.
func (h *ReportHandler) build(scope *access.Scope, settings ReportSettings, lang string, now time.Time, taskURL func(issueID int) string) (subject, body string, err error) {
	if lang != "en" {
		lang = "ru"
	}
	w := reportWords[lang]
	analytics := &AnalyticsHandler{DB: h.DB}
	date := now.Format("02.01.2006")
	esc := html.EscapeString

	var b strings.Builder
	b.WriteString(`<div style="font-family:Arial,Helvetica,sans-serif;font-size:14px;color:#1f2328;max-width:760px">`)
	b.WriteString(`<h2 style="margin:0 0 4px;font-size:18px">` + esc(w["title"]) + `</h2>`)
	b.WriteString(`<div style="margin:0 0 20px;font-size:12px;color:#8c959f">` + esc(fmt.Sprintf(w["on"], date)) + `</div>`)
	section := func(key string) {
		b.WriteString(`<h3 style="margin:24px 0 8px;font-size:15px">` + esc(w[key]) + `</h3>`)
	}
	table := func(heads ...string) {
		b.WriteString(`<table cellspacing="0" cellpadding="0" style="border-collapse:collapse;width:100%"><tr>`)
		for _, head := range heads {
			b.WriteString(`<th style="` + reportHead + `">` + esc(head) + `</th>`)
		}
		b.WriteString(`</tr>`)
	}
	cell := func(text string, style string) {
		b.WriteString(`<td style="` + reportCell + style + `">` + text + `</td>`)
	}
	none := func() {
		b.WriteString(`<div style="font-size:13px;color:#8c959f">` + esc(w["none"]) + `</div>`)
	}

	if settings.has("stats") {
		section("stats")
		table()
		for _, stat := range analytics.stats(scope, now) {
			value := fmt.Sprintf("%d", int(math.Round(stat.Value)))
			if stat.IsPct {
				value = fmt.Sprintf("%.1f%%", stat.Value)
			}
			b.WriteString(`<tr>`)
			cell(esc(w[stat.Label]), "")
			cell(`<b>`+value+`</b>`, reportNumber)
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</table>`)
	}

	if settings.has("team_load") {
		team, err := analytics.teamLoad(scope, now)
		if err != nil {
			return "", "", err
		}
		section("team_load")
		if len(team) == 0 {
			none()
		} else {
			max := 0
			for _, row := range team {
				if row.Open > max {
					max = row.Open
				}
			}
			table(w["employee"], w["open"], "", w["in_test"], w["closed"], w["overdue"], w["bugs"], w["high"], w["no_estimate"])
			for _, row := range team {
				b.WriteString(`<tr>`)
				cell(esc(row.Name), "")
				cell(fmt.Sprint(row.Open), reportNumber)
				cell(reportBar(row.Open, max, "#5e6ad2"), "")
				for _, n := range []int{row.Testing, row.Closed, row.Overdue, row.Bugs, row.HighPrio, row.NoEstimate} {
					cell(fmt.Sprint(n), reportNumber)
				}
				b.WriteString(`</tr>`)
			}
			b.WriteString(`</table>`)
		}
	}

	if settings.has("distribution") {
		dist, err := analytics.distribution(scope)
		if err != nil {
			return "", "", err
		}
		section("distribution")
		if len(dist) == 0 {
			none()
		} else {
			totals := make([]int, len(dist))
			max := 0
			for i, project := range dist {
				for _, a := range project.Assignees {
					totals[i] += a.Count
				}
				if totals[i] > max {
					max = totals[i]
				}
			}
			table(w["project"], w["tasks"], "", w["employee"])
			for i, project := range dist {
				var people []string
				for _, a := range project.Assignees {
					name := a.Name
					if strings.TrimSpace(name) == "" {
						name = w["unassigned"]
					}
					people = append(people, fmt.Sprintf("%s — %d", esc(name), a.Count))
				}
				b.WriteString(`<tr>`)
				cell(esc(project.ProjectName), "")
				cell(fmt.Sprint(totals[i]), reportNumber)
				cell(reportBar(totals[i], max, "#27a644"), "")
				cell(strings.Join(people, ", "), "color:#57606a;")
				b.WriteString(`</tr>`)
			}
			b.WriteString(`</table>`)
		}
	}

	if settings.has("deadlines") || settings.OverdueList {
		groups, err := analytics.deadlines(scope, now)
		if err != nil {
			return "", "", err
		}
		today := now.Format("2006-01-02")
		tasks := func(group DeadlineGroup, late bool) {
			if len(group.Tasks) == 0 {
				none()
				return
			}
			heads := []string{w["task"], w["project"], w["assignee"], w["due"]}
			if late {
				heads = append(heads, w["days_late"])
			}
			table(heads...)
			for i, task := range group.Tasks {
				if i == reportListLimit {
					break
				}
				due := ""
				if task.DueDate != nil {
					due = *task.DueDate
				}
				if len(due) > 10 {
					due = due[:10]
				}
				b.WriteString(`<tr>`)
				// Номер и название задачи — ссылка на её карточку в дашборде
				name := fmt.Sprintf("#%d %s", task.ExternalID, esc(task.Subject))
				if link := taskURL(task.ExternalID); link != "" {
					name = `<a href="` + esc(link) + `" style="color:#0969da;text-decoration:none">` + name + `</a>`
				}
				cell(name, "")
				cell(esc(task.ProjectName), "color:#57606a;")
				cell(esc(task.AssignedToName), "color:#57606a;")
				cell(reportDate(due), reportNumber)
				if late {
					days := 0
					if a, errA := time.Parse("2006-01-02", due); errA == nil {
						if t, errT := time.Parse("2006-01-02", today); errT == nil {
							days = int(math.Round(t.Sub(a).Hours() / 24))
						}
					}
					cell(fmt.Sprint(days), reportNumber+"color:#d1242f;")
				}
				b.WriteString(`</tr>`)
			}
			b.WriteString(`</table>`)
			if extra := len(group.Tasks) - reportListLimit; extra > 0 {
				b.WriteString(`<div style="margin-top:6px;font-size:12px;color:#8c959f">` + esc(fmt.Sprintf(w["more"], extra)) + `</div>`)
			}
		}
		for _, group := range groups {
			switch {
			// Просроченные — часть дедлайнов: они в письме, если выбран
			// любой из двух разделов
			case group.Name == "overdue":
				section("overdue_list")
				tasks(group, true)
			case group.Name != "overdue" && settings.has("deadlines"):
				b.WriteString(`<h3 style="margin:24px 0 8px;font-size:15px">` + esc(w["deadlines"]+": "+w[group.Name]) + `</h3>`)
				tasks(group, false)
			}
		}
	}

	b.WriteString(`<p style="margin:24px 0 0;font-size:12px;color:#8c959f">` + esc(w["footer"]) + `</p></div>`)
	return fmt.Sprintf(w["subject"], date), b.String(), nil
}
