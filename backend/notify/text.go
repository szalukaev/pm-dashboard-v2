package notify

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// Тексты уведомлений для писем. В интерфейсе те же тексты собирает клиент из
// своего словаря; письмо собирается на сервере, поэтому шаблоны лежат и здесь.
// Язык — «ru» или «en», любой другой считается русским.

// titles — заголовок уведомления по правилу; %d — число дней, %s — состояние.
var titles = map[string]map[string]string{
	"ru": {
		RuleTaskAssigned:        "Вам назначена задача",
		RuleTaskChanged:         "Задача изменена",
		RuleTaskDueSoon:         "До срока задачи осталось дней: %d",
		RuleTaskOverdue:         "Задача просрочена, дней: %d",
		RuleContractEnding:      "До окончания договора осталось дней: %d",
		RuleContractExpired:     "Срок договора истёк",
		RuleInvoiceUnpaid:       "Счёт не оплачен, дней: %d",
		RuleSprintEnding:        "До окончания спринта осталось дней: %d",
		RuleSprintOverdue:       "Спринт просрочен",
		RuleProjectOverdue:      "Задача просрочена, дней: %d",
		RuleProjectHighPriority: "Задача с высшим приоритетом",
		RuleProjectTaskClosed:   "Задача закрыта",
		RuleSyncFailed:          "Синхронизация с Redmine завершилась с ошибкой",
		RuleLicenseState:        "Лицензия: %s",
	},
	"en": {
		RuleTaskAssigned:        "A task was assigned to you",
		RuleTaskChanged:         "A task was changed",
		RuleTaskDueSoon:         "Days left until the task is due: %d",
		RuleTaskOverdue:         "The task is overdue, days: %d",
		RuleContractEnding:      "Days left until the contract ends: %d",
		RuleContractExpired:     "The contract has expired",
		RuleInvoiceUnpaid:       "The invoice is unpaid, days: %d",
		RuleSprintEnding:        "Days left until the sprint ends: %d",
		RuleSprintOverdue:       "The sprint is overdue",
		RuleProjectOverdue:      "The task is overdue, days: %d",
		RuleProjectHighPriority: "A task with the highest priority",
		RuleProjectTaskClosed:   "A task was closed",
		RuleSyncFailed:          "The sync with Redmine ended with an error",
		RuleLicenseState:        "License: %s",
	},
}

// words — остальные слова писем.
var words = map[string]map[string]string{
	"ru": {
		"status": "Статус", "priority": "Приоритет", "due_date": "Срок", "estimated_hours": "Оценка, ч",
		"due": "Срок", "issued": "Счёт выставлен", "grace": "льготный период", "read_only": "только чтение",
		"subject": "PM Dashboard: новых уведомлений — %d", "intro": "Новые уведомления по вашим правилам:",
		"footer": "Правила уведомлений настраиваются в дашборде: Настройки → Уведомления.",
	},
	"en": {
		"status": "Status", "priority": "Priority", "due_date": "Due date", "estimated_hours": "Estimate, h",
		"due": "Due", "issued": "Invoice issued", "grace": "grace period", "read_only": "read only",
		"subject": "PM Dashboard: new notifications — %d", "intro": "New notifications by your rules:",
		"footer": "The notification rules are set up in the dashboard: Settings → Notifications.",
	},
}

func language(lang string) string {
	if lang == "en" {
		return "en"
	}
	return "ru"
}

// params — данные уведомления, как они лежат в журнале.
type params struct {
	IssueID int      `json:"issue_id"`
	Subject string   `json:"subject"`
	Project string   `json:"project"`
	Name    string   `json:"name"`
	Due     string   `json:"due"`
	Issued  string   `json:"issued"`
	Days    int      `json:"days"`
	State   string   `json:"state"`
	Changes []Change `json:"changes"`
}

// dateText записывает дату ГГГГ-ММ-ДД как ДД.ММ.ГГГГ; пустую — прочерком.
func dateText(date string) string {
	if len(date) < 10 {
		return "—"
	}
	return date[8:10] + "." + date[5:7] + "." + date[0:4]
}

