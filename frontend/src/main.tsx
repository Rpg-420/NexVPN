import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import './styles/main.css'
import { getLanguage, setLanguage } from './lib/i18n.ts'

// Initialize language & RTL
setLanguage(getLanguage())

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
