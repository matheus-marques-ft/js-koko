import { onUnmounted } from 'vue';

import type { LunaEventType } from '@/utils/lunaBus';
import type { TerminalSessionInfo } from '@/types/modules/postmessage.type';

import { useTerminalContext } from '@/context/terminalContext';

/**
 * Replaces the original sendLunaEvent and eventBus, while also providing Luna communication
 */
export const useTerminalEvents = () => {
  const context = useTerminalContext();

  /**
   * Send a Luna event
   * @param {string} event - event name
   * @param {any} data - event data
   */
  const sendLunaEvent = (event: string, data: any) => {
    context.sendLunaEvent(event, data);
  };

  /**
   * Listen for terminal session events
   * @param {Function} callback - event handler function
   */
  const onTerminalSession = (callback: (info: TerminalSessionInfo) => void) => {
    context.eventBus.on('terminal-session', callback);

    onUnmounted(() => {
      context.eventBus.off('terminal-session', callback);
    });

    return () => context.eventBus.off('terminal-session', callback);
  };

  /**
   * Listen for terminal connect events
   * @param {Function} callback - event handler function
   */
  const onTerminalConnect = (callback: (data: { id: string }) => void) => {
    context.eventBus.on('terminal-connect', callback);

    onUnmounted(() => {
      context.eventBus.off('terminal-connect', callback);
    });

    return () => context.eventBus.off('terminal-connect', callback);
  };

  /**
   * Listen for Luna events - used for cross-component communication
   * @param {Function} callback - event handler function
   */
  const onLunaEvent = (callback: (data: { event: string; data: any }) => void) => {
    context.eventBus.on('luna-event', callback);

    onUnmounted(() => {
      context.eventBus.off('luna-event', callback);
    });

    return () => context.eventBus.off('luna-event', callback);
  };

  /**
   * Emit a terminal session event
   * @param {TerminalSessionInfo} info - terminal session info
   */
  const emitTerminalSession = (info: TerminalSessionInfo) => {
    context.eventBus.emit('terminal-session', info);
  };

  const sendMittEvent = (event: string, data?: any) => {
    context.sendMittEvent(event, data || {});
  };

  const onMittEvent = (event: string, callback: (data: any) => void) => {
    const unsubscribe = context.onMittEvent(event, callback);

    onUnmounted(() => {
      unsubscribe();
    });

    return unsubscribe;
  };

  /**
   * Emit a terminal connect event
   * @param {string} id - terminal ID
   */
  const emitTerminalConnect = (id: string) => {
    context.eventBus.emit('terminal-connect', { id });
  };

  /**
   * Send a message to Luna (the parent window)
   * @param name - event name
   * @param data - event data
   */
  const sendToLuna = <K extends LunaEventType>(name: K, data: any) => {
    context.lunaCommunicator.sendLuna(name, data);
  };

  /**
   * Listen for messages from Luna
   * @param type - event type
   * @param handler - event handler function
   * @returns
   */
  const onLunaMessage = <K extends LunaEventType>(type: K, handler: (data: any) => void) => {
    context.lunaCommunicator.onLuna(type, handler);

    onUnmounted(() => {
      context.lunaCommunicator.offLuna(type, handler);
    });

    return () => context.lunaCommunicator.offLuna(type, handler);
  };

  /**
   * Listen for a one-time Luna message
   * @param type - event type
   * @param handler - event handler function
   */
  const onLunaMessageOnce = <K extends LunaEventType>(type: K, handler: (data: any) => void) => {
    context.lunaCommunicator.once(type, handler);
  };

  return {
    sendLunaEvent,
    emitTerminalSession,
    emitTerminalConnect,
    onTerminalSession,
    onTerminalConnect,
    onLunaEvent,
    sendMittEvent,
    onMittEvent,

    sendToLuna,
    onLunaMessage,
    onLunaMessageOnce,

    lunaCommunicator: context.lunaCommunicator,
  };
};