func orDash(text string) string {
	if text == "" {
		return "—"
	}
	return text
}

// Title возвращает заголовок уведомления на языке lang.
func Title(n Notification, lang string) string {
	lang = language(lang)
	var p params
	_ = json.Unmarshal(n.Params, &p)
	format, known := titles[lang][n.Rule]
	switch {
	case !known:
		return n.Rule
	case n.Rule == RuleLicenseState:
		return fmt.Sprintf(format, words[lang][p.State])
	case strings.Contains(format, "%d"):
		return fmt.Sprintf(format, p.Days)
	}
	return format
}

// Lines возвращает строки под заголовком: о какой задаче, договоре или
// спринте речь и что изменилось.
func Lines(n Notification, lang string) []string {
	lang = language(lang)
	w := words[lang]
	var p params
	_ = json.Unmarshal(n.Params, &p)

	var lines []string
	if p.IssueID != 0 {
		lines = append(lines, strings.TrimSpace(fmt.Sprintf("#%d %s", p.IssueID, p.Subject)))
	}
	for _, change := range p.Changes {
		from, to := orDash(change.From), orDash(change.To)
		if change.Field == "due_date" {
			from, to = dateText(change.From), dateText(change.To)
		}
		lines = append(lines, fmt.Sprintf("%s: %s → %s", w[change.Field], from, to))
	}
	if p.Project != "" {
		lines = append(lines, p.Project)
	}
	if p.Name != "" {
		lines = append(lines, p.Name)
	}
	if p.IssueID == 0 && p.Due != "" {
		lines = append(lines, w["due"]+": "+dateText(p.Due))
	}
	if p.Issued != "" {
		lines = append(lines, w["issued"]+": "+dateText(p.Issued))
	}
	return lines
}

// issueID возвращает номер задачи, о которой уведомление, или 0.
func issueID(n Notification) int {
	var p params
	_ = json.Unmarshal(n.Params, &p)
	return p.IssueID
}

// Digest собирает письмо с уведомлениями: тему и текст в HTML. taskURL даёт
// ссылку на карточку задачи в дашборде; если она пуста или taskURL — nil,
// номер и название задачи остаются текстом.
func Digest(list []Notification, lang string, taskURL func(issueID int) string) (subject, body string) {
	lang = language(lang)
	w := words[lang]
	subject = fmt.Sprintf(w["subject"], len(list))

	var b strings.Builder
	b.WriteString(`<div style="font-family:Arial,Helvetica,sans-serif;font-size:14px;color:#1f2328;max-width:640px">`)
	b.WriteString(`<p style="margin:0 0 16px">` + html.EscapeString(w["intro"]) + `</p>`)
	for _, n := range list {
		b.WriteString(`<div style="margin:0 0 12px;padding:12px 14px;border:1px solid #d8dee4;border-radius:8px">`)
		b.WriteString(`<div style="font-weight:bold">` + html.EscapeString(Title(n, lang)) + `</div>`)
		link := ""
		if id := issueID(n); id != 0 && taskURL != nil {
			link = taskURL(id)
		}
		for i, line := range Lines(n, lang) {
			text := html.EscapeString(line)
			// Первая строка уведомления о задаче — её номер и название:
			// клик открывает карточку задачи в дашборде
			if i == 0 && link != "" {
				text = `<a href="` + html.EscapeString(link) + `" style="color:#0969da;text-decoration:none">` + text + `</a>`
			}
			b.WriteString(`<div style="margin-top:4px;color:#57606a">` + text + `</div>`)
		}
		b.WriteString(`<div style="margin-top:6px;font-size:12px;color:#8c959f">` + n.CreatedAt.Format("02.01.2006 15:04") + `</div>`)
		b.WriteString(`</div>`)
	}
	b.WriteString(`<p style="margin:16px 0 0;font-size:12px;color:#8c959f">` + html.EscapeString(w["footer"]) + `</p></div>`)
	return subject, b.String()
}
