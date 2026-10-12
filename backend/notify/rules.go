// Пакет notify создаёт уведомления пользователей по их правилам: сравнивает
// состояние задач до и после синхронизации, проверяет сроки и кладёт
// уведомления в журнал. Одно событие приходит пользователю один раз.
package notify

import "encoding/json"

// Правила уведомлений. Ключ правила хранится в журнале и в настройках
// пользователя, по нему интерфейс и письма подбирают текст.
const (
	// Мои задачи: «я ответственный» — задача назначена на сотрудника
	// источника, с которым связан пользователь.
	RuleTaskAssigned = "task_assigned" // мне назначена задача
	RuleTaskChanged  = "task_changed"  // в моей задаче изменились статус, приоритет, срок или оценка
	RuleTaskDueSoon  = "task_due_soon" // до срока моей задачи осталось не больше Days дней
	RuleTaskOverdue  = "task_overdue"  // моя задача просрочена — каждый день, пока не закрыта

	// Мои договоры и счета по ним.
	RuleContractEnding  = "contract_ending"  // до окончания договора осталось не больше Days дней
	RuleContractExpired = "contract_expired" // срок договора истёк — один раз
	RuleInvoiceUnpaid   = "invoice_unpaid"   // счёт не оплачен Days дней — один раз

	// Спринты, которые мне видны.
	RuleSprintEnding  = "sprint_ending"  // до окончания спринта осталось не больше Days дней
	RuleSprintOverdue = "sprint_overdue" // спринт просрочен — один раз

	// По проектам, кто бы ни был ответственным; Projects — какие проекты.
	RuleProjectOverdue      = "project_overdue"       // задача просрочена на Days дней — один раз
	RuleProjectHighPriority = "project_high_priority" // появилась задача с высшим приоритетом
	RuleProjectTaskClosed   = "project_task_closed"   // задача закрыта

	// Системные, только администратору.
	RuleSyncFailed   = "sync_failed"   // синхронизация с источником завершилась с ошибкой
	RuleLicenseState = "license_state" // лицензия в льготном периоде или в режиме «только чтение»
)

// Rule — настройка одного правила у пользователя.
type Rule struct {
	// Bell: показывать в колокольчике; Email: включать в письмо.
	Bell  bool `json:"bell"`
	Email bool `json:"email"`
	// Days: порог в днях, смысл зависит от правила.
	Days int `json:"days,omitempty"`
	// Projects: проекты, по которым работает правило; пусто — все.
	Projects []int `json:"projects,omitempty"`
}

// Enabled сообщает, доставляется ли правило хоть куда-нибудь.
func (r Rule) Enabled() bool { return r.Bell || r.Email }

// ForProject сообщает, работает ли правило для проекта.
func (r Rule) ForProject(projectID int) bool {
	if len(r.Projects) == 0 {
		return true
	}
	for _, id := range r.Projects {
		if id == projectID {
			return true
		}
	}
	return false
}

// Rules — правила пользователя по ключам.
type Rules map[string]Rule

// spec описывает правило: настройку по умолчанию и допустимые дни.
type spec struct {
	def Rule
	// hasDays: у правила есть порог в днях, от minDays до maxDays.
	hasDays          bool
	minDays, maxDays int
	// adminOnly: правило срабатывает только у администратора.
	adminOnly bool
}

// specs — все известные правила. Правило, которого здесь нет, не
// сохраняется и не срабатывает. Правила о своём (задачи, договоры, спринты)
// включены сразу; правила по проектам пользователь включает сам.
var specs = map[string]spec{
	RuleTaskAssigned: {def: Rule{Bell: true}},
	RuleTaskChanged:  {def: Rule{Bell: true}},
	RuleTaskDueSoon:  {def: Rule{Bell: true, Days: 3}, hasDays: true, minDays: 0, maxDays: 60},
	RuleTaskOverdue:  {def: Rule{Bell: true}},

	RuleContractEnding:  {def: Rule{Bell: true, Days: 14}, hasDays: true, minDays: 0, maxDays: 180},
	RuleContractExpired: {def: Rule{Bell: true}},
	RuleInvoiceUnpaid:   {def: Rule{Bell: true, Days: 14}, hasDays: true, minDays: 1, maxDays: 180},

	RuleSprintEnding:  {def: Rule{Bell: true, Days: 2}, hasDays: true, minDays: 0, maxDays: 30},
	RuleSprintOverdue: {def: Rule{Bell: true}},

	RuleProjectOverdue:      {def: Rule{Days: 1}, hasDays: true, minDays: 1, maxDays: 365},
	RuleProjectHighPriority: {def: Rule{}},
	RuleProjectTaskClosed:   {def: Rule{}},

	RuleSyncFailed:   {def: Rule{Bell: true}, adminOnly: true},
	RuleLicenseState: {def: Rule{Bell: true}, adminOnly: true},
}

// Keys возвращает ключи известных правил в порядке показа.
func Keys() []string {
	return []string{
		RuleTaskAssigned, RuleTaskChanged, RuleTaskDueSoon, RuleTaskOverdue,
		RuleContractEnding, RuleContractExpired, RuleInvoiceUnpaid,
		RuleSprintEnding, RuleSprintOverdue,
		RuleProjectOverdue, RuleProjectHighPriority, RuleProjectTaskClosed,
		RuleSyncFailed, RuleLicenseState,
	}
}

// AdminOnly сообщает, что правило срабатывает только у администратора.
func AdminOnly(key string) bool { return specs[key].adminOnly }

// Defaults возвращает правила пользователя, который ничего не настраивал.
func Defaults() Rules {
	rules := Rules{}
	for key, s := range specs {
		rules[key] = s.def
	}
	return rules
}

// Effective возвращает действующие правила: сохранённые пользователем поверх
// правил по умолчанию. raw — содержимое user_settings.alert_rules.
func Effective(raw []byte) Rules {
	rules := Defaults()
	saved := Rules{}
	if len(raw) == 0 || json.Unmarshal(raw, &saved) != nil {
		return rules
	}
	for key, rule := range saved {
		s, known := specs[key]
		if !known {
			continue
		}
		if s.hasDays && (rule.Days < s.minDays || rule.Days > s.maxDays) {
			rule.Days = s.def.Days
		}
		if !s.hasDays {
			rule.Days = 0
		}
		rules[key] = rule
	}
	return rules
}

// Valid проверяет правила, присланные интерфейсом: только известные ключи,
// дни в допустимых пределах, проекты — положительные номера.
func Valid(rules Rules) bool {
	for key, rule := range rules {
		s, known := specs[key]
		if !known {
			return false
		}
		if s.hasDays && (rule.Days < s.minDays || rule.Days > s.maxDays) {
			return false
		}
		if !s.hasDays && rule.Days != 0 {
			return false
		}
		if len(rule.Projects) > 500 {
			return false
		}
		for _, id := range rule.Projects {
			if id <= 0 {
				return false
			}
		}
	}
	return true
}
