<script lang="ts">
import { defineComponent } from "vue";
import {
  deleteSource,
  importSources,
  listSources,
  updateSource,
  type BookSource,
} from "../../api/sources";
import {
  createUploadCollection,
  createURLCollection,
  deleteSourceCollection,
  listSourceCollections,
  replaceUploadCollection,
  syncSourceCollection,
  updateSourceCollection,
  type SourceCollection,
  type SyncInterval,
} from "../../api/source-collections";
import AppButton from "../../ui/components/AppButton.vue";
import FeatureScaffold from "../../ui/components/FeatureScaffold.vue";
import SourceCollectionDialog from "./SourceCollectionDialog.vue";
import SourceEditorDialog from "./SourceEditorDialog.vue";
import SourceImportDialog from "./SourceImportDialog.vue";
import {
  clampPage,
  pageCount,
  pageItems,
  pageRange,
} from "./source-pagination";
import {
  isJavaScriptSource,
  isSearchEligible,
  isWebViewSource,
  parseSourceImport,
  selectedImportJSON,
  sourceCapabilities,
  type SourceImportPreview,
} from "./source-import";
export default defineComponent({
  name: "SourceManagementView",
  components: {
    AppButton,
    FeatureScaffold,
    SourceCollectionDialog,
    SourceEditorDialog,
    SourceImportDialog,
  },
  data() {
    return {
      sources: [] as BookSource[],
      collections: [] as SourceCollection[],
      selectedCollectionId: "all",
      collectionDialog: false,
      editingCollection: null as SourceCollection | null,
      collectionBusy: false,
      collectionError: "",
      pendingCollectionDelete: null as SourceCollection | null,
      replacingCollectionId: "",
      loading: true,
      error: "",
      notice: "",
      query: "",
      group: "",
      importItems: [] as SourceImportPreview[],
      importFile: "",
      importing: false,
      importError: "",
      editing: null as BookSource | null,
      editorBusy: false,
      editorError: "",
      pendingDelete: null as BookSource | null,
      busyUrl: "",
      page: 1,
      pageSize: 25,
    };
  },
  computed: {
    groups(): string[] {
      return [
        ...new Set(
          this.sources
            .map((source) => source.bookSourceGroup || "")
            .filter(Boolean),
        ),
      ].sort((a, b) => a.localeCompare(b));
    },
    filtered(): BookSource[] {
      const query = this.query.trim().toLocaleLowerCase();
      return this.sources.filter(
        (source) =>
          (this.selectedCollectionId === "all" ||
            (this.selectedCollectionId === "standalone"
              ? !source.collectionId
              : source.collectionId === this.selectedCollectionId)) &&
          (!this.group || source.bookSourceGroup === this.group) &&
          (!query ||
            `${source.bookSourceName}\n${source.bookSourceUrl}\n${source.bookSourceGroup || ""}`
              .toLocaleLowerCase()
              .includes(query)),
      );
    },
    totalPages(): number {
      return pageCount(this.filtered.length, this.pageSize);
    },
    visibleSources(): BookSource[] {
      return pageItems(this.filtered, this.page, this.pageSize);
    },
    visibleRange(): { start: number; end: number } {
      return pageRange(this.page, this.pageSize, this.filtered.length);
    },
    enabledCount(): number {
      return this.sources.filter((source) => source.enabled).length;
    },
    exploreCount(): number {
      return this.sources.filter(
        (source) => source.enabledExplore && source.exploreUrl,
      ).length;
    },
    searchableCount(): number {
      return this.sources.filter(isSearchEligible).length;
    },
    javascriptCount(): number {
      return this.sources.filter(isJavaScriptSource).length;
    },
    webViewCount(): number {
      return this.sources.filter(isWebViewSource).length;
    },
    selectedCollectionSources(): BookSource[] {
      return this.sources.filter(
        (source) => source.collectionId === this.selectedCollectionId,
      );
    },
    selectedCollectionStats(): {
      total: number;
      enabled: number;
      searchable: number;
      explore: number;
      javascript: number;
      webview: number;
    } {
      const sources = this.selectedCollectionSources;
      return {
        total: sources.length,
        enabled: sources.filter((source) => source.enabled).length,
        searchable: sources.filter(isSearchEligible).length,
        explore: sources.filter(
          (source) => source.enabledExplore && source.exploreUrl,
        ).length,
        javascript: sources.filter(isJavaScriptSource).length,
        webview: sources.filter(isWebViewSource).length,
      };
    },
  },
  watch: {
    query() {
      this.page = 1;
    },
    group() {
      this.page = 1;
    },
    selectedCollectionId() {
      this.page = 1;
    },
    filtered() {
      this.page = clampPage(this.page, this.filtered.length, this.pageSize);
    },
  },
  async mounted() {
    await this.load();
  },
  methods: {
    async load() {
      this.loading = true;
      this.error = "";
      try {
        [this.sources, this.collections] = await Promise.all([
          listSources(),
          listSourceCollections(),
        ]);
      } catch (cause) {
        this.error =
          cause instanceof Error
            ? cause.message
            : this.$t("sources.loadFailed");
      } finally {
        this.loading = false;
      }
    },
    collectionName(source: BookSource): string {
      if (!source.collectionId) return this.$t("sources.collections.standalone");
      return this.collections.find((item) => item.id === source.collectionId)?.name || "";
    },
    capabilities(source: BookSource) {
      return sourceCapabilities(source);
    },
    openCollectionDialog(collection: SourceCollection | null = null) {
      this.editingCollection = collection;
      this.collectionError = "";
      this.collectionDialog = true;
    },
    async saveCollection(input: { mode: string; name: string; url: string; interval: SyncInterval; file: File | null }) {
      this.collectionBusy = true;
      this.collectionError = "";
      try {
        if (this.editingCollection) {
          await updateSourceCollection(this.editingCollection.id, { name: input.name, syncInterval: input.interval });
        } else if (input.mode === "upload" && input.file) {
          await createUploadCollection(input.name, input.file);
        } else {
          await createURLCollection(input.name, input.url, input.interval);
        }
        this.collectionDialog = false;
        this.editingCollection = null;
        this.notice = this.$t("sources.collections.saved");
        await this.load();
      } catch (cause) {
        this.collectionError = cause instanceof Error ? cause.message : this.$t("sources.collections.failed");
      } finally {
        this.collectionBusy = false;
      }
    },
    async syncCollection(collection: SourceCollection) {
      this.collectionBusy = true;
      this.error = "";
      try {
        const result = await syncSourceCollection(collection.id);
        this.notice = this.$t("sources.collections.synced", { ...result.changes });
        await this.load();
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t("sources.collections.failed");
      } finally {
        this.collectionBusy = false;
      }
    },
    chooseReplacement(collection: SourceCollection) {
      this.replacingCollectionId = collection.id;
      (this.$refs.replacementInput as HTMLInputElement)?.click();
    },
    async replaceCollectionFile(event: Event) {
      const input = event.target as HTMLInputElement;
      const file = input.files?.[0];
      input.value = "";
      if (!file || !this.replacingCollectionId) return;
      this.collectionBusy = true;
      try {
        const result = await replaceUploadCollection(this.replacingCollectionId, file);
        this.notice = this.$t("sources.collections.synced", { ...result.changes });
        await this.load();
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t("sources.collections.failed");
      } finally {
        this.collectionBusy = false;
        this.replacingCollectionId = "";
      }
    },
    async confirmCollectionDelete() {
      if (!this.pendingCollectionDelete) return;
      this.collectionBusy = true;
      try {
        await deleteSourceCollection(this.pendingCollectionDelete.id);
        this.notice = this.$t("sources.collections.deleted", { name: this.pendingCollectionDelete.name });
        if (this.selectedCollectionId === this.pendingCollectionDelete.id) this.selectedCollectionId = "all";
        this.pendingCollectionDelete = null;
        await this.load();
      } catch (cause) {
        this.error = cause instanceof Error ? cause.message : this.$t("sources.collections.failed");
      } finally {
        this.collectionBusy = false;
      }
    },
    async chooseFile(event: Event) {
      const input = event.target as HTMLInputElement;
      const file = input.files?.[0];
      input.value = "";
      if (!file) return;
      this.importError = "";
      if (file.size > 50 * 1024 * 1024) {
        this.importError = this.$t("sources.import.tooLarge");
        return;
      }
      try {
        this.importItems = parseSourceImport(await file.text());
        this.importFile = file.name;
      } catch {
        this.importError = this.$t("sources.import.invalid");
        this.notice = "";
      }
    },
    async confirmImport() {
      if (this.importing) return;
      this.importing = true;
      this.importError = "";
      try {
        const result = await importSources(
          selectedImportJSON(this.importItems),
        );
        this.notice = this.$t("sources.import.success", {
          imported: result.imported,
          total: result.total,
        });
        this.importItems = [];
        this.importFile = "";
        await this.load();
      } catch (cause) {
        this.importError =
          cause instanceof Error
            ? cause.message
            : this.$t("sources.import.failed");
      } finally {
        this.importing = false;
      }
    },
    async saveSource(source: BookSource) {
      if (!this.editing) return;
      this.editorBusy = true;
      this.editorError = "";
      try {
        await updateSource(this.editing.sourceId!, source);
        this.notice = this.$t("sources.editor.saved");
        this.editing = null;
        await this.load();
      } catch (cause) {
        this.editorError =
          cause instanceof Error
            ? cause.message
            : this.$t("sources.editor.failed");
      } finally {
        this.editorBusy = false;
      }
    },
    async toggle(source: BookSource, field: "enabled" | "enabledExplore") {
      this.busyUrl = source.sourceId!;
      this.error = "";
      try {
        await updateSource(source.sourceId!, {
          ...source,
          [field]: !source[field],
        });
        await this.load();
      } catch (cause) {
        this.error =
          cause instanceof Error
            ? cause.message
            : this.$t("sources.updateFailed");
      } finally {
        this.busyUrl = "";
      }
    },
    async confirmDelete() {
      if (!this.pendingDelete) return;
      this.busyUrl = this.pendingDelete.sourceId!;
      this.error = "";
      try {
        await deleteSource(this.pendingDelete.sourceId!);
        this.notice = this.$t("sources.delete.deleted", {
          name: this.pendingDelete.bookSourceName,
        });
        this.pendingDelete = null;
        await this.load();
      } catch (cause) {
        this.error =
          cause instanceof Error
            ? cause.message
            : this.$t("sources.delete.failed");
      } finally {
        this.busyUrl = "";
      }
    },
  },
});
</script>
<template>
  <FeatureScaffold
    :title="$t('sources.title')"
    :description="$t('sources.description')"
    >
