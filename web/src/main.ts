import { mount } from "svelte";
import App from "./App.svelte";
import { initializeLocale } from "$lib/locale";
import { initializeTheme } from "./hooks/settings/theme";
import { trackInstallation } from "./platform/install.svelte";
import "./styles.css";

initializeLocale();
initializeTheme();
trackInstallation();
mount(App, { target: document.getElementById("app")! });
