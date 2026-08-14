<script lang="ts">
  import { HostService } from "../../bindings/go-ssh-ui";
  import type { Config, Category, Host } from "../../bindings/go-ssh/config/models";
  import type { Files } from "../../bindings/go-ssh-ui/internal/configx/models";
  import { SvelteSet } from "svelte/reactivity";
  import { FolderPlus } from "@lucide/svelte";
  import CategoryTree from "../components/CategoryTree.svelte";
  import CategoryForm from "../components/CategoryForm.svelte";
  import HostForm from "../components/HostForm.svelte";

  let {
    onConnectHost,
    onOpenFiles,
  }: {
    onConnectHost: (categoryPath: string[], host: Host) => void;
    onOpenFiles: (categoryPath: string[], host: Host) => void;
  } = $props();

  type HostRef = { id: string; name: string };

  let cfg: Config | null = $state(null);
  let files: Files | null = $state(null);
  let loadError: string | null = $state(null);
  // A plain `Set` doesn't get Svelte's reactivity tracking on add/delete -
  // only property access on plain objects/arrays is tracked automatically.
  // SvelteSet is the reactivity/reactivity-aware wrapper needed for the
  // expand/collapse toggle to actually re-render.
  let expanded = new SvelteSet<string>();
  let allHosts: HostRef[] = $state([]);

  let categoryFormRef: ReturnType<typeof CategoryForm> = $state()!;
  let hostFormRef: ReturnType<typeof HostForm> = $state()!;

  function flattenHosts(categories: Category[] | null | undefined, out: HostRef[] = []): HostRef[] {
    for (const cat of categories ?? []) {
      for (const h of cat.hosts ?? []) {
        if (h.id) out.push({ id: h.id, name: h.name });
      }
      flattenHosts(cat.categories, out);
    }
    return out;
  }

  async function load() {
    try {
      const [treeResult, filesResult] = await Promise.all([HostService.GetTree(), HostService.GetFiles()]);
      cfg = treeResult;
      files = filesResult;
      allHosts = flattenHosts(treeResult?.categories);
      loadError = null;
    } catch (err) {
      loadError = String(err);
    }
  }
  load();

  function applyConfig(next: Config) {
    cfg = next;
    allHosts = flattenHosts(next.categories);
  }

  function addRootCategory() {
    categoryFormRef.openAdd([]);
  }

  async function deleteCategory(path: string[], category: Category) {
    if (!confirm(`"${category.name}" kategorisini (ve içindekileri) silmek istediğine emin misin?`)) return;
    try {
      const next = await HostService.DeleteCategory({ sourceFile: category.sourceFile ?? "", categoryPath: path });
      if (next) cfg = next;
    } catch (err) {
      loadError = String(err);
    }
  }

  async function deleteHost(path: string[], host: Host) {
    if (!confirm(`"${host.name}" hostunu silmek istediğine emin misin?`)) return;
    try {
      const next = await HostService.DeleteHost({ sourceFile: host.sourceFile ?? "", categoryPath: path, name: host.name });
      if (next) cfg = next;
    } catch (err) {
      loadError = String(err);
    }
  }
</script>

<div class="hosts-view">
  <div class="row end action-bar">
    <button type="button" class="primary" onclick={addRootCategory}>
      <FolderPlus size={14} strokeWidth={2} />
      Kategori
    </button>
  </div>

  {#if loadError}
    <p class="error-text">{loadError}</p>
  {:else if !cfg || !files}
    <p class="muted">Yükleniyor…</p>
  {:else if (cfg.categories ?? []).length === 0}
    <p class="muted">Henüz kategori yok. Başlamak için "+ Kategori" ile bir tane oluştur.</p>
  {:else}
    <div class="glass-panel tree-panel">
      {#each cfg.categories ?? [] as cat (cat.name)}
        <CategoryTree
          category={cat}
          path={[cat.name]}
          {expanded}
          onAddSubcategory={(p) => categoryFormRef.openAdd(p)}
          onEditCategory={(p, c) => categoryFormRef.openEdit(p, c)}
          onDeleteCategory={deleteCategory}
          onAddHost={(p) => hostFormRef.openAdd(p)}
          onEditHost={(p, h) => hostFormRef.openEdit(p, h)}
          onDeleteHost={deleteHost}
          {onConnectHost}
          {onOpenFiles}
        />
      {/each}
    </div>
  {/if}
</div>

{#if files}
  <CategoryForm bind:this={categoryFormRef} {files} onSaved={applyConfig} />
  <HostForm bind:this={hostFormRef} {files} {allHosts} onSaved={applyConfig} />
{/if}

<style>
  .hosts-view {
    padding: 16px 20px 32px;
    height: 100%;
    overflow-y: auto;
  }
  .action-bar {
    margin-bottom: 12px;
  }
  .tree-panel {
    padding: 8px;
  }
</style>
