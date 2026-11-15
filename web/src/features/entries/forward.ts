export { default as Button } from "../../components/workspace/ForwardButton.svelte";
export {
  observeSessionEvent,
  disconnectSessionFeed,
} from "../../domains/workspace/forwards.svelte";
export const pages = { auth: () => import("../../pages/ForwardAuth.svelte") };
export const dialog = () =>
  import("../../components/workspace/ForwardDialog.svelte");
