import {
  terminalArrowKeys,
  terminalKeys,
  terminalLeadingKeys,
  terminalModifierKeys,
} from "../../domains/terminal/terminal-keys";
import type { XrViewSelection } from "./xr-view-state";
import { XrSurface, type XrRegion } from "./xr-surface";

export function createXrKeyboard(target: () => XrViewSelection | undefined) {
  const surface = new XrSurface(1200, 540);
  surface.group.position.set(0, -0.95, -1.7);
  surface.setSize(1.5, 0.675);
  surface.group.visible = false;
  let symbols = false;
  let signature = "";
  function send(value: string) {
    const current = target();
    if (!current?.source.canInput(current.layout.kind, current.layout.sourceId))
      return;
    current.source.input(value);
    signature = "";
  }
  return {
    surface,
    update() {
      if (!surface.group.visible) return;
      const binding = target();
      const current = binding?.source;
      const enabled =
        !!binding &&
        !!current?.canInput(binding.layout.kind, binding.layout.sourceId);
      const next = JSON.stringify([
        current?.id,
        current?.title,
        current?.modifiers,
        current?.shiftLocked,
        binding?.layout.id,
        enabled,
        symbols,
      ]);
      if (signature === next) return;
      signature = next;
      const regions: XrRegion[] = [];
      const rows = symbols
        ? ["1234567890", "!@#$%^&*()", "[]{}<>/\\|~`", "-_=+;:'\",.?"]
        : ["1234567890", "qwertyuiop", "asdfghjkl", "zxcvbnm"];
      rows.forEach((row, y) =>
        row.split("").forEach((key, x) =>
          regions.push({
            id: `key:${key}`,
            label:
              current?.modifiers.shift && !symbols ? key.toUpperCase() : key,
            x: 8 + x * 100,
            y: 56 + y * 76,
            width: 96,
            height: 72,
            disabled: !enabled,
            run: () => send(key),
          }),
        ),
      );
      [
        ...terminalLeadingKeys,
        ["⌫", "\x7f"] as const,
        ["空格", terminalKeys.space] as const,
        ["Ctrl+C", terminalKeys.interrupt] as const,
        ...terminalArrowKeys,
      ].forEach(([label, value], index) =>
        regions.push({
          id: `control:${index}`,
          label,
          x: 8 + index * 118,
          y: 366,
          width: 114,
          height: 72,
          disabled: !enabled,
          run: () => send(value),
        }),
      );
      terminalModifierKeys.forEach(([key, label], index) =>
        regions.push({
          id: `modifier:${key}`,
          label,
          selected: current?.modifiers[key],
          disabled: !enabled,
          x: 8 + index * 160,
          y: 448,
          width: 156,
          height: 72,
          run: () => {
            const binding = target();
            if (
              binding?.source.canInput(
                binding.layout.kind,
                binding.layout.sourceId,
              )
            )
              binding.source.toggleModifier(key);
            signature = "";
          },
        }),
      );
      regions.push(
        {
          id: "caps",
          label: "锁定 Shift",
          selected: current?.shiftLocked,
          x: 488,
          y: 448,
          width: 190,
          height: 72,
          disabled: !enabled,
          run: () => {
            const binding = target();
            if (
              binding?.source.canInput(
                binding.layout.kind,
                binding.layout.sourceId,
              )
            )
              binding.source.lockModifier("shift");
            signature = "";
          },
        },
        {
          id: "reset",
          label: "清除修饰",
          x: 684,
          y: 448,
          width: 180,
          height: 72,
          disabled: !enabled,
          run: () => {
            const binding = target();
            if (
              binding?.source.canInput(
                binding.layout.kind,
                binding.layout.sourceId,
              )
            )
              binding.source.resetModifiers();
            signature = "";
          },
        },
        {
          id: "symbols",
          label: symbols ? "ABC" : "符号",
          x: 870,
          y: 448,
          width: 150,
          height: 72,
          run: () => {
            symbols = !symbols;
            signature = "";
          },
        },
        {
          id: "hide",
          label: "收起",
          x: 1026,
          y: 448,
          width: 166,
          height: 72,
          run: () => {
            surface.group.visible = false;
          },
        },
      );
      surface.regions = regions;
      surface.draw(() =>
        surface.text(
          `${current?.title ?? "请选择输入视图"} · ${binding?.layout.id.slice(-8) ?? "无目标"} · ${enabled ? "共享输入 / 草稿" : "只读，当前内容不可输入"}`,
          16,
          12,
          "#edf4ef",
          "24px system-ui",
        ),
      );
    },
    dispose() {
      surface.dispose();
    },
  };
}
