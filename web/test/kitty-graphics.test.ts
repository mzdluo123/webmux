import { expect, test, vi } from "vitest";
import {
  createKittyGraphics,
  createKittyStream,
  parseKittyControl,
  type KittyImage,
  type KittyPlacement,
  type KittyPlacementSpec,
} from "../src/kitty-graphics";

const encoder = new TextEncoder();
const decoder = new TextDecoder();

function raw(width: number, height: number) {
  return btoa(String.fromCharCode(...new Uint8Array(width * height * 4)));
}

function harness(
  decode: (input: {
    bytes: Uint8Array;
    format: number;
    width?: number;
    height?: number;
  }) => Promise<KittyImage> = async ({ width, height }) =>
    fakeImage(width ?? 2, height ?? 2),
) {
  const replies: string[] = [];
  const logs: string[] = [];
  const moves: [number, number][] = [];
  const placements: {
    image: KittyImage;
    spec: KittyPlacementSpec;
    disposed: boolean;
    handle: KittyPlacement;
  }[] = [];
  let cursor = { column: 0, line: 0 };
  const graphics = createKittyGraphics({
    cellSize: () => ({ width: 10, height: 20 }),
    cursor: () => cursor,
    viewport: () => ({ top: 0, bottom: 24 }),
    reply: (text) => {
      replies.push(text);
      logs.push(`reply:${text}`);
    },
    write: (data, parsed) => {
      logs.push(`text:${decoder.decode(data)}`);
      parsed();
    },
    moveCursor: (columns, rows) => {
      moves.push([columns, rows]);
      cursor = { column: cursor.column + columns, line: cursor.line + rows };
    },
    decode: vi.fn(decode),
    renderer: {
      place(image, spec, dismissed) {
        const placement = {
          image,
          spec,
          disposed: false,
          handle: undefined as unknown as KittyPlacement,
        };
        placements.push(placement);
        placement.handle = {
          dispose() {
            if (placement.disposed) return;
            placement.disposed = true;
            dismissed();
          },
          bounds: () => ({
            column: spec.column,
            line: spec.line,
            columns: spec.columns,
            rows: spec.rows,
          }),
        };
        return placement.handle;
      },
    },
  });
  const send = (value: string) =>
    new Promise<void>((resolve) =>
      graphics.write(encoder.encode(value), resolve),
    );
  return {
    graphics,
    replies,
    logs,
    moves,
    placements,
    send,
    setCursor(column: number, line: number) {
      cursor = { column, line };
    },
  };
}

function fakeImage(width = 2, height = 2): KittyImage {
  return {
    width,
    height,
    source: {} as CanvasImageSource,
  };
}

test("the stream passes text through and buffers APC chunks split lazily", () => {
  const stream = createKittyStream();
  const events: { kind: string; value: string }[] = [];
  const emit = (event: {
    kind: string;
    data?: Uint8Array;
    control?: string;
    payload?: string;
  }) =>
    events.push(
      event.kind === "text"
        ? { kind: "text", value: decoder.decode(event.data) }
        : {
            kind: "graphics",
            value: `${event.control}:${event.payload}`,
          },
    );
  stream.write(encoder.encode("hello\x1b"), emit);
  stream.write(encoder.encode("_G"), emit);
  stream.write(encoder.encode("a=T,i=1;AAAA\x1b"), emit);
  stream.write(encoder.encode("\\world"), emit);
  expect(events).toEqual([
    { kind: "text", value: "hello" },
    { kind: "graphics", value: "a=T,i=1:AAAA" },
    { kind: "text", value: "world" },
  ]);
  expect(stream.pendingBytes).toBe(0);
});

test("non-kitty APC sequences stay in the text stream", () => {
  const stream = createKittyStream();
  const events: { kind: string; value: string }[] = [];
  const emit = (event: {
    kind: string;
    data?: Uint8Array;
    control?: string;
    payload?: string;
  }) =>
    events.push(
      event.kind === "text"
        ? { kind: "text", value: decoder.decode(event.data) }
        : { kind: "graphics", value: `${event.control}:${event.payload}` },
    );
  stream.write(encoder.encode("a\x1b_X b\x1b\\c"), emit);
  expect(events).toEqual([{ kind: "text", value: "a\x1b_X b\x1b\\c" }]);
});

