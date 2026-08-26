import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { installNativeBridge } from "./lib/native";
import "./styles/app.css";

// Before the first render: eight components read `window.aos` synchronously to
// decide whether they are drawing a desktop window or a browser tab, and a
// bridge installed after mount would have them all decide wrong once and then
// never re-decide. See lib/native.ts.
installNativeBridge();

const root = document.getElementById("root");
if (!root) throw new Error("the page has no #root to mount into");

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
