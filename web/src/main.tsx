import { StrictMode } from "react";
import ReactDOM from "react-dom/client";
import { RouterProvider } from "@tanstack/react-router";

import "./styles.css";
import "./ui/mesa.css";

import { Providers } from "./app/providers";
import { router } from "./app/router";
import { plainTextFields } from "./lib/plainInput";

const rootElement = document.getElementById("app");
if (rootElement && !rootElement.innerHTML) {
  plainTextFields(document.body);
  ReactDOM.createRoot(rootElement).render(
    <StrictMode>
      <Providers>
        <RouterProvider router={router} />
      </Providers>
    </StrictMode>,
  );
}