test("control data rejects malformed keys", () => {
  expect(parseKittyControl("a=T,f=100,i=7")).toEqual({
    a: "T",
    f: "100",
    i: "7",
  });
  expect(() => parseKittyControl("ab=1")).toThrow(/invalid control/);
  expect(() => parseKittyControl("a=T,a=p")).toThrow(/invalid control/);
});

test("a transmitted image is stored and displayed at the cursor", async () => {
  const board = harness();
  board.setCursor(3, 5);
  await board.send(`before\x1b_Ga=T,f=32,s=2,v=2,i=1;${raw(2, 2)}\x1b\\after`);
  expect(board.logs.filter((entry) => entry.startsWith("text:"))).toEqual([
    "text:before",
    "text:after",
  ]);
  expect(board.placements).toHaveLength(1);
  expect(board.placements[0].spec).toMatchObject({
    column: 3,
    line: 5,
    columns: 1,
    rows: 1,
    drawWidth: 2,
    drawHeight: 2,
    offsetX: 0,
    offsetY: 0,
    z: 0,
  });
  expect(board.moves).toEqual([[1, 0]]);
  expect(board.replies).toEqual(["\x1b_Gi=1;OK\x1b\\"]);
});

test("consecutive placements advance the cursor before the next one", async () => {
  const board = harness();
  await board.send(`\x1b_Ga=T,f=32,s=2,v=2,i=1;${raw(2, 2)}\x1b\\`);
  await board.send(`\x1b_Ga=p,i=1\x1b\\\x1b_Ga=p,i=1\x1b\\`);
  expect(board.placements.map((placement) => placement.spec.column)).toEqual([
    0, 1, 2,
  ]);
});

test("source rectangles are clipped to the image", async () => {
  const board = harness();
  await board.send(
    `\x1b_Ga=T,f=32,s=4,v=4,i=1,x=2,y=2,w=100,h=100;${raw(4, 4)}\x1b\\`,
  );
  expect(board.placements[0].spec).toMatchObject({
    sourceX: 2,
    sourceY: 2,
    sourceWidth: 2,
    sourceHeight: 2,
  });
});

test("delete by screen position follows the placement's current line", async () => {
  const board = harness();
  await board.send(`\x1b_Ga=T,f=32,s=2,v=2,i=1;${raw(2, 2)}\x1b\\`);
  board.placements[0].spec.line = 2;
  await board.send("\x1b_Ga=d,d=p,x=1,y=3\x1b\\");
  expect(board.placements[0].disposed).toBe(true);
});

test("delete by image-id range accepts full-width ids", async () => {
  const board = harness();
  await board.send(`\x1b_Ga=T,f=32,s=2,v=2,i=1;${raw(2, 2)}\x1b\\`);
  await board.send("\x1b_Ga=d,d=r,x=0,y=4294967295\x1b\\");
  expect(board.placements[0].disposed).toBe(true);
});

test("a dismissed anonymous placement releases its image", async () => {
  const dispose = vi.fn();
  const board = harness(async ({ width, height }) => ({
    ...fakeImage(width, height),
    dispose,
  }));
  await board.send(`\x1b_Ga=T,f=32,s=2,v=2;${raw(2, 2)}\x1b\\`);
  expect(board.placements).toHaveLength(1);
  board.placements[0].handle.dispose();
  expect(dispose).toHaveBeenCalledTimes(1);
});

test("placement ids are acknowledged with their resolved image id", async () => {
  const board = harness();
  await board.send(`\x1b_Ga=t,f=32,s=2,v=2,I=13;${raw(2, 2)}\x1b\\`);
  await board.send("\x1b_Ga=p,I=13\x1b\\");
  expect(board.replies.at(-1)).toBe("\x1b_Gi=1,I=13;OK\x1b\\");
  await board.send("\x1b_Ga=p,p=7\x1b\\");
  expect(board.replies.at(-1)).toBe("\x1b_Gi=1,I=13;OK\x1b\\");
});

