<script lang="ts">
  import { PasswordService, TerminalService } from "../bindings/go-ssh-ui";
  import { Server, KeyRound, Lock, Menu, Home, X, Megaphone, FolderOpen } from "@lucide/svelte";
  import { SvelteSet } from "svelte/reactivity";
  import type { Host } from "../bindings/go-ssh/config/models";
  import UnlockScreen from "./views/UnlockScreen.svelte";
  import HostsView from "./views/HostsView.svelte";
  import PasswordsView from "./views/PasswordsView.svelte";
  import TerminalPane, { type TerminalStatus } from "./components/TerminalPane.svelte";
  import FilesPane from "./components/FilesPane.svelte";

  let unlocked: boolean | null = $state(null);
  // "Home" is a pinned, non-closable tab showing the host tree / password
  // manager - conceptually the start page. Connecting to a host opens an
  // additional real tab next to it; switching tabs never disconnects
  // anything, since every TerminalPane stays mounted (just hidden) once
  // opened - only closing a tab's × actually tears down its session.
  let sidebarNav: "hosts" | "passwords" = $state("hosts");
  let sidebarCollapsed = $state(false);

  type TerminalTab = {
    id: string;
    categoryPath: string[];
    host: Host;
    status: TerminalStatus;
    sessionId?: string;
  };
  let terminalTabs: TerminalTab[] = $state([]);

  type FileTab = { id: string; categoryPath: string[]; host: Host };
  let fileTabs: FileTab[] = $state([]);

  let activeTabId: string = $state("home");

  // Broadcast: type once, send to every selected terminal tab's session -
  // each target TerminalPane stays mounted even while hidden, so its own
  // "terminal:output" subscription still writes the reply into its buffer.
  let broadcastOpen = $state(false);
  let broadcastTargets = new SvelteSet<string>();
  let broadcastText = $state("");

  async function refreshStatus() {
    const status = await PasswordService.Status();
    unlocked = status.unlocked;
  }
  refreshStatus();

  async function lock() {
    await PasswordService.Lock();
    unlocked = false;
  }

  function selectNav(nav: "hosts" | "passwords") {
    sidebarNav = nav;
    activeTabId = "home";
  }

  function connectHost(categoryPath: string[], host: Host) {
    const id = crypto.randomUUID();
    terminalTabs = [...terminalTabs, { id, categoryPath, host, status: "connecting" }];
    activeTabId = id;
    if (broadcastOpen) broadcastTargets.add(id);
  }

  function openFiles(categoryPath: string[], host: Host) {
    const existing = fileTabs.find((t) => t.host.name === host.name && t.categoryPath.join("/") === categoryPath.join("/"));
    if (existing) {
      activeTabId = existing.id;
      return;
    }
    const id = crypto.randomUUID();
    fileTabs = [...fileTabs, { id, categoryPath, host }];
    activeTabId = id;
  }

  function setTabStatus(id: string, status: TerminalStatus) {
    terminalTabs = terminalTabs.map((t) => (t.id === id ? { ...t, status } : t));
  }

  function setTabSessionId(id: string, sessionId: string) {
    terminalTabs = terminalTabs.map((t) => (t.id === id ? { ...t, sessionId } : t));
  }

  function closeTab(id: string) {
    terminalTabs = terminalTabs.filter((t) => t.id !== id);
    fileTabs = fileTabs.filter((t) => t.id !== id);
    broadcastTargets.delete(id);
    if (activeTabId === id) activeTabId = "home";
  }

  function toggleBroadcast() {
    if (!broadcastOpen) {
      for (const t of terminalTabs) broadcastTargets.add(t.id);
    }
    broadcastOpen = !broadcastOpen;
  }

  function toggleBroadcastTarget(id: string) {
    if (broadcastTargets.has(id)) broadcastTargets.delete(id);
    else broadcastTargets.add(id);
  }

  async function sendBroadcast(e: Event) {
    e.preventDefault();
    const text = broadcastText;
    if (!text) return;
    const targets = terminalTabs.filter((t) => broadcastTargets.has(t.id) && t.sessionId);
    broadcastText = "";
    await Promise.all(targets.map((t) => TerminalService.Write(t.sessionId!, text + "\r")));
  }
</script>

