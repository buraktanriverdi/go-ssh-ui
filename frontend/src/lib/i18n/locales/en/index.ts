import type { Dict } from "../tr";
import { app } from "./app";
import { unlockScreen } from "./unlockScreen";
import { confirmDialog } from "./confirmDialog";
import { promptDialog } from "./promptDialog";
import { settingsView } from "./settingsView";
import { categoryForm } from "./categoryForm";
import { categoryTree } from "./categoryTree";
import { hostKeyDialog } from "./hostKeyDialog";
import { targetFilePicker } from "./targetFilePicker";
import { terminalPane } from "./terminalPane";
import { recordReviewDialog } from "./recordReviewDialog";
import { hostsView } from "./hostsView";
import { passwordsView } from "./passwordsView";
import { hostForm } from "./hostForm";
import { filesPane } from "./filesPane";

// Explicit `: Dict` (not `as const`/inferred) is load-bearing: assigning an
// object literal to it triggers TypeScript's excess/missing property checks
// against the Turkish source tree, so a namespace or key added to `tr` but
// forgotten here fails `npm run check` instead of silently falling back to
// Turkish at runtime (see i18n.svelte.ts's resolve() fallback).
export const en: Dict = {
  app,
  unlockScreen,
  confirmDialog,
  promptDialog,
  settingsView,
  categoryForm,
  categoryTree,
  hostKeyDialog,
  targetFilePicker,
  terminalPane,
  recordReviewDialog,
  hostsView,
  passwordsView,
  hostForm,
  filesPane,
};
