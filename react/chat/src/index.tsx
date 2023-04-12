import React from 'react';
import ReactDOM from 'react-dom/client';
import './index.css';
import PresetTalk from './PresetTalk'; // Import the PresetTalk component
import reportWebVitals from './reportWebVitals';

const root = ReactDOM.createRoot(
  document.getElementById('root') as HTMLElement
);
root.render(
  <React.StrictMode>
    <PresetTalk content={null} /> {/* Use PresetTalk instead of App */}
  </React.StrictMode>
);

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals();
