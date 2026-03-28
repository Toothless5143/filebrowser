<template>
  <div class="trash-view">
    <div class="card-title">
      <h1><i class="material-icons" style="vertical-align: middle; margin-right: 0.3em;">delete</i>{{ $t('trash.title') }}</h1>
      <p class="description">{{ $t('trash.description') }}</p>
    </div>

    <!-- Source selector (if multiple sources) -->
    <div v-if="availableSources.length > 1" class="source-selector">
      <label>{{ $t('general.source') }}:</label>
      <select v-model="selectedSource" @change="loadTrash">
        <option v-for="src in availableSources" :key="src" :value="src">{{ src }}</option>
      </select>
    </div>

    <!-- Loading state -->
    <div v-if="loading" class="loading-state">
      <i class="material-icons spin">sync</i>
      <span>{{ $t('general.loading') }}</span>
    </div>

    <!-- Empty trash state -->
    <div v-else-if="!loading && trashItems.length === 0" class="empty-state">
      <i class="material-icons" style="font-size: 4em; color: var(--textSecondary);">delete_outline</i>
      <p>{{ $t('trash.empty') }}</p>
    </div>

    <!-- Trash item list -->
    <div v-else>
      <!-- Toolbar -->
      <div class="trash-toolbar">
        <div class="toolbar-left">
          <label class="select-all-label">
            <input
              type="checkbox"
              :checked="allSelected"
              :indeterminate.prop="someSelected && !allSelected"
              @change="toggleSelectAll"
            />
            <span>{{ $t('buttons.selectAll') }}</span>
          </label>
          <span v-if="selectedIds.length > 0" class="selection-count">
            {{ selectedIds.length }} {{ $t('trash.selected') }}
          </span>
        </div>
        <div class="toolbar-right">
          <button
            v-if="selectedIds.length > 0"
            class="button button--flat"
            @click="restoreSelected"
            :disabled="restoring"
          >
            <i class="material-icons">restore_from_trash</i>
            {{ $t('trash.restore') }}
          </button>
          <button
            v-if="selectedIds.length > 0"
            class="button button--flat button--red"
            @click="deleteSelected"
            :disabled="deleting"
          >
            <i class="material-icons">delete_forever</i>
            {{ $t('trash.deletePermanently') }}
          </button>
          <button
            class="button button--flat button--red"
            @click="confirmEmptyTrash"
            :disabled="deleting || trashItems.length === 0"
          >
            <i class="material-icons">delete_sweep</i>
            {{ $t('trash.emptyTrash') }}
          </button>
        </div>
      </div>

      <!-- Items list -->
      <div class="trash-list">
        <div
          v-for="item in trashItems"
          :key="item.trashId"
          class="trash-item"
          :class="{ selected: isSelected(item.trashId) }"
          @click="toggleSelect(item.trashId)"
        >
          <div class="trash-item-checkbox">
            <input
              type="checkbox"
              :checked="isSelected(item.trashId)"
              @click.stop
              @change="toggleSelect(item.trashId)"
            />
          </div>
          <div class="trash-item-icon">
            <i class="material-icons">{{ item.isDir ? 'folder' : 'insert_drive_file' }}</i>
          </div>
          <div class="trash-item-info">
            <div class="trash-item-name">{{ item.name }}</div>
            <div class="trash-item-meta">
              <span class="trash-item-path" :title="item.originalPath">{{ item.originalPath }}</span>
              <span class="trash-item-date">{{ formatDate(item.deletedAt) }}</span>
              <span v-if="!item.isDir" class="trash-item-size">{{ formatSize(item.size) }}</span>
            </div>
          </div>
          <div class="trash-item-actions">
            <button
              class="button button--flat button--small"
              :title="$t('trash.restore')"
              @click.stop="restoreItem(item.trashId)"
            >
              <i class="material-icons">restore_from_trash</i>
            </button>
            <button
              class="button button--flat button--small button--red"
              :title="$t('trash.deletePermanently')"
              @click.stop="deleteItem(item.trashId)"
            >
              <i class="material-icons">delete_forever</i>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Confirm empty trash dialog -->
    <div v-if="showEmptyConfirm" class="overlay confirm-overlay" @click.self="showEmptyConfirm = false">
      <div class="card confirm-dialog">
        <div class="card-title">
          <h2>{{ $t('trash.emptyTrash') }}</h2>
        </div>
        <div class="card-content">
          <p>{{ $t('trash.emptyConfirm') }}</p>
        </div>
        <div class="card-actions">
          <button class="button button--flat button--grey" @click="showEmptyConfirm = false">
            {{ $t('general.cancel') }}
          </button>
          <button class="button button--flat button--red" @click="emptyTrash">
            {{ $t('trash.emptyTrash') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { trashApi } from "@/api";
import { state } from "@/store";
import { notify } from "@/notify";

export default {
  name: "Trash",
  data() {
    return {
      trashItems: [],
      selectedIds: [],
      loading: false,
      restoring: false,
      deleting: false,
      selectedSource: "",
      showEmptyConfirm: false,
    };
  },
  computed: {
    availableSources() {
      return state.sources && state.sources.info ? Object.keys(state.sources.info) : [];
    },
    allSelected() {
      return this.trashItems.length > 0 && this.selectedIds.length === this.trashItems.length;
    },
    someSelected() {
      return this.selectedIds.length > 0;
    },
  },
  mounted() {
    // Pick a default source
    const sources = this.availableSources;
    if (sources.length > 0) {
      this.selectedSource = state.sources.current || sources[0];
    } else {
      this.selectedSource = state.req?.source || "";
    }
    this.loadTrash();
  },
  methods: {
    async loadTrash() {
      if (!this.selectedSource) return;
      this.loading = true;
      this.selectedIds = [];
      try {
        this.trashItems = await trashApi.listTrash(this.selectedSource);
        // Sort by deletedAt descending (most recently deleted first)
        this.trashItems.sort((a, b) => new Date(b.deletedAt) - new Date(a.deletedAt));
      } catch (e) {
        // Error already shown by the API layer
      } finally {
        this.loading = false;
      }
    },
    isSelected(trashId) {
      return this.selectedIds.includes(trashId);
    },
    toggleSelect(trashId) {
      const idx = this.selectedIds.indexOf(trashId);
      if (idx >= 0) {
        this.selectedIds.splice(idx, 1);
      } else {
        this.selectedIds.push(trashId);
      }
    },
    toggleSelectAll() {
      if (this.allSelected) {
        this.selectedIds = [];
      } else {
        this.selectedIds = this.trashItems.map((i) => i.trashId);
      }
    },
    async restoreItem(trashId) {
      this.restoring = true;
      try {
        await trashApi.restoreFromTrash(this.selectedSource, [trashId]);
        notify.showSuccessToast(this.$t("trash.restoredSuccess"));
        await this.loadTrash();
      } catch (e) {
        // Error already shown by the API layer
      } finally {
        this.restoring = false;
      }
    },
    async restoreSelected() {
      if (this.selectedIds.length === 0) return;
      this.restoring = true;
      try {
        const result = await trashApi.restoreFromTrash(this.selectedSource, [...this.selectedIds]);
        if (result.failed && result.failed.length > 0) {
          notify.showError(this.$t("trash.restorePartial", { count: result.failed.length }));
        } else {
          notify.showSuccessToast(this.$t("trash.restoredSuccess"));
        }
        await this.loadTrash();
      } catch (e) {
        // Error already shown by the API layer
      } finally {
        this.restoring = false;
      }
    },
    async deleteItem(trashId) {
      this.deleting = true;
      try {
        await trashApi.deleteFromTrash(this.selectedSource, [trashId]);
        notify.showSuccessToast(this.$t("trash.deletedSuccess"));
        await this.loadTrash();
      } catch (e) {
        // Error already shown by the API layer
      } finally {
        this.deleting = false;
      }
    },
    async deleteSelected() {
      if (this.selectedIds.length === 0) return;
      this.deleting = true;
      try {
        const result = await trashApi.deleteFromTrash(this.selectedSource, [...this.selectedIds]);
        if (result.failed && result.failed.length > 0) {
          notify.showError(this.$t("trash.deletePartial", { count: result.failed.length }));
        } else {
          notify.showSuccessToast(this.$t("trash.deletedSuccess"));
        }
        await this.loadTrash();
      } catch (e) {
        // Error already shown by the API layer
      } finally {
        this.deleting = false;
      }
    },
    confirmEmptyTrash() {
      this.showEmptyConfirm = true;
    },
    async emptyTrash() {
      this.showEmptyConfirm = false;
      this.deleting = true;
      try {
        await trashApi.emptyTrash(this.selectedSource);
        notify.showSuccessToast(this.$t("trash.emptiedSuccess"));
        await this.loadTrash();
      } catch (e) {
        // Error already shown by the API layer
      } finally {
        this.deleting = false;
      }
    },
    formatDate(dateStr) {
      if (!dateStr) return "";
      const d = new Date(dateStr);
      return d.toLocaleString();
    },
    formatSize(bytes) {
      if (!bytes) return "0 B";
      const units = ["B", "KB", "MB", "GB", "TB"];
      let i = 0;
      let size = bytes;
      while (size >= 1024 && i < units.length - 1) {
        size /= 1024;
        i++;
      }
      return `${size.toFixed(1)} ${units[i]}`;
    },
  },
};
</script>

<style scoped>
.trash-view {
  padding: 1em;
  max-width: 900px;
  margin: 0 auto;
}

.card-title {
  margin-bottom: 1em;
}

.description {
  color: var(--textSecondary, #888);
  margin-top: 0.25em;
}

.source-selector {
  margin-bottom: 1em;
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.source-selector select {
  padding: 0.25em 0.5em;
  border: 1px solid var(--borderColor, #ccc);
  border-radius: 4px;
  background: var(--surfaceSecondary, #fff);
  color: var(--textPrimary, #000);
}

.loading-state {
  display: flex;
  align-items: center;
  gap: 0.5em;
  padding: 2em;
  justify-content: center;
  color: var(--textSecondary, #888);
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.empty-state {
  text-align: center;
  padding: 3em;
  color: var(--textSecondary, #888);
}

.trash-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75em 0;
  border-bottom: 1px solid var(--borderColor, #e0e0e0);
  margin-bottom: 0.5em;
  flex-wrap: wrap;
  gap: 0.5em;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 1em;
}

.select-all-label {
  display: flex;
  align-items: center;
  gap: 0.4em;
  cursor: pointer;
  user-select: none;
}

.selection-count {
  color: var(--textSecondary, #888);
  font-size: 0.9em;
}

.toolbar-right {
  display: flex;
  gap: 0.5em;
  flex-wrap: wrap;
}

.trash-list {
  display: flex;
  flex-direction: column;
  gap: 0.25em;
}

.trash-item {
  display: flex;
  align-items: center;
  padding: 0.75em;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.15s;
  border: 1px solid transparent;
}

.trash-item:hover {
  background: var(--surfaceSecondary, rgba(0,0,0,0.04));
}

.trash-item.selected {
  background: var(--primaryColorLight, rgba(63, 81, 181, 0.08));
  border-color: var(--primaryColor, #3f51b5);
}

.trash-item-checkbox {
  margin-right: 0.75em;
  flex-shrink: 0;
}

.trash-item-icon {
  margin-right: 0.75em;
  flex-shrink: 0;
  color: var(--textSecondary, #888);
}

.trash-item-info {
  flex: 1;
  min-width: 0;
}

.trash-item-name {
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.trash-item-meta {
  display: flex;
  gap: 1em;
  font-size: 0.8em;
  color: var(--textSecondary, #888);
  flex-wrap: wrap;
  margin-top: 0.2em;
}

.trash-item-path {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 30em;
}

.trash-item-actions {
  display: flex;
  gap: 0.25em;
  flex-shrink: 0;
  margin-left: 0.5em;
}

.button--small {
  padding: 0.2em 0.4em !important;
  font-size: 0.85em;
}

.confirm-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
}

.confirm-dialog {
  max-width: 400px;
  width: 90%;
  background: var(--surfacePrimary, #fff);
  border-radius: 8px;
  padding: 1.5em;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.2);
}

.confirm-dialog .card-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5em;
  margin-top: 1em;
}
</style>