{#if unlocked === null}
  <div class="boot-wrap">
    <p class="muted">Yükleniyor…</p>
  </div>
{:else if unlocked === false}
  <UnlockScreen onUnlocked={() => (unlocked = true)} />
{:else}
  <div class="shell" style:grid-template-columns={sidebarCollapsed ? "44px 1fr" : "var(--sidebar-width) 1fr"}>
    <!-- One toolbar row spans the full window width so the sidebar's and the
         content's top edges are guaranteed to align. -->
    <div class="toolbar chrome chrome-border-bottom">
      <div class="toolbar-side"></div>
      <div class="toolbar-main chrome-border-left">
        <div class="tab-strip">
          <button type="button" class="tab {activeTabId === 'home' ? 'active' : ''}" onclick={() => (activeTabId = "home")}>
            <Home size={13} strokeWidth={2} />
            <span>Başlangıç</span>
          </button>
          {#each terminalTabs as tab (tab.id)}
            <div class="tab {activeTabId === tab.id ? 'active' : ''}">
              <button type="button" class="tab-select" onclick={() => (activeTabId = tab.id)}>
                <span class="status-dot status-{tab.status}"></span>
                <span>{tab.host.name}</span>
              </button>
              <button type="button" class="icon-btn tab-close" onclick={() => closeTab(tab.id)} title="Kapat">
                <X size={12} strokeWidth={2} />
              </button>
            </div>
          {/each}
          {#each fileTabs as tab (tab.id)}
            <div class="tab {activeTabId === tab.id ? 'active' : ''}">
              <button type="button" class="tab-select" onclick={() => (activeTabId = tab.id)}>
                <FolderOpen size={12} strokeWidth={2} />
                <span>{tab.host.name}</span>
              </button>
              <button type="button" class="icon-btn tab-close" onclick={() => closeTab(tab.id)} title="Kapat">
                <X size={12} strokeWidth={2} />
              </button>
            </div>
          {/each}
        </div>
        {#if terminalTabs.length > 0}
          <button
            type="button"
            class="icon-btn broadcast-toggle {broadcastOpen ? 'active' : ''}"
            onclick={toggleBroadcast}
            title="Toplu komut"
          >
            <Megaphone size={15} strokeWidth={2} />
          </button>
        {/if}
      </div>
    </div>

    <!-- Always rendered (never removed) so the hamburger toggle - the only
         way back out of the collapsed state - never disappears along with
         the rest of the sidebar. Collapsing just narrows the grid column
         and hides everything below the toggle. -->
    <nav class="sidebar chrome chrome-border-right">
      <div class="sidebar-header">
        <button type="button" class="icon-btn hamburger-btn" onclick={() => (sidebarCollapsed = !sidebarCollapsed)} title="Kenar çubuğu">
          <Menu size={16} strokeWidth={2} />
        </button>
        {#if !sidebarCollapsed}
          <span class="brand">go-ssh-ui</span>
        {/if}
      </div>
      {#if !sidebarCollapsed}
        <div class="sidebar-box glass-panel">
          <button
            type="button"
            class="list-row nav-row {sidebarNav === 'hosts' && activeTabId === 'home' ? 'selected' : ''}"
            onclick={() => selectNav("hosts")}
          >
            <Server size={15} strokeWidth={2} />
            <span>Hostlar</span>
          </button>
          <button
            type="button"
            class="list-row nav-row {sidebarNav === 'passwords' && activeTabId === 'home' ? 'selected' : ''}"
            onclick={() => selectNav("passwords")}
          >
            <KeyRound size={15} strokeWidth={2} />
            <span>Şifreler</span>
          </button>
        </div>
        <div class="spacer"></div>
        <button type="button" class="list-row nav-row" onclick={lock}>
          <Lock size={15} strokeWidth={2} />
          <span>Kilitle</span>
        </button>
      {/if}
    </nav>

    <main class="content">
      <div class="tab-pane" style:display={activeTabId === "home" ? "block" : "none"}>
        {#if sidebarNav === "hosts"}
          <HostsView onConnectHost={connectHost} onOpenFiles={openFiles} />
        {:else}
          <PasswordsView />
        {/if}
      </div>
      {#each terminalTabs as tab (tab.id)}
        <div class="tab-pane" style:display={activeTabId === tab.id ? "block" : "none"}>
          <TerminalPane
            categoryPath={tab.categoryPath}
            hostName={tab.host.name}
            onStatusChange={(status) => setTabStatus(tab.id, status)}
            onSessionId={(sessionId) => setTabSessionId(tab.id, sessionId)}
          />
        </div>
      {/each}
      {#each fileTabs as tab (tab.id)}
        <div class="tab-pane" style:display={activeTabId === tab.id ? "block" : "none"}>
          <FilesPane categoryPath={tab.categoryPath} hostName={tab.host.name} />
        </div>
      {/each}
    </main>

    {#if broadcastOpen && terminalTabs.length > 0}
      <div class="broadcast-bar chrome chrome-border-top">
        <div class="broadcast-targets">
          <span class="muted broadcast-label">Hedef:</span>
          {#each terminalTabs as tab (tab.id)}
            <button
              type="button"
              class="chip {broadcastTargets.has(tab.id) ? 'active' : ''}"
              onclick={() => toggleBroadcastTarget(tab.id)}
            >
              {tab.host.name}
            </button>
          {/each}
        </div>
        <form class="broadcast-input-row" onsubmit={sendBroadcast}>
          <input type="text" placeholder="Seçili sekmelere komut gönder…" bind:value={broadcastText} />
          <button type="submit" class="primary" disabled={broadcastTargets.size === 0 || !broadcastText}>Gönder</button>
        </form>
      </div>
    {/if}
  </div>
{/if}

<style>
  .boot-wrap {
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .shell {
    height: 100vh;
    display: grid;
    grid-template-rows: var(--toolbar-height) minmax(0, 1fr) auto;
    grid-template-areas: "toolbar toolbar" "sidebar content" "broadcast broadcast";
    transition: grid-template-columns 0.15s ease;
  }
  .toolbar {
    grid-area: toolbar;
    display: flex;
    align-items: center;
    -webkit-app-region: drag;
  }
  .toolbar-side {
    width: var(--sidebar-width);
    flex-shrink: 0;
    display: flex;
    align-items: center;
    padding-left: 74px; /* clears the traffic-light inset */
    -webkit-app-region: drag;
  }
  .brand {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-muted);
  }
  .toolbar-main {
    flex: 1;
    height: 100%;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 8px;
    min-width: 0;
  }
  .tab-strip {
    display: flex;
    align-items: center;
    gap: 4px;
    height: 100%;
    flex: 1;
    min-width: 0;
    overflow-x: auto;
    -webkit-app-region: no-drag;
  }
  .broadcast-toggle {
    flex-shrink: 0;
    -webkit-app-region: no-drag;
  }
  .broadcast-toggle.active {
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .tab {
    display: flex;
    align-items: center;
    height: 30px;
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    flex-shrink: 0;
  }
  .tab:hover {
    background: var(--row-hover);
  }
  .tab.active {
    background: var(--surface-bg-strong);
    color: var(--text);
  }
  .tab-select {
    display: flex;
    align-items: center;
    gap: 6px;
    border: none;
    background: transparent;
    padding: 0 8px;
    height: 100%;
    font-size: 12.5px;
    color: inherit;
  }
  .tab-close {
    width: 20px;
    height: 20px;
    margin-right: 4px;
  }
  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
    background: var(--text-faint);
  }
  .status-dot.status-connecting {
    background: #f5a623;
  }
  .status-dot.status-connected {
    background: #2ecc71;
  }
  .status-dot.status-closed {
    background: var(--danger);
  }
  .sidebar {
    grid-area: sidebar;
    display: flex;
    flex-direction: column;
    padding: 8px;
    gap: 8px;
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
  }
  .sidebar-header {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }
  .hamburger-btn {
    flex-shrink: 0;
  }
  .sidebar-box {
    padding: 6px;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .nav-row {
    border: none;
    background: transparent;
    width: 100%;
    text-align: left;
    justify-content: flex-start;
    font-size: 13px;
  }
  .nav-row.selected :global(svg) {
    color: var(--row-selected-text);
  }
  .spacer {
    flex: 1;
  }
  .content {
    grid-area: content;
    overflow: hidden;
    min-height: 0;
    position: relative;
  }
  .tab-pane {
    height: 100%;
  }
  .broadcast-bar {
    grid-area: broadcast;
    padding: 8px 14px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .broadcast-targets {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }
  .broadcast-label {
    font-size: 11px;
  }
  .chip {
    padding: 2px 10px;
    font-size: 11.5px;
    border-radius: 999px;
    color: var(--text-muted);
  }
  .chip.active {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
  }
  .broadcast-input-row {
    display: flex;
    gap: 8px;
  }
  .broadcast-input-row input {
    flex: 1;
  }
</style>
