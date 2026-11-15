/** Replace the current text selection and place the caret after the insertion. */
export function insertAtSelection(
  text: string,
  start: number,
  end: number,
  insert: string,
) {
  return {
    text: text.slice(0, start) + insert + text.slice(end),
    caret: start + insert.length,
  };
}
