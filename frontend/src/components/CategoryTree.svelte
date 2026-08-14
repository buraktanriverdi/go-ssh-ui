<script lang="ts">
  import type { Category, Host } from "../../bindings/go-ssh/config/models";
  import { ChevronRight, ChevronDown, Folder, Server, Plus, FolderPlus, Pencil, Trash2, FolderOpen } from "@lucide/svelte";
  import CategoryTree from "./CategoryTree.svelte";

  let {
    category,
    path,
    expanded,
    onAddSubcategory,
    onEditCategory,
    onDeleteCategory,
    onAddHost,
    onEditHost,
    onDeleteHost,
    onConnectHost,
    onOpenFiles,
  }: {
    category: Category;
    path: string[];
    expanded: Set<string>;
    onAddSubcategory: (path: string[]) => void;
    onEditCategory: (path: string[], category: Category) => void;
    onDeleteCategory: (path: string[], category: Category) => void;
    onAddHost: (path: string[]) => void;
    onEditHost: (path: string[], host: Host) => void;
    onDeleteHost: (path: string[], host: Host) => void;
    onConnectHost: (path: string[], host: Host) => void;
    onOpenFiles: (path: string[], host: Host) => void;
  } = $props();

  let key = $derived(path.join("/"));
  function toggle() {
    if (expanded.has(key)) expanded.delete(key);
    else expanded.add(key);
  }
</script>

<div class="category-node">
  <div class="list-row category-row" onclick={toggle} role="button" tabindex="0" onkeydown={(e) => e.key === "Enter" && toggle()}>
    <span class="chevron">
      {#if expanded.has(key)}<ChevronDown size={13} strokeWidth={2.5} />{:else}<ChevronRight size={13} strokeWidth={2.5} />{/if}
    </span>
    <Folder size={15} strokeWidth={1.75} class="row-icon" />
    <span class="category-name">{category.name}</span>
    {#if category.description}<span class="muted category-desc">{category.description}</span>{/if}
    <span class="spacer"></span>
    <button type="button" class="icon-btn" onclick={(e) => { e.stopPropagation(); onAddHost(path); }} title="Host ekle">
      <Plus size={14} strokeWidth={2} />
    </button>
    <button type="button" class="icon-btn" onclick={(e) => { e.stopPropagation(); onAddSubcategory(path); }} title="Alt kategori ekle">
      <FolderPlus size={14} strokeWidth={2} />
    </button>
    <button type="button" class="icon-btn" onclick={(e) => { e.stopPropagation(); onEditCategory(path, category); }} title="Düzenle">
      <Pencil size={13} strokeWidth={2} />
    </button>
    <button type="button" class="icon-btn danger" onclick={(e) => { e.stopPropagation(); onDeleteCategory(path, category); }} title="Sil">
      <Trash2 size={13} strokeWidth={2} />
    </button>
  </div>

  {#if expanded.has(key)}
    <div class="category-children">
      {#each category.categories ?? [] as sub (sub.name)}
        <CategoryTree
          category={sub}
          path={[...path, sub.name]}
          {expanded}
          {onAddSubcategory}
          {onEditCategory}
          {onDeleteCategory}
          {onAddHost}
          {onEditHost}
          {onDeleteHost}
          {onConnectHost}
          {onOpenFiles}
        />
      {/each}
      {#each category.hosts ?? [] as host (host.name)}
        <div
          class="list-row host-row"
          onclick={() => onConnectHost(path, host)}
          role="button"
          tabindex="0"
          onkeydown={(e) => e.key === "Enter" && onConnectHost(path, host)}
        >
          <span class="chevron-spacer"></span>
          <Server size={14} strokeWidth={1.75} class="row-icon" />
          <span class="host-name">{host.name}</span>
          {#if host.description}<span class="muted">{host.description}</span>{/if}
          <span class="spacer"></span>
          <button type="button" class="icon-btn files-btn" onclick={(e) => { e.stopPropagation(); onOpenFiles(path, host); }} title="Dosyalar">
            <FolderOpen size={13} strokeWidth={2} />
          </button>
          <button type="button" class="icon-btn" onclick={(e) => { e.stopPropagation(); onEditHost(path, host); }} title="Düzenle">
            <Pencil size={13} strokeWidth={2} />
          </button>
          <button type="button" class="icon-btn danger" onclick={(e) => { e.stopPropagation(); onDeleteHost(path, host); }} title="Sil">
            <Trash2 size={13} strokeWidth={2} />
          </button>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .category-row,
  .host-row {
    cursor: default;
    padding: 4px 6px;
  }
  .chevron {
    display: inline-flex;
    width: 14px;
    color: var(--text-muted);
  }
  .chevron-spacer {
    width: 14px;
  }
  :global(.row-icon) {
    color: var(--text-muted);
    flex-shrink: 0;
  }
  .category-name,
  .host-name {
    font-weight: 500;
  }
  .category-desc {
    font-weight: normal;
  }
  .spacer {
    flex: 1;
  }
  .category-row .icon-btn,
  .host-row .icon-btn {
    opacity: 0;
  }
  .category-row:hover .icon-btn,
  .host-row:hover .icon-btn {
    opacity: 1;
  }
  /* "Dosyalar" is a primary action (like connecting), not a secondary one
     like edit/delete - it stays visible instead of hover-only so it's
     actually discoverable. */
  .host-row .files-btn {
    opacity: 1;
    color: var(--accent);
  }
  .category-children {
    margin-left: 12px;
    border-left: 1px solid var(--surface-border);
    padding-left: 8px;
  }
  .host-row {
    margin-left: 4px;
  }
</style>
