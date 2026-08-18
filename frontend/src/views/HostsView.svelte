<script lang="ts">
  import { onDestroy, tick } from "svelte";
  import { HostService } from "../../bindings/go-ssh-ui";
  import type { Config, Category, Host } from "../../bindings/go-ssh/config/models";
  import type { Files } from "../../bindings/go-ssh-ui/internal/configx/models";
  import { SvelteSet } from "svelte/reactivity";
  import { FolderPlus } from "@lucide/svelte";
  import CategoryTree from "../components/CategoryTree.svelte";
  import CategoryForm from "../components/CategoryForm.svelte";
  import HostForm from "../components/HostForm.svelte";
  import ConfirmDialog from "../components/ConfirmDialog.svelte";
  import { t } from "../lib/i18n/i18n.svelte";

  let {
    active = false,
    onConnectHost,
    onOpenFiles,
  }: {
    // Whether the Başlangıç tab is the one currently showing - this view
    // stays mounted in the background (App.svelte only toggles its tab-pane's
    // display) whenever another tab is active, so the arrow-key navigation
    // below must check this before acting or it'd steal keystrokes meant for
    // whatever tab (e.g. a terminal) is actually visible.
    active?: boolean;
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
  let confirmDialogRef: ReturnType<typeof ConfirmDialog> = $state()!;

  function flattenHosts(categories: Category[] | null | undefined, out: HostRef[] = []): HostRef[] {
    for (const cat of categories ?? []) {
      for (const h of cat.hosts ?? []) {
        if (h.id) out.push({ id: h.id, name: h.name });
      }
      flattenHosts(cat.categories, out);
    }
    return out;
  }

  // Keyboard tree navigation: Up/Down move across whatever rows are
  // currently on screen (categories *and* hosts) - a collapsed category is
  // one stop you arrow past, not a doorway that silently opens as you go by.
  // Right/Left are the actual open/close controls, mirroring a standard
  // treeview: Right opens the selected (collapsed) category; Left closes an
  // open one, or - once already closed, or on a host - steps up to the
  // parent category instead. Space/Return only ever connects a host.
  type TreeRow = { kind: "category"; path: string[]; category: Category } | { kind: "host"; path: string[]; host: Host };

  function rowKey(row: TreeRow): string {
    return row.kind === "category" ? row.path.join("/") : [...row.path, row.host.name].join("/");
  }

  // Order matches the tree's own render order (CategoryTree renders a
  // category's subcategories before its own direct hosts), so top-to-bottom
  // on screen matches Down-arrow order.
  function visibleRows(categories: Category[] | null | undefined, path: string[] = []): TreeRow[] {
    const out: TreeRow[] = [];
    for (const cat of categories ?? []) {
      const catPath = [...path, cat.name];
      out.push({ kind: "category", path: catPath, category: cat });
      if (expanded.has(catPath.join("/"))) {
        out.push(...visibleRows(cat.categories, catPath));
        for (const h of cat.hosts ?? []) out.push({ kind: "host", path: catPath, host: h });
      }
    }
    return out;
  }

  function parentRowOf(row: TreeRow, list: TreeRow[]): TreeRow | undefined {
    const parentPath = row.kind === "host" ? row.path : row.path.slice(0, -1);
    if (parentPath.length === 0) return undefined;
    const parentKey = parentPath.join("/");
    return list.find((r) => r.kind === "category" && rowKey(r) === parentKey);
  }

  let selectedRowKey: string | null = $state(null);
  let rows = $derived.by(() => visibleRows(cfg?.categories));

  async function selectRow(row: TreeRow) {
    selectedRowKey = rowKey(row);
    await tick();
    document.querySelector(`[data-row-key="${CSS.escape(selectedRowKey)}"]`)?.scrollIntoView({ block: "nearest" });
  }

  function handleTreeKeydown(e: KeyboardEvent) {
    if (!active || e.metaKey || e.ctrlKey || e.altKey) return;
    // Native <dialog> (CategoryForm/HostForm/ConfirmDialog, ...) doesn't stop
    // its keydowns from bubbling all the way to window even while it's the
    // modal top layer, so without this guard confirming/cancelling one of
    // those with Enter/Space would also connect to the selected host
    // underneath.
    if (document.querySelector("dialog[open]")) return;
    const target = document.activeElement;
    if (target instanceof HTMLElement && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable)) return;
    // A focused <button> (add-category, edit/delete/files icons, ...) or a
    // category/host row (both role="button") already does its own thing on
    // Space/Enter - don't also fire onConnectHost for a stale selection.
    if ((e.key === " " || e.key === "Enter") && target instanceof HTMLElement && (target.tagName === "BUTTON" || target.getAttribute("role") === "button")) {
      return;
    }
    const list = rows;
    const idx = list.findIndex((r) => rowKey(r) === selectedRowKey);
    const current = idx >= 0 ? list[idx] : null;

    if (e.key === "ArrowDown") {
      e.preventDefault();
      if (list.length === 0) return;
      selectRow(list[Math.min(idx + 1, list.length - 1)]);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      if (list.length === 0) return;
      selectRow(list[Math.max(idx - 1, 0)]);
    } else if (e.key === "ArrowRight") {
      if (!current || current.kind !== "category") return;
      e.preventDefault();
      expanded.add(current.path.join("/"));
    } else if (e.key === "ArrowLeft") {
      if (!current) return;
      e.preventDefault();
      if (current.kind === "category" && expanded.has(current.path.join("/"))) {
        expanded.delete(current.path.join("/"));
      } else {
        const parent = parentRowOf(current, list);
        if (parent) selectRow(parent);
      }
    } else if (e.key === " " || e.key === "Enter") {
      if (current?.kind === "host") {
        e.preventDefault();
        onConnectHost(current.path, current.host);
      }
    }
  }
  window.addEventListener("keydown", handleTreeKeydown);
  onDestroy(() => window.removeEventListener("keydown", handleTreeKeydown));

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

  // Called from outside (App.svelte, via bind:this) when a host was added
  // by something other than this view's own forms - e.g. Faz 5's "Kayıt"
  // review dialog, which saves directly through HostService.AddHost from a
  // terminal tab. This view only fetches on its own mount, and since
  // switching tabs/nav doesn't remount it when it was already showing
  // Hostlar, a save from elsewhere would otherwise sit invisible until the
  // next actual remount.
  export function reload() {
    load();
  }

  function applyConfig(next: Config) {
    cfg = next;
    allHosts = flattenHosts(next.categories);
  }

  function addRootCategory() {
    categoryFormRef.openAdd([]);
  }

  async function deleteCategory(path: string[], category: Category) {
    const ok = await confirmDialogRef.open(t("hostsView.confirmDelete.category", { name: category.name }));
    if (!ok) return;
    try {
      const next = await HostService.DeleteCategory({ sourceFile: category.sourceFile ?? "", categoryPath: path });
      if (next) cfg = next;
    } catch (err) {
      loadError = String(err);
    }
  }

  async function deleteHost(path: string[], host: Host) {
    const ok = await confirmDialogRef.open(t("hostsView.confirmDelete.host", { name: host.name }));
    if (!ok) return;
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
      {t("hostsView.buttons.addCategory")}
    </button>
  </div>

  {#if loadError}
    <p class="error-text">{loadError}</p>
  {:else if !cfg || !files}
    <p class="muted">{t("hostsView.loading")}</p>
  {:else if (cfg.categories ?? []).length === 0}
    <p class="muted">{t("hostsView.emptyState")}</p>
  {:else}
    <div class="glass-panel tree-panel">
      {#each cfg.categories ?? [] as cat (cat.name)}
        <CategoryTree
          category={cat}
          path={[cat.name]}
          {expanded}
          {selectedRowKey}
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
<ConfirmDialog bind:this={confirmDialogRef} />

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
