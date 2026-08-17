<script lang="ts">
  // Stand-in for window.confirm(), which Wails v3's macOS webview never
  // resolves - WKWebView only shows a JS confirm()/alert() panel if the app
  // implements the corresponding WKUIDelegate method, and this app's native
  // layer only wires up the file-open panel (see webview_window_darwin.m).
  // Left unimplemented, WKWebView just resolves confirm() to false right
  // away, so every `if (!confirm(...)) return;` guard bailed out silently -
  // this is why the delete buttons looked like they did nothing. A <dialog>
  // element needs no native delegate at all (it's just DOM+CSS), which is
  // also why the add/edit forms elsewhere in this app already work fine.
  let dialog: HTMLDialogElement;
  let message = $state("");
  let confirmLabel = $state("Sil");
  let resolveFn: ((value: boolean) => void) | null = null;

  export function open(msg: string, confirmText = "Sil"): Promise<boolean> {
    message = msg;
    confirmLabel = confirmText;
    dialog.showModal();
    return new Promise((resolve) => {
      resolveFn = resolve;
    });
  }

  function respond(value: boolean) {
    dialog.close();
    resolveFn?.(value);
    resolveFn = null;
  }
</script>

<dialog bind:this={dialog} class="glass-panel confirm-dialog" onclose={() => respond(false)}>
  <p>{message}</p>
  <div class="row end">
    <button type="button" onclick={() => respond(false)}>Vazgeç</button>
    <button type="button" class="primary danger-btn" onclick={() => respond(true)}>{confirmLabel}</button>
  </div>
</dialog>

<style>
  .confirm-dialog {
    width: min(360px, 92vw);
  }
  .confirm-dialog p {
    margin: 0 0 16px;
    font-size: 13px;
  }
  .danger-btn {
    background: var(--danger);
    border-color: var(--danger);
  }
</style>
