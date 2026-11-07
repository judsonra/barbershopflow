import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { registerSW } from 'virtual:pwa-register'
import App from './App'
import { api } from './api'
import { restoreTheme } from './branding'
import './styles.css'

// Antes do primeiro render: a tela já nasce com a paleta da barbearia em vez
// de piscar o Clássico enquanto o app busca /tenant.
restoreTheme(window.location.pathname, api.isAuthenticated())

registerSW({ immediate: true })

createRoot(document.getElementById('root')!).render(
  <StrictMode><App /></StrictMode>
)
