import { expect, test } from "vitest";
import type { Terminal } from "@xterm/xterm";
import {
  Group,
  OrthographicCamera,
  Scene,
  WebGLRenderer,
  WebGLRenderTarget,
} from "three";
import { XrTerminalMesh } from "../../src/hooks/terminal/xr-terminal-mesh";
import {
  orderXrSurfaces,
  XrSurface,
} from "../../src/hooks/terminal/xr-surface";
import { installTerminalFixture, openTerminalApi } from "./terminal-fixture";

installTerminalFixture();

async function write(terminal: Terminal, value: string) {
  const { promise, resolve } = Promise.withResolvers<void>();
  terminal.write(value, resolve);
  await promise;
}

test("logical-grid growth keeps GPU terminal pixels live", async () => {
  const { api } = await openTerminalApi();
  api.setImmersive(true);
  api.resizeGrid(20, 10);
  const terminal = api.emulator!;
  terminal.options.theme = {
    red: "#ff0000",
    blue: "#0000ff",
    green: "#00ff00",
  };
  terminal.options.cursorBlink = false;
  await write(terminal, "\x1b[41m\x1b[2J");
  const scene = new Scene();
  const parent = new Group();
  parent.scale.set(1.6, 1.1, 1);
  scene.add(parent);
  const camera = new OrthographicCamera(-1, 1, 1, -1, 0.1, 10);
  camera.position.z = 2;
  const renderer = new WebGLRenderer();
  const target = new WebGLRenderTarget(8, 8);
  renderer.setRenderTarget(target);
  const view = new XrTerminalMesh(parent);
  const pixel = new Uint8Array(4);
  function capture() {
    renderer.render(scene, camera);
    renderer.readRenderTargetPixels(target, 4, 4, 1, 1, pixel);
  }
  try {
    view.sync(terminal, [1.6, 1.1]);
    capture();
    expect(pixel[0]).toBeGreaterThan(pixel[2]!);
    api.resizeGrid(40, 10);
    await write(terminal, "\x1b[44m\x1b[2J");
    view.update([1.6, 1.1]);
    capture();
    expect(pixel[2]).toBeGreaterThan(pixel[0]!);
    await write(terminal, "\x1b[42m\x1b[2J");
    view.update([1.6, 1.1]);
    capture();
    expect(pixel[1]).toBeGreaterThan(pixel[2]!);
    expect(renderer.getContext().getError()).toBe(0);
  } finally {
    view.dispose();
    target.dispose();
    renderer.dispose();
    renderer.forceContextLoss();
  }
});

test("coplanar panels cover terminal children coherently while physical depth still wins", async () => {
  const { api } = await openTerminalApi();
  api.setImmersive(true);
  api.resizeGrid(20, 10);
  const terminal = api.emulator!;
  terminal.options.theme = { red: "#ff0000" };
  terminal.options.cursorBlink = false;
  await write(terminal, "\x1b[41m\x1b[2J");
  const upper = new XrSurface();
  const lower = new XrSurface();
  for (const surface of [lower, upper]) surface.setSize(1.6, 1.1);
  lower.draw(() => {
    lower.context.fillStyle = "#00ff00";
    lower.context.fillRect(0, 0, lower.width, lower.height);
  });
  upper.draw(() => {
    upper.context.fillStyle = "#0000ff";
    upper.context.fillRect(0, 0, upper.width, upper.height);
  });
  const scene = new Scene();
  scene.add(lower.group, upper.group);
  orderXrSurfaces([lower, upper]);
  const view = new XrTerminalMesh(lower.group);
  const camera = new OrthographicCamera(-0.85, 0.85, 0.6, -0.6, 0.1, 10);
  camera.position.z = 2;
  const renderer = new WebGLRenderer();
  const target = new WebGLRenderTarget(64, 64);
  renderer.setRenderTarget(target);
  const body = new Uint8Array(4);
  const header = new Uint8Array(4);
  function capture() {
    renderer.render(scene, camera);
    renderer.readRenderTargetPixels(target, 32, 32, 1, 1, body);
    renderer.readRenderTargetPixels(target, 32, 58, 1, 1, header);
  }
  try {
    view.sync(terminal, [1.6, 1.1]);
    capture();
    expect(body[2]).toBeGreaterThan(body[0]!);
    expect(header[2]).toBeGreaterThan(header[1]!);
    lower.group.position.z = 0.05;
    capture();
    expect(body[0]).toBeGreaterThan(body[2]!);
    expect(header[1]).toBeGreaterThan(header[2]!);
  } finally {
    view.dispose();
    lower.dispose();
    upper.dispose();
    target.dispose();
    renderer.dispose();
    renderer.forceContextLoss();
  }
});
