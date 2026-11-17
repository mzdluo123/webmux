import { describe, expect, it } from "vite-plus/test";
import {
  HistoryCursorSchema,
  HandoffPreviewRequestSchema,
  HandoffSendRequestSchema,
} from "../src/contracts/schemas";
describe("native history cursors", () => {
  it("preserves exact decimal positions", () => {
    for (const value of ["0", "1", "9007199254740993", "9223372036854775807"])
      expect(HistoryCursorSchema.parse(value)).toBe(value);
  });
  it("rejects ambiguous and overflowing positions", () => {
    for (const value of [
      "",
      "01",
      "-1",
      "+1",
      "1e3",
      "1.0",
      "9223372036854775808",
    ])
      expect(HistoryCursorSchema.safeParse(value).success).toBe(false);
  });
});
describe("ephemeral handoff protocol", () => {
  const request = {
    configId: "cfg",
    targetSessionId: "",
    agentConfig: {},
    userText: "当前任务\n\tkeep  spaces",
    location: {
      cwd: "/work",
      repositoryId: "",
      worktreeId: "",
      newWorktreeBase: "",
      newWorktreeBranch: "",
    },
  };
  it("preserves the current user task verbatim", () => {
    expect(HandoffPreviewRequestSchema.parse(request).userText).toBe(
      request.userText,
    );
  });
  it("rejects retired draft controls", () => {
    expect(
      HandoffPreviewRequestSchema.safeParse({
        ...request,
        revision: 1,
        retry: true,
      }).success,
    ).toBe(false);
  });
  it("requires the operation handle for a send", () => {
    expect(HandoffSendRequestSchema.safeParse({}).success).toBe(false);
    expect(
      HandoffSendRequestSchema.parse({ operationId: "instance_operation" }),
    ).toEqual({ operationId: "instance_operation" });
  });
});
