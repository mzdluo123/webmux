import Content from "./collapsible-content.svelte";
import Trigger from "./collapsible-trigger.svelte";
import Root from "./collapsible.svelte";
import MotionRoot from "./motion-root.svelte";
import { useManualDisclosureMotion } from "./manual-motion.svelte";

export {
  Root,
  MotionRoot,
  Content,
  Trigger,
  useManualDisclosureMotion,
  //
  Root as Collapsible,
  Content as CollapsibleContent,
  Trigger as CollapsibleTrigger,
};
