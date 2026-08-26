import { mount } from 'svelte'
import '@fontsource-variable/inter'
import './app.css'
import App from './App.svelte'

const target = document.getElementById('app')
if (!target) throw new Error('Target element #app not found')

mount(App, { target })