<div class="page">
      <section class="toolbar">
        <div class="stats">
          <strong>{{
            $t("sources.stats.total", { count: sources.length })
          }}</strong><span>{{
            $t("sources.stats.enabled", { count: enabledCount })
          }}</span><span>{{
            $t("sources.stats.searchable", { count: searchableCount })
          }}</span><span>{{
            $t("sources.stats.explore", { count: exploreCount })
          }}</span><span>{{
            $t("sources.stats.javascript", { count: javascriptCount })
          }}</span><span>{{
            $t("sources.stats.webview", { count: webViewCount })
          }}</span>
        </div>
        <div class="toolbar-actions">
          <AppButton @click="openCollectionDialog()">{{ $t("sources.collections.add") }}</AppButton>
          <label class="import-button"><span>{{ $t("sources.import.openStandalone") }}</span><input
              type="file"
              accept=".json,application/json"
              @change="chooseFile"
          ></label>
        </div>
        <input ref="replacementInput" class="hidden-input" type="file" accept=".json,application/json" @change="replaceCollectionFile">
      </section>
      <section v-if="collections.length" class="collection-strip">
        <button :class="{ active: selectedCollectionId === 'all' }" @click="selectedCollectionId = 'all'">
          <strong>{{ $t('sources.collections.all') }}</strong>
        </button>
        <button v-for="collection in collections" :key="collection.id" :class="{ active: selectedCollectionId === collection.id }" @click="selectedCollectionId = collection.id">
          <strong>{{ collection.name }}</strong>
        </button>
        <button :class="{ active: selectedCollectionId === 'standalone' }" @click="selectedCollectionId = 'standalone'">
          <strong>{{ $t('sources.collections.standalone') }}</strong>
        </button>
      </section>
      <section v-if="selectedCollectionId !== 'all' && selectedCollectionId !== 'standalone'" class="collection-actions">
        <template v-for="collection in collections" :key="collection.id">
          <template v-if="collection.id === selectedCollectionId">
            <div class="collection-details">
              <strong>{{ collection.name }}</strong>
              <div class="collection-badges">
                <span>{{ collection.originKind === 'url' ? $t('sources.collections.url') : $t('sources.collections.upload') }}</span>
                <span>{{ $t('sources.stats.total', { count: selectedCollectionStats.total }) }}</span>
                <span>{{ $t('sources.stats.enabled', { count: selectedCollectionStats.enabled }) }}</span>
                <span>{{ $t('sources.stats.searchable', { count: selectedCollectionStats.searchable }) }}</span>
                <span>{{ $t('sources.stats.explore', { count: selectedCollectionStats.explore }) }}</span>
                <span>{{ $t('sources.stats.javascript', { count: selectedCollectionStats.javascript }) }}</span>
                <span>{{ $t('sources.stats.webview', { count: selectedCollectionStats.webview }) }}</span>
              </div>
              <small class="collection-address">{{ collection.originUrl || collection.originFilename }}</small>
              <p v-if="collection.lastError" class="collection-error">{{ collection.lastError }}</p>
            </div>
            <div class="collection-action-buttons">
              <AppButton variant="secondary" @click="openCollectionDialog(collection)">{{ $t('sources.collections.renameSettings') }}</AppButton>
              <AppButton variant="secondary" :busy="collectionBusy" @click="collection.originKind === 'url' ? syncCollection(collection) : chooseReplacement(collection)">{{ collection.originKind === 'url' ? $t('sources.collections.syncNow') : $t('sources.collections.replaceFile') }}</AppButton>
              <AppButton variant="danger" @click="pendingCollectionDelete = collection">{{ $t('sources.collections.delete') }}</AppButton>
            </div>
          </template>
        </template>
      </section>
      <section class="filters">
        <label><span>{{ $t("sources.search") }}</span><input
            v-model="query"
            type="search"
            :placeholder="$t('sources.searchPlaceholder')"
