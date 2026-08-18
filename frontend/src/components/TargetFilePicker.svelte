<script lang="ts">
  import type { Files } from "../../bindings/go-ssh-ui/internal/configx/models";
  import { t } from "../lib/i18n/i18n.svelte";

  let { files, value = $bindable("") }: { files: Files; value: string } = $props();

  let selection = $state(files.configPath);
  let newFileName = $state("");

  function update() {
    if (selection === "__new__") {
      const name = newFileName.trim();
      if (!name) {
        value = "";
        return;
      }
      const withExt = /\.(ya?ml)$/i.test(name) ? name : `${name}.yaml`;
      value = `${files.confDDir}/${withExt}`;
    } else {
      value = selection;
    }
  }
  update();
</script>

<div class="field">
  <label for="target-file">{t("targetFilePicker.label")}</label>
  <select id="target-file" bind:value={selection} onchange={update}>
    <option value={files.configPath}>{t("targetFilePicker.mainFileOption")}</option>
    {#each files.confDFiles ?? [] as f (f)}
      <option value={f}>{f.split("/").pop()}</option>
    {/each}
    <option value="__new__">{t("targetFilePicker.newFileOption")}</option>
  </select>
  {#if selection === "__new__"}
    <input
      type="text"
      placeholder={t("targetFilePicker.newFileNamePlaceholder")}
      bind:value={newFileName}
      oninput={update}
      style="margin-top: 6px"
    />
  {/if}
</div>
