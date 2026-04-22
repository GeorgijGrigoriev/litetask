import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ConfigProvider, theme } from 'antd'
import 'antd/dist/reset.css'
import './index.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ConfigProvider
      theme={{
        algorithm: theme.darkAlgorithm,
        token: {
          colorPrimary: '#7b68ee',
          borderRadius: 4,
          fontFamily: "'DM Sans', system-ui, sans-serif",
          colorBgContainer: '#13152a',
          colorBgElevated: '#1c1e36',
          colorBorder: '#2a2d4a',
          colorText: '#dce0f5',
          colorTextSecondary: '#7a7e9e',
          colorBgLayout: '#0b0c18',
          fontSize: 14,
          lineHeight: 1.55,
        },
        components: {
          Layout: {
            bodyBg: '#0b0c18',
            headerBg: '#111327',
          },
          Card: {
            headerBg: '#1c1e36',
          },
          Table: {
            headerBg: '#1c1e36',
          },
          Modal: {
            contentBg: '#13152a',
            headerBg: '#13152a',
          },
          Drawer: {
            colorBgElevated: '#13152a',
          },
          Tabs: {
            itemColor: '#7a7e9e',
            itemHoverColor: '#dce0f5',
            itemSelectedColor: '#7b68ee',
            inkBarColor: '#7b68ee',
          },
          Select: {
            optionSelectedBg: 'rgba(123,104,238,0.15)',
          },
          Message: {
            colorBgElevated: '#1c1e36',
          },
        },
      }}
    >
      <App />
    </ConfigProvider>
  </StrictMode>,
)
