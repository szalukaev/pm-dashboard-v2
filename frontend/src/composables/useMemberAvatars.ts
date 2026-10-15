import { ref } from 'vue'
import axios from 'axios'

// Фото сотрудников, у которых есть пользователь системы с аватаром.
// Загружаются один раз на всю страницу; у кого фото нет — рисуем инициалы.

interface MemberAvatar {
  external_id: number
  name: string
  avatar: string
}

const byId = ref(new Map<number, string>())
const byName = ref(new Map<string, string>())
let requested = false

function load() {
  if (requested) return
  requested = true
  axios.get<MemberAvatar[]>('/api/member-avatars').then(({ data }) => {
    byId.value = new Map(data.map(a => [a.external_id, a.avatar]))
    byName.value = new Map(data.map(a => [a.name, a.avatar]))
  }).catch(() => { requested = false })
}

// Фото по ID сотрудника в источнике, а если ID нет (шапка группы) — по имени
function avatarSrc(id?: number | null, name?: string | null): string {
  if (id != null) {
    const src = byId.value.get(id)
    if (src) return src
  }
  return name ? byName.value.get(name) ?? '' : ''
}

export function useMemberAvatars() {
  load()
  return { avatarSrc }
}
