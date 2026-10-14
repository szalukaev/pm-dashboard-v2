// Права доступа: структуры, которыми экраны администрирования обмениваются
// с сервером (см. backend/access).

export interface Permissions {
  project_ids: number[]
  team_ids: number[]
  // Всё целиком, списки не учитываются
  all_projects: boolean
  all_team: boolean
  // Видны и задачи без исполнителя
  show_unassigned: boolean
  // Плюс задачи, назначенные самому пользователю
  own_tasks_only: boolean
  // Данные можно смотреть, но не менять
  read_only: boolean
  // Плюс проекты, в которых пользователь состоит в источнике данных
  from_source: boolean
  // null — всё, иначе разрешённые ключи
  visible_tabs: string[] | null
  widgets: string[] | null
  // Разрешённые действия со спринтами; null — все действия
  sprint_actions: string[] | null
  // Доступные виды доски Kanban; null = оба
  kanban_modes: string[] | null
  // Разрешённые действия с оплатами; null — все действия
  payment_actions: string[] | null
  // В «Оплатах» только записи, созданные самим пользователем; null — не задано
  // (в личных правах это «только свои», в группе ограничения нет)
  payments_own_only: boolean | null
}

export interface NamedPermissions {
  id: number
  name: string
  permissions: Permissions
}

// Права пользователя по источникам и их итог
export interface Grants {
  individual: Permissions | null
  template: NamedPermissions | null
  groups: NamedPermissions[]
  effective: Permissions
}

export interface ProjectItem {
  id: number
  name: string
  parent_id: number | null
}

export interface MemberItem {
  id: number
  name: string
}

// Ключи вкладок и виджетов аналитики, которые можно скрыть; подписи —
// navigation.<вкладка> и access.widgets.<виджет> в словаре.
export const TABS = ['tasks', 'analytics', 'kanban', 'sprint', 'payments']
export const WIDGETS = ['stats', 'health', 'velocity', 'team_load', 'hours', 'bug_hours', 'distribution', 'deadlines', 'gantt']
// Действия со спринтами, каждое разрешается отдельно; подписи — access.sprint.<действие>
export const SPRINT_ACTIONS = ['create', 'edit', 'tasks', 'close', 'delete']
// Виды доски Kanban: users — по сотрудникам, statuses — по проектам (статусы
// одного проекта); подписи — access.kanban.<вид>
export const KANBAN_MODES = ['users', 'statuses']
// Действия с оплатами по сущностям; подписи — access.payment.<действие>
export const PAYMENT_ACTIONS = [
  'org_create', 'org_edit', 'org_delete',
  'contract_create', 'contract_edit', 'contract_delete',
  'invoice_create', 'invoice_delete', 'invoice_pay', 'download',
]

export function emptyPermissions(): Permissions {
  return {
    project_ids: [],
    team_ids: [],
    all_projects: false,
    all_team: false,
    show_unassigned: false,
    own_tasks_only: false,
    read_only: false,
    from_source: false,
    visible_tabs: null,
    widgets: null,
    sprint_actions: null,
    kanban_modes: null,
    payment_actions: null,
    payments_own_only: null,
  }
}

export function clonePermissions(p: Permissions): Permissions {
  return {
    ...p,
    project_ids: [...(p.project_ids || [])],
    team_ids: [...(p.team_ids || [])],
    visible_tabs: p.visible_tabs ? [...p.visible_tabs] : null,
    widgets: p.widgets ? [...p.widgets] : null,
    sprint_actions: p.sprint_actions ? [...p.sprint_actions] : null,
    kanban_modes: p.kanban_modes ? [...p.kanban_modes] : null,
    payment_actions: p.payment_actions ? [...p.payment_actions] : null,
  }
}

export interface TreeItem {
  id: number
  name: string
  depth: number
  hasChildren: boolean
}

// Проекты плоским списком в порядке дерева с глубиной вложенности. Проект,
// родителя которого нет в списке, начинает свою ветку.
export function projectTree(projects: ProjectItem[]): TreeItem[] {
  const ids = new Set(projects.map(p => p.id))
  const children = new Map<number, ProjectItem[]>()
  for (const p of projects) {
    const parent = p.parent_id && ids.has(p.parent_id) ? p.parent_id : 0
    if (!children.has(parent)) children.set(parent, [])
    children.get(parent)!.push(p)
  }
  const result: TreeItem[] = []
  const walk = (parent: number, depth: number) => {
    for (const p of children.get(parent) || []) {
      result.push({ id: p.id, name: p.name, depth, hasChildren: children.has(p.id) })
      walk(p.id, depth + 1)
    }
  }
  walk(0, 0)
  return result
}

// Проект со всеми его подпроектами
export function withDescendants(projects: ProjectItem[], id: number): number[] {
  const result = [id]
  for (let i = 0; i < result.length; i++) {
    for (const p of projects) {
      if (p.parent_id === result[i] && !result.includes(p.id)) result.push(p.id)
    }
  }
  return result
}

// Подпроекты, покрытые заданными проектами: право на проект — это право на
// всю его ветку, поэтому своя галочка им не нужна.
export function coveredByParents(projects: ProjectItem[], selected: number[]): Set<number> {
  const covered = new Set<number>()
  for (const id of selected) {
    for (const child of withDescendants(projects, id)) {
      if (child !== id) covered.add(child)
    }
  }
  return covered
}
