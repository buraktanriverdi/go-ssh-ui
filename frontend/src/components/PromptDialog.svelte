<script lang="ts">
  import { onMount } from "svelte";
  import { TerminalService } from "../../bindings/go-ssh-ui";
  import type { Prompt } from "../../bindings/go-ssh-ui/internal/sshengine/models";
  import { t } from "../lib/i18n/i18n.svelte";

  let { prompt, onDone }: { prompt: Prompt; onDone: () => void } = $props();

  let dialog: HTMLDialogElement;
  let values = $state((prompt.questions ?? []).map(() => ""));
  let busy = $state(false);

  onMount(() => dialog.showModal());

  async function submit(e: Event) {
    e.preventDefault();
    busy = true;
    await TerminalService.AnswerPrompt(prompt.id, values, true);
    onDone();
  }

  async function cancel() {
    await TerminalService.AnswerPrompt(prompt.id, [], false);
    onDone();
  }
</script>

<dialog bind:this={dialog} class="glass-panel">
  <h3>{prompt.kind === "key-passphrase" ? t("promptDialog.keyPassphraseTitle") : prompt.name || t("promptDialog.defaultTitle")}</h3>
  {#if prompt.instruction}<p class="muted">{prompt.instruction}</p>{/if}
  <form onsubmit={submit}>
    {#each prompt.questions ?? [] as q, i}
      <div class="field">
        <label for={"prompt-" + i}>{q}</label>
        <input id={"prompt-" + i} type={prompt.echo?.[i] ? "text" : "password"} bind:value={values[i]} autofocus={i === 0} />
      </div>
    {/each}
    <div class="row end">
      <button type="button" onclick={cancel}>{t("promptDialog.cancel")}</button>
      <button type="submit" class="primary" disabled={busy}>{t("promptDialog.submit")}</button>
    </div>
  </form>
</dialog>
