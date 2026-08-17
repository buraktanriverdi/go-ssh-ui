<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Events } from "@wailsio/runtime";
  import { Terminal } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";
  import "@xterm/xterm/css/xterm.css";
  import { TerminalService, RecordService } from "../../bindings/go-ssh-ui";
  import type { Prompt } from "../../bindings/go-ssh-ui/internal/sshengine/models";
  import type { Snapshot } from "../../bindings/go-ssh-ui/internal/record/models";
  import PromptDialog from "./PromptDialog.svelte";
  import HostKeyDialog from "./HostKeyDialog.svelte";
  import RecordReviewDialog from "./RecordReviewDialog.svelte";

  export type TerminalStatus = "connecting" | "connected" | "closed";

  let {
    categoryPath = [],
    hostName,
    local = false,
    onStatusChange,
    onSessionId,
    onHostSaved,
  }: {
    categoryPath?: string[];
    hostName: string;
    local?: boolean;
    onStatusChange?: (status: TerminalStatus) => void;
    onSessionId?: (id: string) => void;
    onHostSaved?: () => void;
  } = $props();

  let container: HTMLDivElement;
  let term: Terminal;
  let fitAddon: FitAddon;
  let resizeObserver: ResizeObserver;

  let sessionId: string | null = $state(null);
  let status: TerminalStatus = $state("connecting");
  let closeError: string | null = $state(null);
  let pendingPrompt: Prompt | null = $state(null);

  // Faz 5 "Kayıt" - only offered on local shell tabs (see PLAN.md's scope
  // note: it's for recording a user manually running the real `ssh`
  // binary, not an already go-ssh-ui-managed connection). recordLabel is
  // the live "prompt bekleniyor"/"yakalandı" status text driven by
  // "record:event"; recordSnapshot is set once Stop() comes back with an
  // actually-detected command, which opens the review dialog.
  let recording = $state(false);
  let recordLabel: string | null = $state(null);
  let recordSnapshot: Snapshot | null = $state(null);

  const unsubs: (() => void)[] = [];

  function setStatus(next: TerminalStatus) {
    status = next;
    onStatusChange?.(next);
  }

  onMount(() => {
    term = new Terminal({
      fontFamily: "ui-monospace, SF Mono, Menlo, monospace",
      fontSize: 13,
      cursorBlink: true,
      allowTransparency: true,
      theme: { background: "#00000000" },
    });
    fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(container);
    fitAddon.fit();

    term.onData((data) => {
      if (sessionId) TerminalService.Write(sessionId, data).catch(() => {});
    });

    resizeObserver = new ResizeObserver(() => {
      fitAddon.fit();
      if (sessionId) TerminalService.Resize(sessionId, term.cols, term.rows).catch(() => {});
    });
    resizeObserver.observe(container);

    connect();
  });

  async function connect() {
    // Subscriptions go up before the Connect call is even sent, so no event
    // the Go side emits once it starts working can be missed.
    unsubs.push(
      Events.On("terminal:output", (ev) => {
        if (ev.data.sessionId === sessionId) term.write(ev.data.data);
      }),
    );
    unsubs.push(
      Events.On("terminal:prompt", (ev) => {
        if (ev.data.sessionId === sessionId) pendingPrompt = ev.data;
      }),
    );
    unsubs.push(
      Events.On("terminal:connected", (ev) => {
        if (ev.data.sessionId === sessionId) {
          setStatus("connected");
          term.focus();
        }
      }),
    );
    unsubs.push(
      Events.On("terminal:closed", (ev) => {
        if (ev.data.sessionId === sessionId) {
          setStatus("closed");
          closeError = ev.data.error ?? null;
        }
      }),
    );
    if (local) {
      unsubs.push(
        Events.On("record:event", (ev) => {
          if (ev.data.sessionId !== sessionId) return;
          recordLabel = ev.data.kind === "prompt-detected" ? `"${ev.data.label}" bekleniyor` : "Yakalandı, devam ediliyor…";
        }),
      );
    }

    const res = local
      ? await TerminalService.ConnectLocal({ cols: term.cols, rows: term.rows })
      : await TerminalService.Connect({
          categoryPath,
          name: hostName,
          cols: term.cols,
          rows: term.rows,
        });
    sessionId = res.sessionId;
    onSessionId?.(res.sessionId);
  }

  function promptDone() {
    pendingPrompt = null;
  }

  async function toggleRecording() {
    if (!sessionId) return;
    if (recording) {
      recording = false;
      recordLabel = null;
      try {
        const snap = await RecordService.Stop(sessionId);
        if (snap?.steps && snap.steps.length > 0) recordSnapshot = snap;
      } catch {
        // Best effort - nothing to recover into if this fails.
      }
    } else {
      try {
        await RecordService.Start(sessionId);
        recording = true;
      } catch {
        // Best effort - toggle stays off.
      }
    }
  }

  onDestroy(() => {
    unsubs.forEach((u) => u());
    resizeObserver?.disconnect();
    if (recording && sessionId) {
      RecordService.Stop(sessionId).catch(() => {});
    }
    if (sessionId && status !== "closed") {
      TerminalService.Disconnect(sessionId).catch(() => {});
    }
    term?.dispose();
  });
