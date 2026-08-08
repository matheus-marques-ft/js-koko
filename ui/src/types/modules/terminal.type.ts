export interface ITerminalSettings {
  // Terminal font size
  fontSize: number;

  // Terminal line height
  lineHeight: number;

  // Terminal font family
  fontFamily: string;

  // Terminal theme
  themeName: string;

  // Whether to enable Ctrl+C as Ctrl+Z
  ctrlCAsCtrlZ: string;

  // Whether to enable quick paste
  quickPaste: string;

  // Whether to enable backspace key as Ctrl+H
  backspaceAsCtrlH: string;

  // Theme
  theme: string;
}
