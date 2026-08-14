<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Events } from "@wailsio/runtime";
  import { Terminal } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";
  import "@xterm/xterm/css/xterm.css";
  import { TerminalService } from "../../bindings/go-ssh-ui";
  import type { Prompt } from "../../bindings/go-ssh-ui/internal/sshengine/models";
  import PromptDialog from "./PromptDialog.svelte";
  import HostKeyDialog from "./HostKeyDialog.svelte";

  export type TerminalStatus = "connecting" | "connected" | "closed";

  let {
    categoryPath,
    hostName,
    onStatusChange,
    onSessionId,
  }: {
    categoryPath: string[];
    hostName: string;
    onStatusChange?: (status: TerminalStatus) => void;
    onSessionId?: (id: string) => void;
  } = $props();

  let container: HTMLDivElement;
  let term: Terminal;
  let fitAddon: FitAddon;
  let resizeObserver: ResizeObserver;

  let sessionId: string | null = $state(null);
  let status: TerminalStatus = $state("connecting");
  let closeError: string | null = $state(null);
  let pendingPrompt: Prompt | null = $state(null);

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

    const res = await TerminalService.Connect({
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

  onDestroy(() => {
    unsubs.forEach((u) => u());
    resizeObserver?.disconnect();
    if (sessionId && status !== "closed") {
      TerminalService.Disconnect(sessionId).catch(() => {});
    }
    term?.dispose();
  });
</script>

<div class="terminal-wrap">
  <div class="terminal-container" bind:this={container}></div>
  {#if status === "connecting"}
    <div class="status-overlay">
      <span class="muted">{hostName} bağlanıyor…</span>
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

<style>
  .terminal-wrap {
    height: 100%;
    position: relative;
  }
  .terminal-container {
    height: 100%;
    padding: 8px 12px;
  }
  .terminal-container :global(.xterm) {
    height: 100%;
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
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    z-index: 1;
  }
  .status-overlay.error {
    border-color: var(--danger);
  }
</style>
