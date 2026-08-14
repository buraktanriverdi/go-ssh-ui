<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Events } from "@wailsio/runtime";
  import { SvelteMap } from "svelte/reactivity";
  import {
    Folder,
    File,
    Link2,
    ArrowUp,
    RefreshCw,
    FolderPlus,
    Download,
    Upload,
    Trash2,
    Pencil,
    HardDrive,
    Server,
    FolderOpen,
  } from "@lucide/svelte";
  import { FileService } from "../../bindings/go-ssh-ui";
  import type { Entry } from "../../bindings/go-ssh-ui/internal/scpfs/models";

  let { categoryPath, hostName }: { categoryPath: string[]; hostName: string } = $props();

  const hostReq = { categoryPath, name: hostName };

  let canBrowse: boolean | null = $state(null);
  let checkError: string | null = $state(null);

  // Remote paths are always relative-to-login-home segments, rebuilt on
  // every request - each RunRemoteCommand/OpenExec is a fresh one-off ssh
  // command, so there's no persistent remote cwd to rely on (see
  // scpfs.Service.List's "." default).
  let remoteSegs: string[] = $state([]);
  let remoteEntries: Entry[] = $state([]);
  let remoteLoading = $state(false);
  let remoteError: string | null = $state(null);

  let localDir = $state("");
  let localEntries: Entry[] = $state([]);
  let localLoading = $state(false);
  let localError: string | null = $state(null);
  let selectedLocal: Entry | null = $state(null);

  type Transfer = { name: string; done: number; total: number; error?: string; finished: boolean };
  let transfers = new SvelteMap<string, Transfer>();

  let mkdirDialog: HTMLDialogElement;
  let mkdirName = $state("");
  let renameDialog: HTMLDialogElement;
  let renameTarget: Entry | null = $state(null);
  let renameValue = $state("");

  const dropZoneId = `filedrop-${crypto.randomUUID()}`;
  let dropActive = $state(false);
  const unsubs: (() => void)[] = [];

  function remoteDirParam(): string {
    return remoteSegs.length ? remoteSegs.join("/") : ".";
  }

  function remotePath(name: string): string {
    return [...remoteSegs, name].join("/");
  }

  async function loadRemote() {
    remoteLoading = true;
    remoteError = null;
    try {
      remoteEntries = (await FileService.List({ ...hostReq, dir: remoteDirParam() })) ?? [];
    } catch (err) {
      remoteError = String(err);
    } finally {
      remoteLoading = false;
    }
  }

  async function loadLocal() {
    localLoading = true;
    localError = null;
    try {
      localEntries = (await FileService.ListLocal(localDir)) ?? [];
    } catch (err) {
      localError = String(err);
    } finally {
      localLoading = false;
    }
  }

  function openRemoteDir(entry: Entry) {
    if (!entry.isDir) return;
    remoteSegs = [...remoteSegs, entry.name];
    loadRemote();
  }

  function remoteUp() {
    if (!remoteSegs.length) return;
    remoteSegs = remoteSegs.slice(0, -1);
    loadRemote();
  }

  function jumpRemote(index: number) {
    remoteSegs = remoteSegs.slice(0, index);
    loadRemote();
  }

  function openLocalDir(entry: Entry) {
    if (entry.isDir) {
      localDir = localDir.endsWith("/") ? localDir + entry.name : localDir + "/" + entry.name;
      selectedLocal = null;
      loadLocal();
    } else {
      selectedLocal = entry;
    }
  }

  function localUp() {
    const idx = localDir.lastIndexOf("/");
    if (idx <= 0) return;
    localDir = localDir.slice(0, idx);
    selectedLocal = null;
    loadLocal();
  }

  function localSegments(): { name: string; path: string }[] {
    const parts = localDir.split("/").filter(Boolean);
    let acc = "";
    return parts.map((p) => {
      acc += "/" + p;
      return { name: p, path: acc };
    });
  }

  function jumpLocal(path: string) {
    localDir = path;
    selectedLocal = null;
    loadLocal();
  }

  function trackTransfer(id: string, name: string) {
    transfers.set(id, { name, done: 0, total: 0, finished: false });
  }

  function openMkdir() {
    mkdirName = "";
    mkdirDialog.showModal();
  }

  async function submitMkdir(e: Event) {
    e.preventDefault();
    try {
      await FileService.Mkdir({ ...hostReq, dir: remotePath(mkdirName) });
      mkdirDialog.close();
      loadRemote();
    } catch (err) {
      remoteError = String(err);
    }
  }

  function openRename(entry: Entry) {
    renameTarget = entry;
    renameValue = entry.name;
    renameDialog.showModal();
  }

  async function submitRename(e: Event) {
    e.preventDefault();
    if (!renameTarget) return;
    try {
      await FileService.Rename({ ...hostReq, from: remotePath(renameTarget.name), to: remotePath(renameValue) });
      renameDialog.close();
      loadRemote();
    } catch (err) {
      remoteError = String(err);
    }
  }

  async function deleteRemote(entry: Entry) {
    if (!confirm(`"${entry.name}" silinsin mi?`)) return;
    try {
      await FileService.Delete({ ...hostReq, target: remotePath(entry.name) });
      loadRemote();
    } catch (err) {
      remoteError = String(err);
    }
  }

  async function downloadRemote(entry: Entry) {
    if (entry.isDir) return;
    const res = await FileService.Download({ ...hostReq, remotePath: remotePath(entry.name) });
    trackTransfer(res.transferId, entry.name);
  }

  async function uploadLocalPath(localPath: string) {
    const name = localPath.split("/").pop() ?? localPath;
    const res = await FileService.Upload({ ...hostReq, remoteDir: remoteDirParam(), localPath });
    trackTransfer(res.transferId, name);
  }

  async function uploadSelected() {
    if (!selectedLocal) return;
    await uploadLocalPath(localDir.endsWith("/") ? localDir + selectedLocal.name : localDir + "/" + selectedLocal.name);
    selectedLocal = null;
  }

  async function pickAndUpload() {
    const path = await FileService.PickLocalFile();
    if (path) await uploadLocalPath(path);
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    const units = ["KB", "MB", "GB", "TB"];
    let val = bytes / 1024;
    let i = 0;
    while (val >= 1024 && i < units.length - 1) {
      val /= 1024;
      i++;
    }
    return `${val.toFixed(val >= 10 ? 0 : 1)} ${units[i]}`;
  }

  onMount(async () => {
    unsubs.push(
      Events.On("file:progress", (ev) => {
        const t = transfers.get(ev.data.transferId);
        if (t) transfers.set(ev.data.transferId, { ...t, done: ev.data.done, total: ev.data.total });
      }),
    );
    unsubs.push(
      Events.On("file:done", (ev) => {
        const t = transfers.get(ev.data.transferId);
        if (t) transfers.set(ev.data.transferId, { ...t, finished: true, error: ev.data.error });
        if (!ev.data.error) loadRemote();
        setTimeout(() => transfers.delete(ev.data.transferId), ev.data.error ? 6000 : 2500);
      }),
    );
    unsubs.push(
      Events.On("files:dropped", (ev) => {
        if (ev.data.elementId !== dropZoneId) return;
        dropActive = false;
        for (const f of ev.data.files as string[]) uploadLocalPath(f);
      }),
    );

    try {
      canBrowse = await FileService.CanBrowse(hostReq);
    } catch (err) {
      checkError = String(err);
      canBrowse = false;
    }
    if (canBrowse) {
      try {
        localDir = await FileService.HomeDir();
      } catch {
        localDir = "/";
      }
      await Promise.all([loadRemote(), loadLocal()]);
    }
  });

  onDestroy(() => {
    unsubs.forEach((u) => u());
  });
