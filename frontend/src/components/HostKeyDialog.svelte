<script lang="ts">
  import { onMount } from "svelte";
  import { TerminalService } from "../../bindings/go-ssh-ui";
  import type { Prompt } from "../../bindings/go-ssh-ui/internal/sshengine/models";
  import { t } from "../lib/i18n/i18n.svelte";

  let { prompt, onDone }: { prompt: Prompt; onDone: () => void } = $props();

  let dialog: HTMLDialogElement;
  let busy = $state(false);

  onMount(() => dialog.showModal());

  async function trust() {
    busy = true;
    await TerminalService.AnswerPrompt(prompt.id, [], true);
    onDone();
  }

  async function reject() {
    await TerminalService.AnswerPrompt(prompt.id, [], false);
    onDone();
  }
</script>

<dialog bind:this={dialog} class="glass-panel">
  <h3>{prompt.isChanged ? t("hostKeyDialog.changedTitle") : t("hostKeyDialog.unknownTitle")}</h3>
  <p class="mono">{prompt.hostname}</p>
  {#if prompt.isChanged}
    <p class="error-text">
      {t("hostKeyDialog.changedWarning")}
    </p>
  {:else}
    <p class="muted">{t("hostKeyDialog.firstConnectHint")}</p>
  {/if}
  <p class="mono key-line">{prompt.keyType} {prompt.fingerprint}</p>
  <div class="row end">
    <button type="button" onclick={reject}>{t("hostKeyDialog.reject")}</button>
    <button type="button" class={prompt.isChanged ? "danger" : "primary"} onclick={trust} disabled={busy}>{t("hostKeyDialog.trust")}</button>
  </div>
</dialog>

<style>
  .key-line {
    font-size: 12px;
    word-break: break-all;
  }
</style>
