/** The mounted presentation never owns the stream or its parsed model. */
export interface TerminalSocketView {
  fit(): void;
  scrollToBottom(): void;
  isVisible(): boolean;
  isImmersive(): boolean;
  copy(text: string): void;
  resetFind(): void;
  preserveViewport(): boolean;
}

export interface TerminalSocketAttachment {
  themeChanged(): void;
  presentationChanged(): void;
  dispose(): void;
}