test("chunked transmission is assembled before display", async () => {
  const board = harness();
  const payload = raw(2, 2);
  await board.send(
    `\x1b_Ga=T,f=32,s=2,v=2,i=7,m=1;${payload.slice(0, 8)}\x1b\\`,
  );
  expect(board.placements).toHaveLength(0);
  await board.send(`\x1b_Gm=1;${payload.slice(8, 16)}\x1b\\`);
  expect(board.placements).toHaveLength(0);
  await board.send(`\x1b_Gm=0;${payload.slice(16)}\x1b\\`);
  expect(board.placements).toHaveLength(1);
  expect(board.replies).toEqual(["\x1b_Gi=7;OK\x1b\\"]);
});

test("the capability query answers before following text and skips decoding", async () => {
  const decode = vi.fn(async () => fakeImage());
  const board = harness(decode);
  await board.send("\x1b_Ga=q,i=9,f=24,s=1,v=1;AAAA\x1b\\after");
  expect(decode).not.toHaveBeenCalled();
  expect(board.logs).toEqual(["reply:\x1b_Gi=9;OK\x1b\\", "text:after"]);
});

test("the C=1 policy leaves the cursor in place", async () => {
  const board = harness();
  await board.send(`\x1b_Ga=T,f=32,s=2,v=2,i=1,C=1;${raw(2, 2)}\x1b\\`);
  expect(board.moves).toEqual([]);
});

test("placement geometry letterboxes explicit cell rectangles", async () => {
  const board = harness();
  await board.send(`\x1b_Ga=T,f=32,s=4,v=4,i=1,c=2,r=2;${raw(4, 4)}\x1b\\`);
  expect(board.placements[0].spec).toMatchObject({
    columns: 2,
    rows: 2,
    drawWidth: 20,
    drawHeight: 20,
    offsetX: 0,
    offsetY: 10,
  });
});

test("deleting placements keeps image data until an uppercase delete frees it", async () => {
  const board = harness();
  await board.send(`\x1b_Ga=T,f=32,s=2,v=2,i=5;${raw(2, 2)}\x1b\\`);
  await board.send("\x1b_Ga=d,d=i,i=5\x1b\\");
  expect(board.placements[0].disposed).toBe(true);
  await board.send("\x1b_Ga=p,i=5\x1b\\");
  expect(board.placements).toHaveLength(2);
  expect(board.placements[1].disposed).toBe(false);
  await board.send("\x1b_Ga=d,d=I,i=5\x1b\\");
  expect(board.placements[1].disposed).toBe(true);
  await board.send("\x1b_Ga=p,i=5\x1b\\");
  expect(board.replies.at(-1)).toBe("\x1b_Gi=5;ENOENT:image not found\x1b\\");
});

test("image numbers allocate ids and report the newest image", async () => {
  const board = harness();
  await board.send(`\x1b_Ga=t,f=32,s=2,v=2,I=13;${raw(2, 2)}\x1b\\`);
  expect(board.replies).toEqual(["\x1b_Gi=1,I=13;OK\x1b\\"]);
  await board.send("\x1b_Ga=p,I=13\x1b\\");
  expect(board.placements).toHaveLength(1);
});

test("unsupported transmission mediums and animation report errors", async () => {
  const board = harness();
  await board.send("\x1b_Ga=t,i=2,t=f;ZmlsZQ==\x1b\\");
  expect(board.replies.at(-1)).toContain("ENOTSUP");
  await board.send("\x1b_Ga=f,i=2;AAAA\x1b\\");
  expect(board.replies.at(-1)).toContain("ENOTSUP");
});

test("reset disposes placements and stored images", async () => {
  const board = harness();
  await board.send(`\x1b_Ga=T,f=32,s=2,v=2,i=1;${raw(2, 2)}\x1b\\`);
  board.graphics.reset();
  expect(board.placements[0].disposed).toBe(true);
  await board.send("\x1b_Ga=p,i=1\x1b\\");
  expect(board.replies.at(-1)).toContain("ENOENT");
});
