<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LogConsole from '@/components/LogConsole.vue'
import { useTasksStore } from '@/stores/tasks'
import { taskPhaseLabel } from '@/utils/status'

const { t } = useI18n()
const tasksStore = useTasksStore()
</script>

<template>
  <div class="view-shell view-shell--logs">
    <section class="stats-grid">
      <article class="stat-card">
        <span class="stat-label">{{ t('logs.scanTask') }}</span>
        <strong>{{ taskPhaseLabel(tasksStore.scan.phase) }}</strong>
        <small>{{ tasksStore.scan.message || t('common.idle') }}</small>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ t('logs.maintainTask') }}</span>
        <strong>{{ taskPhaseLabel(tasksStore.maintain.phase) }}</strong>
        <small>{{ tasksStore.maintain.message || t('common.idle') }}</small>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ t('logs.inventoryTask') }}</span>
        <strong>{{ taskPhaseLabel(tasksStore.inventory.phase) }}</strong>
        <small>{{ tasksStore.inventory.message || t('common.idle') }}</small>
      </article>
    </section>
    <section class="panel panel--fill">
      <div class="panel-head">
        <div>
          <p class="panel-kicker">{{ t('logs.taskStream') }}</p>
          <h3>{{ t('logs.runtimeEvents') }}</h3>
        </div>
      </div>
      <div class="panel__body">
        <LogConsole :entries="tasksStore.logs" />
      </div>
    </section>
  </div>
</template>