></label><label><span>{{ $t("sources.group") }}</span><select v-model="group">
            <option value="">{{ $t("sources.allGroups") }}</option>
            <option v-for="value in groups" :key="value" :value="value">
              {{ value }}
            </option>
          </select></label>
      </section>
      <p v-if="notice" class="notice" role="status">{{ notice }}</p>
      <p v-if="importError && !importItems.length" class="error" role="alert">
        {{ importError }}
      </p>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <p v-if="loading" aria-busy="true">{{ $t("sources.loading") }}</p>
      <section v-else-if="!sources.length" class="empty">
        <h2>{{ $t("sources.empty.title") }}</h2>
        <p>{{ $t("sources.empty.description") }}</p>
      </section>
      <section v-else-if="!filtered.length" class="empty">
        <h2>{{ $t("sources.empty.filteredTitle") }}</h2>
        <p>{{ $t("sources.empty.filteredDescription") }}</p>
      </section>
      <ul v-else class="source-list">
        <li
          v-for="source in visibleSources"
          :key="source.sourceId"
          :class="{ disabled: !source.enabled }"
        >
          <div class="identity">
            <div class="name">
              <strong>{{ source.bookSourceName }}</strong><span v-if="source.collectionId">{{ collectionName(source) }}</span><span v-if="source.bookSourceGroup">{{
                source.bookSourceGroup
              }}</span>
            </div>
            <small>{{ source.bookSourceUrl }}</small>
            <p v-if="source.bookSourceComment">
              {{ source.bookSourceComment }}
            </p>
            <div class="badges">
              <em
                v-for="capability in capabilities(source)"
                :key="capability"
                >{{ $t(`sources.capabilities.${capability}`) }}</em>
            </div>
          </div>
          <div class="switches">
            <label><input
                type="checkbox"
                :checked="source.enabled"
                :disabled="busyUrl === source.sourceId"
                @change="toggle(source, 'enabled')"
              ><span>{{ $t("sources.enabled") }}</span></label><label><input
                type="checkbox"
                :checked="source.enabledExplore"
                :disabled="
                  busyUrl === source.sourceId || !source.exploreUrl
                "
                @change="toggle(source, 'enabledExplore')"
              ><span>{{ $t("sources.exploreEnabled") }}</span></label>
          </div>
          <div class="actions">
            <AppButton
              variant="secondary"
              :disabled="busyUrl === source.sourceId"
              @click="editing = source"
              >
{{ $t("sources.edit") }}
</AppButton><AppButton
              variant="danger"
              :disabled="busyUrl === source.sourceId"
              @click="pendingDelete = source"
              >
{{ $t("sources.delete.action") }}
</AppButton>
          </div>
        </li>
      </ul>
      <nav
        v-if="filtered.length"
        class="pagination"
        :aria-label="$t('sources.pagination.label')"
      >
        <p>
          {{
            $t("sources.pagination.range", {
              start: visibleRange.start,
              end: visibleRange.end,
              total: filtered.length,
            })
          }}
        </p>
        <div>
          <AppButton
            variant="secondary"
            :disabled="page <= 1"
            @click="page--"
            >
{{ $t("sources.pagination.previous") }}
</AppButton>
          <span>{{
            $t("sources.pagination.page", { page, total: totalPages })
          }}</span>
          <AppButton
            variant="secondary"
            :disabled="page >= totalPages"
            @click="page++"
            >
{{ $t("sources.pagination.next") }}
</AppButton>
        </div>
      </nav>
    </div>
    <SourceCollectionDialog v-if="collectionDialog" :collection="editingCollection" :busy="collectionBusy" :error="collectionError" @submit="saveCollection" @close="collectionDialog = false; editingCollection = null" />
    <SourceImportDialog
      v-if="importItems.length"
      v-model:items="importItems"
      :file-name="importFile"
      :busy="importing"
      :error="importError"
      @confirm="confirmImport"
      @close="
        importItems = [];
        importFile = '';
        importError = '';
      "
    /><SourceEditorDialog
      v-if="editing"
      :source="editing"
      :busy="editorBusy"
      :server-error="editorError"
      @save="saveSource"
      @close="
        editing = null;
        editorError = '';
      "
    />
    <div v-if="pendingCollectionDelete" class="overlay" @click.self="pendingCollectionDelete = null"><section class="confirmation" role="alertdialog"><h2>{{ $t('sources.collections.deleteTitle') }}</h2><p>{{ $t('sources.collections.deleteDescription',{name:pendingCollectionDelete.name,count:pendingCollectionDelete.sourceCount}) }}</p><div><AppButton variant="secondary" :disabled="collectionBusy" @click="pendingCollectionDelete = null">{{ $t('sources.cancel') }}</AppButton><AppButton variant="danger" :busy="collectionBusy" @click="confirmCollectionDelete">{{ $t('sources.collections.deleteConfirm') }}</AppButton></div></section></div>
    <div
      v-if="pendingDelete"
      class="overlay"
      @click.self="pendingDelete = null"
    >
      <section
        class="confirmation"
        role="alertdialog"
        :aria-label="$t('sources.delete.title')"
      >
        <h2>{{ $t("sources.delete.title") }}</h2>
        <p>
          {{
            $t("sources.delete.description", {
              name: pendingDelete.bookSourceName,
            })
          }}
        </p>
        <small>{{ pendingDelete.bookSourceUrl }}</small>
        <div>
          <AppButton
            variant="secondary"
            :disabled="Boolean(busyUrl)"
            @click="pendingDelete = null"
            >
{{ $t("sources.cancel") }}
</AppButton><AppButton
            variant="danger"
            :busy="busyUrl === pendingDelete.sourceId"
            @click="confirmDelete"
            >
{{ $t("sources.delete.confirm") }}
</AppButton>
        </div>
      </section>
    </div>
