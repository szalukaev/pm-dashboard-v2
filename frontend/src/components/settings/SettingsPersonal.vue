<template>
  <div class="settings-personal">
    <h3 class="section-title">Личные данные</h3>
    <div class="form-group">
      <div class="info-row">
        <span class="info-label">Имя пользователя</span>
        <span class="info-value">{{ user?.username || '—' }}</span>
      </div>
      <div class="info-row">
        <span class="info-label">Роль</span>
        <span class="info-value">
          <span class="role-badge" :class="user?.role">{{ user?.role === 'admin' ? 'Администратор' : 'Пользователь' }}</span>
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from '../../stores/auth'
import { storeToRefs } from 'pinia'

const { user } = storeToRefs(useAuthStore())
</script>

<style scoped>
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 20px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.info-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.info-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
  min-width: 160px;
}

.info-value {
  font-size: 13px;
  color: var(--text);
}

.role-badge {
  display: inline-block;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 9999px;
}

.role-badge.admin {
  background: var(--accent-bg);
  color: var(--accent);
}

.role-badge.user {
  background: var(--tag-bg);
  color: var(--text-muted);
}
</style>
