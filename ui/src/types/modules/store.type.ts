import type { customTreeOption, SettingConfig } from '@/types/modules/config.type';

export interface IGlobalState {
  initialized: boolean;

  i18nLoaded: boolean;

  interfaceVendor: string | null;
}

export interface IParamsState {
  shareId: string;

  shareCode: string;

  currentUser: any;

  setting: SettingConfig;
}

export interface ITerminalConfig {
  // Theme
  themeName: string;

  // Quick paste
  quickPaste: string;

  // Ctrl
  ctrlCAsCtrlZ: string;

  // Backspace key
  backspaceAsCtrlH: string;

  // Font size
  fontSize: number;

  // Line height
  lineHeight: number;

  // Font family
  fontFamily: string;

  // Whether Zmodem is enabled
  enableZmodem: boolean;

  // Current Zmodem status
  zmodemStatus: boolean;

  // Current tab
  currentTab: string;

  termSelectionText: string;
}

export interface ITreeState {
  connectInfo: any;

  treeNodes: customTreeOption[];

  currentNode: customTreeOption;

  root: customTreeOption;

  isLoaded: boolean;

  terminalMap: Map<string, any>;
}

export type ObjToKeyValArray<T> = {
  [K in keyof T]: [K, T[K]];
}[keyof T];
