import { mount } from 'svelte'
import './styles/glass.css'
import App from './App.svelte'
import { loadAppearance } from './lib/appearance.svelte'
import { loadDefaultTerminalFontSize } from './lib/terminalZoom.svelte'

loadAppearance()
loadDefaultTerminalFontSize()

// The hotkey window (main.go's second window, HotkeyWindowService) loads
// this same bundle with ?mode=hotkey - it mounts the exact same <App/> (so
// it gets tabs + the Başlangıç page for free, no separate component to keep
// in sync), just told which window it's in so it can adjust a couple of
// window-chrome-only details - see App.svelte's `hotkeyMode` prop.
const isHotkeyWindow = new URLSearchParams(location.search).get('mode') === 'hotkey'
mount(App, { target: document.getElementById('app')!, props: { hotkeyMode: isHotkeyWindow } })