</script>

{#if canBrowse === null}
  <div class="center-wrap"><span class="muted">Kontrol ediliyor…</span></div>
{:else if canBrowse === false}
  <div class="center-wrap">
    <div class="unsupported glass-panel">
      <p>Bu host için dosya yöneticisi henüz desteklenmiyor.</p>
      <p class="muted">Bu host çok adımlı bir bağlantı betiği kullanıyor. Önce bir terminal sekmesinden bağlan.</p>
      {#if checkError}<p class="error-text">{checkError}</p>{/if}
    </div>
  </div>
{:else}
  <div class="files-shell">
    <section class="pane">
      <div class="pane-header">
        <HardDrive size={14} strokeWidth={2} />
        <span class="pane-title">Yerel</span>
        <span class="spacer"></span>
        <button type="button" class="icon-btn" onclick={localUp} title="Üst dizin" disabled={localSegments().length === 0}>
          <ArrowUp size={14} strokeWidth={2} />
        </button>
        <button type="button" class="icon-btn" onclick={loadLocal} title="Yenile">
          <RefreshCw size={14} strokeWidth={2} />
        </button>
      </div>
      <div class="breadcrumb">
        <button type="button" class="crumb" onclick={() => jumpLocal("/")}>/</button>
        {#each localSegments() as seg (seg.path)}
          <span class="crumb-sep">/</span>
          <button type="button" class="crumb" onclick={() => jumpLocal(seg.path)}>{seg.name}</button>
        {/each}
      </div>
      <div class="list">
        {#if localLoading}
          <p class="muted list-msg">Yükleniyor…</p>
        {:else if localError}
          <p class="error-text list-msg">{localError}</p>
        {:else}
          {#each localEntries as entry (entry.name)}
            <div
              class="list-row entry-row {selectedLocal?.name === entry.name ? 'selected' : ''}"
              onclick={() => openLocalDir(entry)}
              role="button"
              tabindex="0"
              onkeydown={(e) => e.key === "Enter" && openLocalDir(entry)}
            >
              {#if entry.isDir}<Folder size={14} strokeWidth={1.75} class="row-icon" />
              {:else if entry.isLink}<Link2 size={14} strokeWidth={1.75} class="row-icon" />
              {:else}<File size={14} strokeWidth={1.75} class="row-icon" />{/if}
              <span class="entry-name">{entry.name}</span>
              <span class="spacer"></span>
              {#if !entry.isDir}<span class="muted entry-meta">{formatSize(entry.size)}</span>{/if}
            </div>
          {/each}
        {/if}
      </div>
      <div class="pane-footer">
        <button type="button" onclick={uploadSelected} disabled={!selectedLocal} class="primary">
          <Upload size={13} strokeWidth={2} />
          Yükle
        </button>
      </div>
    </section>

    <section class="pane">
      <div class="pane-header">
        <Server size={14} strokeWidth={2} />
        <span class="pane-title">Uzak · {hostName}</span>
        <span class="spacer"></span>
        <button type="button" class="icon-btn" onclick={openMkdir} title="Yeni klasör">
          <FolderPlus size={14} strokeWidth={2} />
        </button>
        <button type="button" class="icon-btn" onclick={remoteUp} title="Üst dizin" disabled={remoteSegs.length === 0}>
          <ArrowUp size={14} strokeWidth={2} />
        </button>
        <button type="button" class="icon-btn" onclick={loadRemote} title="Yenile">
          <RefreshCw size={14} strokeWidth={2} />
        </button>
      </div>
      <div class="breadcrumb">
        <button type="button" class="crumb" onclick={() => jumpRemote(0)}>~</button>
        {#each remoteSegs as seg, i (i)}
          <span class="crumb-sep">/</span>
          <button type="button" class="crumb" onclick={() => jumpRemote(i + 1)}>{seg}</button>
        {/each}
      </div>
      <div
        id={dropZoneId}
        data-file-drop-target
        class="list drop-target {dropActive ? 'file-drop-target-active' : ''}"
        ondragenter={() => (dropActive = true)}
        ondragleave={() => (dropActive = false)}
      >
        {#if remoteLoading}
          <p class="muted list-msg">Yükleniyor…</p>
        {:else if remoteError}
          <p class="error-text list-msg">{remoteError}</p>
        {:else if remoteEntries.length === 0}
          <p class="muted list-msg">Klasör boş. Finder'dan dosya sürükleyip bırakabilirsin.</p>
        {:else}
          {#each remoteEntries as entry (entry.name)}
            <div
              class="list-row entry-row"
              onclick={() => openRemoteDir(entry)}
              role="button"
              tabindex="0"
              onkeydown={(e) => e.key === "Enter" && openRemoteDir(entry)}
            >
              {#if entry.isDir}<Folder size={14} strokeWidth={1.75} class="row-icon" />
              {:else if entry.isLink}<Link2 size={14} strokeWidth={1.75} class="row-icon" />
              {:else}<File size={14} strokeWidth={1.75} class="row-icon" />{/if}
              <span class="entry-name">{entry.name}</span>
              {#if entry.isLink && entry.linkTarget}<span class="muted entry-meta">→ {entry.linkTarget}</span>{/if}
              <span class="spacer"></span>
              {#if !entry.isDir}<span class="muted entry-meta">{formatSize(entry.size)}</span>{/if}
              <span class="muted entry-meta">{entry.modTime}</span>
              <span class="row-actions">
                {#if !entry.isDir}
                  <button
                    type="button"
                    class="icon-btn"
                    onclick={(e) => { e.stopPropagation(); downloadRemote(entry); }}
                    title="İndir"
                  >
                    <Download size={13} strokeWidth={2} />
                  </button>
                {/if}
                <button type="button" class="icon-btn" onclick={(e) => { e.stopPropagation(); openRename(entry); }} title="Yeniden adlandır">
                  <Pencil size={13} strokeWidth={2} />
                </button>
                <button
                  type="button"
                  class="icon-btn danger"
                  onclick={(e) => { e.stopPropagation(); deleteRemote(entry); }}
                  title="Sil"
                >
                  <Trash2 size={13} strokeWidth={2} />
                </button>
              </span>
            </div>
          {/each}
        {/if}
      </div>
      <div class="pane-footer">
        <button type="button" onclick={pickAndUpload}>
          <FolderOpen size={13} strokeWidth={2} />
          Dosya seç ve yükle
        </button>
      </div>
    </section>

    {#if transfers.size > 0}
      <div class="transfers glass-panel">
        {#each [...transfers.entries()] as [id, t] (id)}
          <div class="transfer-row">
            <span class="transfer-name">{t.name}</span>
            {#if t.error}
              <span class="error-text">{t.error}</span>
            {:else if t.finished}
              <span class="muted">Tamamlandı</span>
            {:else}
              <div class="transfer-bar">
                <div class="transfer-bar-fill" style:width="{t.total ? (t.done / t.total) * 100 : 0}%"></div>
              </div>
              <span class="muted transfer-pct">{t.total ? Math.round((t.done / t.total) * 100) : 0}%</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<dialog bind:this={mkdirDialog} class="glass-panel">
  <h3>Yeni Klasör</h3>
  <form onsubmit={submitMkdir}>
    <div class="field">
      <label for="mkdir-name">Ad</label>
      <input id="mkdir-name" type="text" bind:value={mkdirName} required autofocus />
    </div>
    <div class="row end">
      <button type="button" onclick={() => mkdirDialog.close()}>Vazgeç</button>
      <button type="submit" class="primary">Oluştur</button>
    </div>
  </form>
</dialog>

<dialog bind:this={renameDialog} class="glass-panel">
  <h3>Yeniden Adlandır</h3>
  <form onsubmit={submitRename}>
    <div class="field">
      <label for="rename-name">Ad</label>
      <input id="rename-name" type="text" bind:value={renameValue} required autofocus />
    </div>
    <div class="row end">
      <button type="button" onclick={() => renameDialog.close()}>Vazgeç</button>
      <button type="submit" class="primary">Kaydet</button>
    </div>
  </form>
</dialog>

<style>
  .center-wrap {
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
  }
  .unsupported {
    padding: 20px 24px;
    max-width: 420px;
    text-align: center;
  }
  .unsupported p {
    margin: 0 0 6px;
  }
  .files-shell {
    height: 100%;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1px;
    background: var(--chrome-border);
    position: relative;
  }
  .pane {
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--app-bg);
  }
  .pane-header {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 10px;
    flex-shrink: 0;
    border-bottom: 1px solid var(--chrome-border);
  }
  .pane-title {
    font-size: 12px;
    font-weight: 600;
  }
  .spacer {
    flex: 1;
  }
  .breadcrumb {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 6px 10px;
    flex-shrink: 0;
    overflow-x: auto;
    white-space: nowrap;
  }
  .crumb {
    border: none;
    background: transparent;
    padding: 2px 5px;
    font-size: 11.5px;
    color: var(--text-muted);
  }
  .crumb:hover {
    background: var(--row-hover);
    color: var(--text);
  }
  .crumb-sep {
    color: var(--text-faint);
    font-size: 11px;
  }
  .list {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
    padding: 4px 6px;
  }
  .list-msg {
    padding: 12px;
    font-size: 12px;
  }
  .entry-row {
    cursor: default;
    padding: 4px 8px;
  }
  :global(.row-icon) {
    color: var(--text-muted);
    flex-shrink: 0;
  }
  .entry-name {
    font-size: 12.5px;
  }
  .entry-meta {
    font-size: 11px;
    white-space: nowrap;
  }
  .row-actions {
    display: flex;
    gap: 2px;
    opacity: 0;
  }
  .entry-row:hover .row-actions {
    opacity: 1;
  }
  .pane-footer {
    padding: 8px 10px;
    border-top: 1px solid var(--chrome-border);
    flex-shrink: 0;
    display: flex;
    justify-content: flex-end;
  }
  .drop-target {
    transition:
      background-color 0.15s ease,
      box-shadow 0.15s ease;
  }
  .drop-target.file-drop-target-active {
    background: rgba(0, 122, 255, 0.08);
    box-shadow: inset 0 0 0 2px var(--accent);
  }
  .transfers {
    position: absolute;
    bottom: 12px;
    right: 12px;
    width: 280px;
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    box-shadow: var(--surface-shadow);
  }
  .transfer-row {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 11.5px;
  }
  .transfer-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .transfer-bar {
    width: 60px;
    height: 4px;
    border-radius: 2px;
    background: var(--surface-bg-strong);
    overflow: hidden;
  }
  .transfer-bar-fill {
    height: 100%;
    background: var(--accent);
  }
  .transfer-pct {
    width: 30px;
    text-align: right;
  }
</style>
