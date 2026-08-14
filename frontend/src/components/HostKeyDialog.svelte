<script lang="ts">
  import { onMount } from "svelte";
  import { TerminalService } from "../../bindings/go-ssh-ui";
  import type { Prompt } from "../../bindings/go-ssh-ui/internal/sshengine/models";

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
  <h3>{prompt.isChanged ? "Host anahtarı değişti" : "Bilinmeyen host"}</h3>
  <p class="mono">{prompt.hostname}</p>
  {#if prompt.isChanged}
    <p class="error-text">
      Bu host için daha önce kaydedilen anahtar farklı. Bu meşru bir sunucu değişikliği olabilir, ama bir
      araya-girme (MITM) saldırısının işareti de olabilir - emin değilsen reddet.
    </p>
  {:else}
    <p class="muted">Bu hosta ilk kez bağlanıyorsun. Devam edersen anahtar ~/.ssh/known_hosts dosyana eklenecek.</p>
  {/if}
  <p class="mono key-line">{prompt.keyType} {prompt.fingerprint}</p>
  <div class="row end">
    <button type="button" onclick={reject}>Reddet</button>
    <button type="button" class={prompt.isChanged ? "danger" : "primary"} onclick={trust} disabled={busy}>Güven</button>
  </div>
</dialog>

<style>
  .key-line {
    font-size: 12px;
    word-break: break-all;
  }
</style>
