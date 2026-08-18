export const settingsView = {
  title: "Settings",
  intro:
    "Adjust the window's transparency, blur, and accent color here. Transparency/blur/color apply instantly; the native window blur takes effect on next launch.",
  appearance: {
    heading: "Appearance",
    opacityLabel: "Background transparency",
    opacityHint: "A lower value shows more of what's behind the window (and the terminal background).",
    blurLabel: "Panel blur",
    blurHint:
      "0 turns off the extra blur on dialogs and panels entirely (sharpest look). The native blur affecting the whole window background is a separate setting - below.",
  },
  terminal: {
    heading: "Terminal",
    fontSizeLabel: "Font size",
    fontSizeHint:
      "The starting font size for newly opened terminals. Doesn't affect terminals already open - each one changes its own size independently with Cmd+=/Cmd+-/Cmd+0 or the right-click menu.",
  },
  language: {
    heading: "Language",
    intro: "Choose the app's interface language.",
    systemLabel: "System",
    systemHint: "Automatically follows the operating system's language.",
    turkishLabel: "Türkçe",
    englishLabel: "English",
  },
  backdrop: {
    heading: "Window backdrop (native)",
    hintDiff:
      "Different from \"Panel blur\" above: this is the layer affecting the desktop/other windows visible behind the window, not dialog cards - all of it follows the transparency setting above.",
    hintRestart: "You need to quit and reopen the app for the change to take effect.",
    saved: "Saved - will apply on next launch.",
    options: {
      translucent: { label: "Blurred (vibrancy)", hint: "The classic frosted-glass macOS look." },
      liquidGlass: {
        label: "Liquid Glass",
        hint: "Apple's new glassy effect (macOS 15+; falls back to Blurred automatically on older versions).",
      },
      transparent: { label: "Transparent (no blur)", hint: "Just as see-through, but crisp - no blur at all." },
    },
  },
  hotkey: {
    heading: "Hotkey Window",
    intro:
      "Like in iTerm: a floating window with a single local terminal that opens and closes with a global shortcut, even while the app is in the background.",
    enabled: "Enabled",
    shortcutLabel: "Shortcut",
    shortcutUnset: "Not set",
    changeShortcut: "Change shortcut",
    capturing: "Press a key combo… (Esc to cancel)",
    unsetHint: "Enabled but no shortcut chosen yet - click \"Change shortcut\" and press a key combination.",
    opacityLabel: "Window transparency",
    colorLabel: "Window background color",
    backdropLabel: "Window backdrop (native)",
    backdropHint: "The color/transparency above layers on top of this. You need to quit and reopen the app for the change to take effect.",
    testButton: "Test the window",
    saved: "Saved",
    liveHint: "Shortcut/color/transparency apply instantly - except the window backdrop (native), which needs a restart.",
    errors: {
      unsupportedKey: "This key isn't supported as a system-wide shortcut. Please try a different key (a letter, digit, or function key is recommended).",
      needsModifier: "This shortcut needs at least one modifier key (Cmd/Ctrl/Option/Shift).",
    },
  },
  accent: {
    heading: "Accent color",
    customTitle: "Pick a custom color",
    colors: {
      blue: "Blue",
      purple: "Purple",
      pink: "Pink",
      red: "Red",
      orange: "Orange",
      yellow: "Yellow",
      green: "Green",
      graphite: "Graphite",
    },
  },
  resetButton: "Reset to defaults",
};
