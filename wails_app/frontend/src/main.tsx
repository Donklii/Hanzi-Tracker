import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import { LimiteDeErro } from './comum/LimiteDeErro'

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <LimiteDeErro>
            <App/>
        </LimiteDeErro>
    </React.StrictMode>
)
