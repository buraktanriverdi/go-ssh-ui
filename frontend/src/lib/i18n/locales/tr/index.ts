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

// Turkish is the source language (the app's original strings) - every other
// locale's Dict type is checked against this one, see ../en/index.ts.
export const tr = {
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

export type Dict = typeof tr;
