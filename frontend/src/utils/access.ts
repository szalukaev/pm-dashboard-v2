// Access control: the shapes the administration screens exchange with the
// server (see backend/access).

export interface Permissions {
  project_ids: number[]
  team_ids: number[]
  // Everything, the lists are ignored
  all_projects: boolean
  all_team: boolean
  // Issues without an assignee are visible too
  show_unassigned: boolean
  // Plus the issues assigned to the user themselves
  own_tasks_only: boolean
  // Data may be viewed but not changed
  read_only: boolean
  // Plus the projects the user is a member of in the data source
  from_source: boolean
  // null = everything, otherwise the allowed keys
  visible_tabs: string[] | null
  widgets: string[] | null
}

export interface NamedPermissions {
  id: number
  name: string
  permissions: Permissions
}

// Rights of a user by source and what they add up to
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

// Keys of the tabs and analytics widgets that can be hidden; labels are
// navigation.<tab> and access.widgets.<widget> in the dictionary.
export const TABS = ['tasks', 'analytics', 'kanban', 'sprint', 'payments']
export const WIDGETS = ['stats', 'team_load', 'distribution', 'deadlines']

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
  }
}

export function clonePermissions(p: Permissions): Permissions {
  return {
    ...p,
    project_ids: [...(p.project_ids || [])],
    team_ids: [...(p.team_ids || [])],
    visible_tabs: p.visible_tabs ? [...p.visible_tabs] : null,
    widgets: p.widgets ? [...p.widgets] : null,
  }
}

export interface TreeItem {
  id: number
  name: string
  depth: number
  hasChildren: boolean
}

// Projects as a flat list in tree order with the nesting depth. A project
// whose parent is not in the list starts its own branch.
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

// A project with all its subprojects
export function withDescendants(projects: ProjectItem[], id: number): number[] {
  const result = [id]
  for (let i = 0; i < result.length; i++) {
    for (const p of projects) {
      if (p.parent_id === result[i] && !result.includes(p.id)) result.push(p.id)
    }
  }
  return result
}

// The subprojects covered by the given projects: a right to a project is a
// right to its whole branch, so these need no tick of their own.
export function coveredByParents(projects: ProjectItem[], selected: number[]): Set<number> {
  const covered = new Set<number>()
  for (const id of selected) {
    for (const child of withDescendants(projects, id)) {
      if (child !== id) covered.add(child)
    }
  }
  return covered
}