</FeatureScaffold>
</template>
<style scoped>
.page {
  display: grid;
  gap: 1rem;
}
.toolbar,
.filters {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
}
.stats {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}
.stats strong,
.stats span {
  border-radius: 999px;
  padding: 0.35rem 0.65rem;
  background: var(--color-paper-muted);
  font-size: 0.78rem;
}
.import-button {
  min-height: 2.75rem;
  display: inline-flex;
  align-items: center;
  border-radius: var(--radius-md);
  padding: 0.65rem 1rem;
  background: var(--color-accent);
  color: white;
  font-weight: 700;
  cursor: pointer;
}
.toolbar-actions { display:flex; flex-wrap:wrap; gap:.5rem; }
.import-button input,
.hidden-input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
}
.collection-strip {
  display: flex;
  gap: 0.55rem;
  overflow: auto;
  padding: 0.2rem 0 0.45rem;
}
.collection-strip button {
  min-width: max-content;
  min-height: 2.75rem;
  display: inline-flex;
  align-items: center;
  text-align: left;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 0.65rem 0.9rem;
  background: var(--color-paper-raised);
  color: var(--color-ink);
}
.collection-strip button.active {
  border-color: var(--color-accent);
  background: var(--color-accent-soft);
}
.collection-strip strong {
  line-height: 1.25;
}
.collection-actions small {
  color: var(--color-ink-muted);
  font-size: 0.75rem;
  overflow-wrap: anywhere;
}

