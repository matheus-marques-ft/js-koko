import type { Terminal } from '@xterm/xterm';

import { createDiscreteApi } from 'naive-ui';

import type { TranslateFunction } from '@/types';
import type { ILunaConfig } from '@/types/modules/config.type';
import type { RowData } from '@/components/Drawer/components/FileManagement/index.vue';

import { AsciiBackspace, AsciiCtrlC, AsciiCtrlZ, AsciiDel } from '@/utils/config';

const { message } = createDiscreteApi(['message']);

/**
 * @description Get the minute label
 * @param item
 * @param t
 */
export function getMinuteLabel(item: number, t: TranslateFunction): string {
  let minuteLabel = t('Minute');

  if (item > 1) {
    minuteLabel = t('Minutes');
  }

  return `${item} ${minuteLabel}`;
}

/**
 * @description Write the buffer into the terminal
 * @param enableZmodem
 * @param zmodemStatus
 * @param terminal
 * @param data
 */
export function writeBufferToTerminal(
  enableZmodem: boolean,
  zmodemStatus: boolean,
  terminal: Terminal | null,
  data: any
) {
  if (!enableZmodem && zmodemStatus) return message.error('Zmodem is not enabled and is currently in Zmodem state; display is not allowed');
  if (!terminal) return;
  terminal.write(new Uint8Array(data));
}

export function preprocessInput(data: string, config: Partial<ILunaConfig>) {
  // If the backspaceAsCtrlH config option is enabled (value "1") and the input data contains the
  // Delete key's ASCII code (AsciiDel, i.e. 127), replace it with the Backspace key's ASCII code
  // (AsciiBackspace, i.e. 8)
  if (config.backspaceAsCtrlH === '1') {
    if (data.charCodeAt(0) === AsciiDel) {
      data = String.fromCharCode(AsciiBackspace);
    }
  }

  if (config.ctrlCAsCtrlZ === '1') {
    if (data.charCodeAt(0) === AsciiCtrlC) {
      data = String.fromCharCode(AsciiCtrlZ);
    }
  }

  // Use string replacement methods to avoid using control characters in regular expressions
  // const escSeq200 = '\u001B[200~';
  // const escSeq201 = '\u001B[201~';

  // if (data.includes(escSeq200) || data.includes(escSeq201)) {
  //   return data.replace(escSeq200, '').replace(escSeq201, '');
  // }

  return data;
}

/**
 * @description Process the file name
 * @param row
 */
export function getFileName(row: RowData) {
  if (row.is_dir) {
    return 'Folder';
  }

  const lastDotIndex = row.name.lastIndexOf('.');

  return lastDotIndex !== -1 ? row.name.slice(lastDotIndex + 1) : 'File';
}

/**
 * @description Send an event to the parent window using postMessage.
 *
 * @param {string} name - The name of the event.
 * @param {any} data - The data to send with the event.
 * @param {string | null} [lunaId] - The ID of the Luna instance.
 * @param {string | null} [origin] - The origin of the message.
 */
export function sendEventToLuna(name: string, data: any, lunaId: string | null = '', origin: string | null = '') {
  if (lunaId !== null && origin !== null) {
    try {
      window.parent.postMessage({ name, id: lunaId, data }, origin);
    } catch (e) {
      console.error(e);
    }
  }
}

/**
 * @description Format the message as a JSON string.
 *
 * @param id - The message ID.
 * @param type - The message type.
 * @param data - The message data.
 * @returns The formatted JSON string.
 */
export function formatMessage(id: string, type: string, data: any) {
  return JSON.stringify({
    id,
    type,
    data,
  });
}
