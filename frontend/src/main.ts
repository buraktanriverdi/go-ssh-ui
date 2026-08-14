import { mount } from 'svelte'
import './styles/glass.css'
import App from './App.svelte'
import { loadAppearance } from './lib/appearance.svelte'

loadAppearance()
mount(App, { target: document.getElementById('app')! })