.collection-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1.5rem;
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-paper-raised);
}
.collection-details {
  min-width: 0;
  display: grid;
  gap: 0.55rem;
}
.collection-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
.collection-badges span {
  flex: 0 0 auto;
  border-radius: 999px;
  padding: 0.18rem 0.45rem;
  background: var(--color-paper-muted);
  color: var(--color-ink-muted);
  font-size: 0.7rem;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
.collection-address {
  display: block;
  min-width: 0;
  padding-top: 0.55rem;
  border-top: 1px solid var(--color-border);
}
.collection-action-buttons {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.55rem;
}
.collection-actions p { margin: 0; }
.collection-error { color:var(--color-danger); font-size:.8rem; }
.filters {
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-paper-raised);
}
.filters label {
  min-width: 0;
  flex: 1;
  display: grid;
  gap: 0.3rem;
}
.filters label:last-child {
  max-width: 18rem;
}
.filters span {
  color: var(--color-ink-muted);
  font-size: 0.78rem;
  font-weight: 700;
}
.filters input,
.filters select {
  width: 100%;
  min-width: 0;
  max-width: 100%;
  min-height: 2.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0.55rem 0.7rem;
  background: white;
  color: var(--color-ink);
}
.notice,
.error {
  margin: 0;
  padding: 0.7rem;
  border-radius: var(--radius-md);
}
.notice {
  background: #e7efe8;
  color: var(--color-success);
}
.error {
  background: #f8e4df;
  color: var(--color-danger);
}
.empty {
  padding: 2.5rem 1rem;
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-lg);
  text-align: center;
}
.empty h2 {
  font: 700 1.2rem var(--font-literary);
}
.empty p {
  color: var(--color-ink-muted);
}
.source-list {
  list-style: none;
  display: grid;
  gap: 0.65rem;
  margin: 0;
  padding: 0;
}
.source-list li {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  gap: 1rem;
  align-items: center;
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-paper-raised);
}
.pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding-top: 0.35rem;
}
.pagination p,
.pagination span {
  margin: 0;
  color: var(--color-ink-muted);
  font-size: 0.82rem;
  font-variant-numeric: tabular-nums;
}
.pagination div {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.75rem;
}
.source-list li.disabled {
  opacity: 0.68;
}
.identity {
  min-width: 0;
}
.name {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.45rem;
}
.name span {
  border-radius: 999px;
  padding: 0.18rem 0.42rem;
  background: var(--color-paper-muted);
  color: var(--color-ink-muted);
  font-size: 0.7rem;
}
.identity small,
.identity p {
  display: block;
  margin: 0.25rem 0 0;
  color: var(--color-ink-muted);
  overflow-wrap: anywhere;
}
.badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  margin-top: 0.45rem;
}
.badges em {
  border-radius: 999px;
  padding: 0.18rem 0.42rem;
  background: var(--color-accent-soft);
  color: var(--color-accent-strong);
  font-size: 0.68rem;
  font-style: normal;
}
.switches {
  display: grid;
  gap: 0.55rem;
}
.switches label {
  min-height: 2.75rem;
  display: flex;
  align-items: center;
  gap: 0.45rem;
}
.switches input {
  width: 1.15rem;
  height: 1.15rem;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.4rem;
}
.overlay {
  position: fixed;
  z-index: 90;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 1rem;
  background: rgb(0 0 0/0.42);
}
.confirmation {
  width: min(30rem, 100%);
  padding: 1rem;
  border-radius: var(--radius-lg);
  background: var(--color-paper-raised);
}
.confirmation h2 {
  font: 700 1.2rem var(--font-literary);
}
.confirmation small {
  display: block;
  color: var(--color-ink-muted);
  overflow-wrap: anywhere;
}
.confirmation div {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 1rem;
}
@media (max-width: 54rem) {
  .source-list li {
    grid-template-columns: 1fr auto;
  }
  .switches {
    grid-column: 1/2;
    display: flex;
    flex-wrap: wrap;
  }
  .actions {
    grid-column: 2/3;
    grid-row: 1/3;
  }
}
@media (max-width: 38rem) {
  .toolbar,
  .filters,
  .collection-actions,
  .pagination {
    align-items: stretch;
    flex-direction: column;
  }
  .collection-action-buttons {
    justify-content: flex-start;
  }
  .collection-action-buttons :deep(.app-button) {
    flex: 1 1 auto;
  }
  .filters label,
  .filters label:last-child {
    width: 100%;
    max-width: none;
  }
  .pagination div {
    width: 100%;
    justify-content: space-between;
  }
  .import-button {
    justify-content: center;
  }
  .source-list li {
    grid-template-columns: 1fr;
  }
  .switches,
  .actions {
    grid-column: auto;
    grid-row: auto;
  }
  .actions {
    justify-content: stretch;
  }
  .actions :deep(.app-button) {
    flex: 1;
  }
  .overlay {
    align-items: end;
    padding: 0;
  }
  .confirmation {
    border-radius: var(--radius-lg) var(--radius-lg) 0 0;
  }
}
</style>