</script>

<div class="terminal-wrap">
  <div class="terminal-container" bind:this={container}></div>
  {#if local && status === "connected"}
    <div class="record-controls">
      {#if recording && recordLabel}
        <span class="record-live muted">{recordLabel}</span>
      {/if}
      <button
        type="button"
        class="record-toggle {recording ? 'active' : ''}"
        onclick={toggleRecording}
        title="Bu terminaldeki ssh girişini host+şifre olarak kaydet"
      >
        <span class="record-dot"></span>
        Kayıt
      </button>
    </div>
  {/if}
  {#if status === "connecting"}
    <div class="status-overlay">
      <span class="muted">{local ? "Terminal açılıyor…" : `${hostName} bağlanıyor…`}</span>
    </div>
  {:else if status === "closed"}
    <div class="status-overlay {closeError ? 'error' : ''}">
      <span class={closeError ? "error-text" : "muted"}>
        {closeError ? `Bağlantı kapandı: ${closeError}` : "Oturum sona erdi"}
      </span>
    </div>
  {/if}
</div>

{#if pendingPrompt}
  {#if pendingPrompt.kind === "host-key"}
    <HostKeyDialog prompt={pendingPrompt} onDone={promptDone} />
  {:else}
    <PromptDialog prompt={pendingPrompt} onDone={promptDone} />
  {/if}
{/if}

{#if recordSnapshot}
  <RecordReviewDialog snapshot={recordSnapshot} onDone={() => (recordSnapshot = null)} onSaved={onHostSaved} />
{/if}

<style>
  .terminal-wrap {
    height: 100%;
    position: relative;
  }
  .terminal-container {
    height: 100%;
  }
  .terminal-container :global(.xterm) {
    height: 100%;
    padding: 8px 12px 24px;
  }
  /* xterm.css hardcodes .xterm-viewport's background to solid #000 (for
     opaque scrollbar rendering on macOS) - that wins over the Terminal's
     `theme.background: "#00000000"` option since the viewport is a plain
     DOM layer, not something xterm's theme touches. Overriding it here is
     what actually lets the app's translucent/blurred background show
     through behind the terminal. */
  .terminal-container :global(.xterm-viewport) {
    background-color: transparent !important;
  }
  .status-overlay {
    position: absolute;
    top: 10px;
    left: 50%;
    transform: translateX(-50%);
    background: var(--surface-bg-strong);
    border: 1px solid var(--surface-border);
    border-radius: var(--radius-sm);
    padding: 4px 14px;
    font-size: 12px;
    backdrop-filter: blur(var(--glass-blur, 16px));
    -webkit-backdrop-filter: blur(var(--glass-blur, 16px));
    z-index: 1;
  }
  .status-overlay.error {
    border-color: var(--danger);
  }
  .record-controls {
    position: absolute;
    top: 10px;
    right: 18px;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .record-live {
    font-size: 11.5px;
    background: var(--surface-bg-strong);
    border: 1px solid var(--surface-border);
    border-radius: var(--radius-sm);
    padding: 3px 10px;
    backdrop-filter: blur(var(--glass-blur, 16px));
    -webkit-backdrop-filter: blur(var(--glass-blur, 16px));
  }
  .record-toggle {
    font-size: 11.5px;
    padding: 3px 10px 3px 8px;
    background: var(--surface-bg-strong);
    backdrop-filter: blur(var(--glass-blur, 16px));
    -webkit-backdrop-filter: blur(var(--glass-blur, 16px));
  }
  .record-toggle.active {
    border-color: var(--danger);
    color: var(--danger);
  }
  .record-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--text-faint);
  }
  .record-toggle.active .record-dot {
    background: var(--danger);
    animation: record-pulse 1.4s ease-in-out infinite;
  }
  @keyframes record-pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.35;
    }
  }
</style>
