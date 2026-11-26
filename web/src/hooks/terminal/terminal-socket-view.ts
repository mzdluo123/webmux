/** The mounted presentation never owns the stream or its parsed model. */
export interface TerminalSocketView {
  fit(): void;
  scrollToBottom(): void;
  isVisible(): boolean;
  copy(text: string): void;
  resetFind(): void;
  preserveViewport(): boolean;
}

export interface TerminalSocketAttachment {
  dispose(): void;
}
