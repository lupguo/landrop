import { render } from 'preact'
import { App } from './app'
import { applyTheme } from './lib/theme'
import './styles.css'

applyTheme()

render(<App />, document.getElementById('app')!)
