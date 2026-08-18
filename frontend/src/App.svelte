<script lang="ts">
  import { onDestroy } from "svelte";
  import { Events, Window as WailsWindow } from "@wailsio/runtime";
  import { PasswordService, TerminalService, HotkeyWindowService } from "../bindings/go-ssh-ui";
  import type { HotkeyWindowSettings } from "../bindings/go-ssh-ui";
  import { Server, KeyRound, Lock, Menu, Home, X, Megaphone, FolderOpen, Settings, SquareTerminal, Plus } from "@lucide/svelte";
  import { SvelteSet } from "svelte/reactivity";
  import type { Host } from "../bindings/go-ssh/config/models";
  import UnlockScreen from "./views/UnlockScreen.svelte";
  import HostsView from "./views/HostsView.svelte";
  import PasswordsView from "./views/PasswordsView.svelte";
  import SettingsView from "./views/SettingsView.svelte";
  import TerminalPane, { type TerminalStatus } from "./components/TerminalPane.svelte";
  import FilesPane from "./components/FilesPane.svelte";

  // Set only for the hotkey window (main.go's second window, ?mode=hotkey -
  // see main.ts). Content is mostly identical to the main window - same tab
  // system, same Başlangıç tab - minus the sidebar (no hamburger/Hostlar/
  // Şifreler/Ayarlar/Kilitle - just tabs + Başlangıç, per user request), and
  // with a CSS-driven background tint instead of the main window's --app-bg,
  // since the hotkey window's color/opacity are its own separate setting
  // (HotkeyWindowService) painted over whatever native Backdrop material
  // main.go gave that window - see applyHotkeyAppearance below.
  let { hotkeyMode = false }: { hotkeyMode?: boolean } = $props();
  if (hotkeyMode) {
    document.documentElement.classList.add("hotkey-window");

    const hex2rgb = (hex: string) => {
      const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
      if (!m) return "20, 20, 26";
      return `${parseInt(m[1].slice(0, 2), 16)}, ${parseInt(m[1].slice(2, 4), 16)}, ${parseInt(m[1].slice(4, 6), 16)}`;
    };
    const applyHotkeyAppearance = (s: HotkeyWindowSettings) => {
      const root = document.documentElement.style;
      root.setProperty("--hotkey-bg-rgb", hex2rgb(s.bgColor));
      root.setProperty("--hotkey-bg-alpha", String(s.opacity / 100));
    };
    HotkeyWindowService.GetSettings().then(applyHotkeyAppearance).catch(() => {});
    // Settings can be changed from either window now (Ayarlar is reachable
    // from the main window even though this one hides its own sidebar) -
    // this keeps the two in sync regardless of which one made the change.
    onDestroy(Events.On("hotkey:settings-changed", (ev) => applyHotkeyAppearance(ev.data as HotkeyWindowSettings)));
  }

  let unlocked: boolean | null = $state(null);
  // "Home" is a pinned, non-closable tab showing the host tree / password
  // manager - conceptually the start page. Connecting to a host opens an
  // additional real tab next to it; switching tabs never disconnects
  // anything, since every TerminalPane stays mounted (just hidden) once
  // opened - only closing a tab's × actually tears down its session.
  let sidebarNav: "hosts" | "passwords" | "settings" = $state("hosts");
  let sidebarCollapsed = $state(false);
  let hostsViewRef: ReturnType<typeof HostsView> | undefined = $state();

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

  // A local tab is a plain bash/zsh shell on the machine go-ssh-ui itself
  // runs on - no host, no SSH. Kept in its own array (rather than folded
  // into TerminalTab) since it has no Host/categoryPath to carry.
  type LocalTab = { id: string; status: TerminalStatus; sessionId?: string };
  let localTabs: LocalTab[] = $state([]);

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

  function selectNav(nav: "hosts" | "passwords" | "settings") {
    sidebarNav = nav;
    activeTabId = "home";
  }

  // Faz 5 "Kayıt": RecordReviewDialog saves the host directly (not through
  // HostsView's own forms), so if HostsView was already mounted on "hosts"
  // nav, switching activeTabId back to it alone doesn't remount it -
  // {#if sidebarNav === "hosts"} only remounts on an actual nav change, and
  // sidebarNav might already be "hosts". Explicitly reload it too so the
  // new host shows up immediately either way.
  function onRecordedHostSaved() {
    selectNav("hosts");
    hostsViewRef?.reload();
  }

  function connectHost(categoryPath: string[], host: Host) {
    const id = crypto.randomUUID();
    terminalTabs = [...terminalTabs, { id, categoryPath, host, status: "connecting" }];
    activeTabId = id;
    if (broadcastOpen) broadcastTargets.add(id);
  }

  function openLocalTerminal() {
    const id = crypto.randomUUID();
    localTabs = [...localTabs, { id, status: "connecting" }];
    activeTabId = id;
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

  function setLocalTabStatus(id: string, status: TerminalStatus) {
    localTabs = localTabs.map((t) => (t.id === id ? { ...t, status } : t));
  }

  function setLocalTabSessionId(id: string, sessionId: string) {
    localTabs = localTabs.map((t) => (t.id === id ? { ...t, sessionId } : t));
  }

  // Same left-to-right order as the tab-strip markup below, so both Cmd+
  // Left/Right cycling and closeTab's "select my left neighbor" land on the
  // tab the user visually expects.
  function allTabIds(): string[] {
    return ["home", ...terminalTabs.map((t) => t.id), ...fileTabs.map((t) => t.id), ...localTabs.map((t) => t.id)];
  }

  function closeTab(id: string) {
    if (activeTabId === id) {
      const ids = allTabIds();
      const idx = ids.indexOf(id);
      activeTabId = idx > 0 ? ids[idx - 1] : "home";
    }
    terminalTabs = terminalTabs.filter((t) => t.id !== id);
    fileTabs = fileTabs.filter((t) => t.id !== id);
    localTabs = localTabs.filter((t) => t.id !== id);
    broadcastTargets.delete(id);
  }

  function cycleTab(direction: 1 | -1) {
    const ids = allTabIds();
    const idx = ids.indexOf(activeTabId);
    if (idx === -1) return;
    activeTabId = ids[(idx + direction + ids.length) % ids.length];
  }

  // Hotkey window's height only ever moves from its bottom edge (Y is
  // pinned to the screen top - see hotkeywindowservice.go's
  // positionOnCursorScreen), so Cmd+Down/Up just nudges height the same way
  // dragging that edge down/up would. No clamping needed here - the native
  // window already carries the SetMinSize/SetMaxSize that drag-resize
  // respects, so a SetSize past either bound is clamped for free. A real
  // resize also fires the same WindowDidResize Go's side already debounces
  // into a persisted save, so this needs no Go changes at all.
  const HOTKEY_HEIGHT_STEP = 24;
  function adjustHotkeyHeight(delta: number) {
    WailsWindow.Size()
      .then(({ width, height }) => WailsWindow.SetSize(width, height + delta))
      .catch(() => {});
  }

  // Cmd+Left/Right/W/T tab management (+ Cmd+Down/Up hotkey-window height,
  // hotkeyMode only) - both windows (main + hotkey) mount this same
  // component, so one listener covers both, see hotkeyMode above.
  // Registered on window with capture:true (not `<svelte:window>`, which
  // only attaches in the bubble phase) so it runs before xterm.js's own
  // keydown handler on its hidden textarea gets a chance to see the event.
  function handleGlobalKeydown(e: KeyboardEvent) {
    if (!e.metaKey || e.ctrlKey || e.altKey) return;
    const target = document.activeElement;
    const isTextEditingContext =
      target instanceof HTMLElement &&
      !target.classList.contains("xterm-helper-textarea") &&
      (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable);
    switch (e.key) {
      case "ArrowRight":
        if (isTextEditingContext) return;
        e.preventDefault();
        cycleTab(1);
        break;
      case "ArrowLeft":
        if (isTextEditingContext) return;
        e.preventDefault();
        cycleTab(-1);
        break;
      case "ArrowDown":
        if (!hotkeyMode || isTextEditingContext) return;
        e.preventDefault();
        adjustHotkeyHeight(HOTKEY_HEIGHT_STEP);
        break;
      case "ArrowUp":
        if (!hotkeyMode || isTextEditingContext) return;
        e.preventDefault();
        adjustHotkeyHeight(-HOTKEY_HEIGHT_STEP);
        break;
      case "w":
      case "W":
        if (!e.shiftKey) {
          e.preventDefault();
          if (activeTabId !== "home") closeTab(activeTabId);
        }
        break;
      case "t":
      case "T":
        if (!e.shiftKey) {
          e.preventDefault();
          openLocalTerminal();
        }
        break;
    }
  }
  window.addEventListener("keydown", handleGlobalKeydown, true);
  onDestroy(() => window.removeEventListener("keydown", handleGlobalKeydown, true));

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
  <div
    class="shell"
    style:grid-template-columns={hotkeyMode ? "1fr" : sidebarCollapsed ? "44px 1fr" : "var(--sidebar-width) 1fr"}
    style:grid-template-areas={hotkeyMode
      ? '"toolbar" "content" "broadcast"'
      : '"toolbar toolbar" "sidebar content" "broadcast broadcast"'}
  >
    <!-- One toolbar row spans the full window width so the sidebar's and the
         content's top edges are guaranteed to align. -->
    <div class="toolbar chrome chrome-border-bottom">
      <div class="toolbar-side" class:hotkey-mode={hotkeyMode}></div>
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
          {#each localTabs as tab (tab.id)}
            <div class="tab {activeTabId === tab.id ? 'active' : ''}">
              <button type="button" class="tab-select" onclick={() => (activeTabId = tab.id)}>
                <SquareTerminal size={12} strokeWidth={2} />
                <span>Terminal</span>
              </button>
              <button type="button" class="icon-btn tab-close" onclick={() => closeTab(tab.id)} title="Kapat">
                <X size={12} strokeWidth={2} />
              </button>
            </div>
          {/each}
        </div>
        <button type="button" class="icon-btn new-terminal-btn" onclick={openLocalTerminal} title="Yeni terminal (bash/zsh)">
          <Plus size={15} strokeWidth={2} />
        </button>
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

    <!-- Hidden entirely in the hotkey window (per user request: just the tab
         system + Başlangıç there, no hamburger/Hostlar/Şifreler/Ayarlar/
         Kilitle) - sidebarNav stays at its "hosts" default with nothing to
         change it, so the Başlangıç tab-pane below just always renders
         HostsView. Otherwise (main window) always rendered - never removed,
         so the hamburger toggle - the only way back out of the collapsed
         state - never disappears along with the rest of the sidebar.
         Collapsing just narrows the grid column and hides everything below
         the toggle. -->
    {#if !hotkeyMode}
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
          <button
            type="button"
            class="list-row nav-row {sidebarNav === 'settings' && activeTabId === 'home' ? 'selected' : ''}"
            onclick={() => selectNav("settings")}
          >
            <Settings size={15} strokeWidth={2} />
            <span>Ayarlar</span>
          </button>
          <button type="button" class="list-row nav-row" onclick={lock}>
            <Lock size={15} strokeWidth={2} />
            <span>Kilitle</span>
          </button>
        {/if}
      </nav>
    {/if}

    <main class="content">
      <div class="tab-pane" style:display={activeTabId === "home" ? "block" : "none"}>
        {#if sidebarNav === "hosts"}
          <HostsView bind:this={hostsViewRef} active={activeTabId === "home"} onConnectHost={connectHost} onOpenFiles={openFiles} />
        {:else if sidebarNav === "passwords"}
          <PasswordsView />
        {:else}
          <SettingsView />
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
      {#each localTabs as tab (tab.id)}
        <div class="tab-pane" style:display={activeTabId === tab.id ? "block" : "none"}>
          <TerminalPane
            local
            hostName="Terminal"
            onStatusChange={(status) => setLocalTabStatus(tab.id, status)}
            onSessionId={(sessionId) => setLocalTabSessionId(tab.id, sessionId)}
            onHostSaved={onRecordedHostSaved}
          />
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
    /* grid-template-areas is set inline (style:grid-template-areas above) -
       differs between the main window (toolbar/sidebar/content/broadcast)
       and the hotkey window (no sidebar column at all, see hotkeyMode). */
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
  /* The hotkey window is Frameless (main.go) - no traffic lights to clear
     space for, and (per user request) no sidebar to align widths with
     either, so this whole reserved strip collapses to nothing, letting the
     tab strip start flush left. */
  .toolbar-side.hotkey-mode {
    width: 0;
    padding-left: 0;
  }

  /* Scoped to the hotkey window only (App.svelte adds this class to <html>
     when hotkeyMode is true) - NOT a bare `:global(html, body)` rule, since
     both windows load the exact same compiled stylesheet and an unscoped
     override here would blank out the *main* window's background too
     (regression fixed by this scoping). --hotkey-bg-rgb/-alpha are set here
     as sane pre-JS defaults and then overridden live (higher-specificity
     inline style) by applyHotkeyAppearance above, from HotkeyWindowService's
     saved color/opacity - painted over whatever native Backdrop material
     main.go gave this window (blurred/liquid-glass/crisp), exactly like the
     main window layers --app-bg over its own Backdrop choice. */
  :global(html.hotkey-window) {
    --hotkey-bg-rgb: 20, 20, 26;
    --hotkey-bg-alpha: 0.9;
  }
  :global(html.hotkey-window),
  :global(html.hotkey-window body) {
    background: rgba(var(--hotkey-bg-rgb), var(--hotkey-bg-alpha)) !important;
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
  .new-terminal-btn {
    flex-shrink: 0;
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
