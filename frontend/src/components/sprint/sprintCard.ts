import type { SprintTask } from '../../stores/sprint'
import type { KanbanCard } from '../../stores/kanban'

// Задача спринта в виде карточки Kanban — карточки в спринтах и бэклоге те же,
// что на доске
export function sprintTaskToCard(task: SprintTask): KanbanCard {
  return {
    external_id: task.external_id,
    subject: task.subject,
    project_name: task.project_name,
    status_name: task.status_name,
    status_id: 0,
    priority_name: task.priority_name,
    priority_id: task.priority_id,
    assigned_to_name: task.assigned_to_name,
    assigned_to_id: null,
    category_name: '',
    due_date: task.due_date,
    estimated_hours: task.estimated_hours,
    is_overdue: task.is_overdue,
  }
}
