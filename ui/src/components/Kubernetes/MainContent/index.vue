<script setup lang="ts">
import type { Ref } from 'vue';
import type { SearchAddon } from '@xterm/addon-search';
import type { DropdownOption, TabPaneProps } from 'naive-ui';
import type { UseDraggableReturn } from 'vue-draggable-plus';

import { v4 as uuid } from 'uuid';
import { useI18n } from 'vue-i18n';
import { storeToRefs } from 'pinia';
import { useDebounceFn } from '@vueuse/core';
import { useDraggable } from 'vue-draggable-plus';
import { BrushCleaning, CircleX, Copy, RotateCcw } from 'lucide-vue-next';
import { computed, h, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';

import mittBus from '@/utils/mittBus';
import { useColor } from '@/hooks/useColor';
import { lunaCommunicator } from '@/utils/lunaBus';
import { useTreeStore } from '@/store/modules/tree.ts';
import { createTerminal } from '@/hooks/useKubernetes.ts';
import { useTerminalStore } from '@/store/modules/terminal.ts';
import { LUNA_MESSAGE_TYPE } from '@/types/modules/message.type';
import { getXTerminalLineContent, updateIcon } from '@/hooks/helper';

const treeStore = useTreeStore();
const terminalStore = useTerminalStore();

const { t } = useI18n();
const { lighten, darken } = useColor();
const { connectInfo } = storeToRefs(treeStore);

const themeColors = computed(() => {
  const colors = {
    '--tab-bg-color': darken(5),
    '--tab-inactive-bg-color': darken(3),
    '--tab-active-bg-color': darken(1),
    '--tab-inactive-text-color': lighten(50),
    '--tab-active-text-color': lighten(60),
    '--icon-color': lighten(45),
    '--icon-hover-bg-color': lighten(8),
  };

  return colors;
});

const nameRef = ref('');
const contextIdentification = ref('');
const showDrawer = ref(false);
const showSearchInput = ref(false);
const showContextMenu = ref(false);
const dropdownY = ref(0);
const dropdownX = ref(0);
const panels: Ref<TabPaneProps[]> = ref([]);
const searchAddonRef = ref<SearchAddon | null>(null);

const processedElements = new Set();
const contextMenuOption = [
  {
    label: t('Reconnect'),
    key: 'reconnect',
    icon: () => h(RotateCcw, { size: 16 }),
  },
  {
    label: t('Close Current Tab'),
    key: 'close',
    icon: () => h(CircleX, { size: 16 }),
  },
  {
    label: t('Close All Tabs'),
    key: 'closeAll',
    icon: () => h(BrushCleaning, { size: 16 }),
  },
  {
    label: t('Clone Connect'),
    key: 'cloneConnect',
    icon: () => h(Copy, { size: 16 }),
  },
];

const swapElements = (arr: any[], index1: number, index2: number) => {
  [arr[index1], arr[index2]] = [arr[index2], arr[index1]];
  return arr;
};

const findNodeById = (nameRef: string) => {
  const treeStore = useTreeStore();

  for (const [_key, value] of treeStore.terminalMap.entries()) {
    if (value.k8s_id === nameRef) {
      treeStore.setCurrentNode(value);

      // Restore the ctrlCAsCtrlZ config for the current node
      if (value.ctrlCAsCtrlZMap && value.ctrlCAsCtrlZMap.has(value.k8s_id)) {
        const ctrlCAsCtrl: string = value.ctrlCAsCtrlZMap.get(value.k8s_id);
        terminalStore.setTerminalConfig('ctrlCAsCtrlZ', ctrlCAsCtrl);
      }

      // Restore the backspaceAsCtrlH config for the current node
      if (value.backspaceAsCtrlHMap && value.backspaceAsCtrlHMap.has(value.k8s_id)) {
        const backspaceAsCtrlH: string = value.backspaceAsCtrlHMap.get(value.k8s_id);
        terminalStore.setTerminalConfig('backspaceAsCtrlH', backspaceAsCtrlH);
      }
      break;
    }
  }
};

/**
 * @description Handle tab close
 *
 * @param name
 */
function handleClose(name: string) {
  const node = treeStore.getTerminalByK8sId(name);
  const socket = node.socket;

  if (socket) {
    socket.send(
      JSON.stringify({
        type: 'K8S_CLOSE',
        id: node.id,
        k8s_id: node.k8s_id,
      })
    );
  }

  const index = panels.value.findIndex(panel => panel.name === name);

  panels.value.splice(index, 1);

  treeStore.removeK8sIdMap(name);

  const panelLength = panels.value.length;

  // If all tabs are closed, automatically close the drawer
  if (panelLength === 0) {
    mittBus.emit('close-drawer');
  }

  // Only auto-position to the previous tab when there is more than 1 tab and the tab being closed is the current one
  if (panelLength >= 1 && nameRef.value === name) {
    nameRef.value = panels.value[panelLength - 1].name as string;
    findNodeById(nameRef.value);
    terminalStore.setTerminalConfig('currentTab', nameRef.value);
    // Also switch focus when auto-switching after close
    focusActiveTerminal(nameRef.value);
  }
}

/**
 * @description Switch tab
 *
 * @param value
 */
function handleChangeTab(value: string) {
  nameRef.value = value;

  findNodeById(value);

  terminalStore.setTerminalConfig('currentTab', value);

  // Switch terminal focus
  focusActiveTerminal(value);
}

/**
 * @description Focus the currently active terminal and blur the others
 */
function focusActiveTerminal(activeK8sId: string) {
  // Delay execution to ensure the DOM update from the tab switch has completed
  nextTick(() => {
    setTimeout(() => {
      for (const [_mapKey, node] of treeStore.terminalMap.entries()) {
        const terminal = node?.terminal;
        if (terminal) {
          if (node.k8s_id === activeK8sId) {
            // Get the terminal's DOM element
            const terminalElement = document.getElementById(activeK8sId);
            if (terminalElement) {
              // Ensure the element is visible
              terminalElement.style.display = '';

              // Force focus
              terminal.focus();

              // Additionally ensure the DOM element also receives focus
              const textareaElements = terminalElement.querySelectorAll('textarea');
              if (textareaElements.length > 0) {
                textareaElements[0].focus();
              }
            }
          } else {
            terminal.blur();
          }
        }
      }
    }, 100);
  });
}

/**
 * @description Shortcut actions on the right side of each tab label
 * @param e
 */
function handleContextMenu(e: PointerEvent) {
  let target: HTMLElement = e.target as HTMLElement;

  while (target && !target.hasAttribute('data-name')) {
    target = target.parentElement as HTMLElement;
  }

  if (target) {
    // Get the configured data attribute
    const dataName: string = target.getAttribute('data-name') as string;

    if (dataName) {
      contextIdentification.value = dataName;

      e.preventDefault();
      showContextMenu.value = true;
      dropdownY.value = e.clientY;
      dropdownX.value = e.clientX;
    }
  }
}

/**
 * @description Reconnect
 */
function handleReconnect(type: string) {
  const operatedNode = treeStore.getTerminalByK8sId(contextIdentification.value);
  const socket = operatedNode?.socket;

  if (type === 'reconnect') {
    if (socket) {
      socket.send(
        JSON.stringify({
          type: 'K8S_CLOSE',
          id: operatedNode.id,
          k8s_id: operatedNode.k8s_id,
        })
      );
    }

    // Find the index of the operated node,
    const index = panels.value.findIndex(panel => panel.name === contextIdentification.value);

    panels.value.splice(index, 1);
    treeStore.removeK8sIdMap(operatedNode.k8s_id);

    const newId = uuid();

    operatedNode.key = newId;
    operatedNode.k8s_id = newId;
    operatedNode.position = index;

    mittBus.emit('connect-terminal', { ...operatedNode });
  } else if (type === 'cloneConnect') {
    mittBus.emit('connect-terminal', { ...operatedNode });
  }

  showContextMenu.value = false;
}

/**
 * @description Context menu callback
 *
 * @param key
 * @param _option
 */
function handleContextMenuSelect(key: string, _option: DropdownOption) {
  switch (key) {
    case 'reconnect': {
      // For reconnecting, only the k8sid needs to change, and a K8S_CLOSE message needs to be sent
      handleReconnect('reconnect');
      break;
    }
    case 'close': {
      handleClose(contextIdentification.value);
      showContextMenu.value = false;
      break;
    }
    case 'closeAll': {
      panels.value.forEach((panel: any) => {
        treeStore.removeK8sIdMap(panel.k8s_id);
      });

      panels.value = [];

      mittBus.emit('close-drawer');

      showContextMenu.value = false;
      break;
    }
    case 'cloneConnect': {
      handleReconnect('cloneConnect');

      break;
    }
  }
}

/**
 * @description Update the unique identifier of the tab
 *
 * @param key
 */
function updateTabElements(key: string) {
  const tabElements = document.querySelectorAll('.n-tabs-tab-wrapper');

  tabElements.forEach(element => {
    if (!processedElements.has(element)) {
      element.setAttribute('data-identification', key);
      processedElements.add(element);
    }
  });
}

/**
 * @description Close the right-side menu
 */
function handleClickOutside() {
  showContextMenu.value = false;
}

/**
 * @description Drag handling for the tab item
 */
function initializeDraggable() {
  const tabsContainer = document.querySelector('.n-tabs-wrapper');

  if (tabsContainer) {
    // For useDraggable, directly operating on panel may cause an undefined value to be injected, causing an error; so the code below always operates on a copy
    useDraggable<UseDraggableReturn>(
      // @ts-expect-error Type error
      tabsContainer,
      JSON.parse(JSON.stringify(panels.value)),
      {
        animation: 150,
        onEnd: async event => {
          if (!event || event.newIndex === undefined || event.oldIndex === undefined) {
            return console.warn('Event or index is undefined');
          }

          const newIndex = event!.newIndex - 1;
          const oldIndex = event!.oldIndex - 1;

          // JSON.parse(JSON.stringify(...)) can't be used here, or it will cause a circular reference; a shallow copy is sufficient
          const clonedPanels = panels.value.map(panel => ({ ...panel }));

          panels.value = swapElements(clonedPanels, newIndex, oldIndex).filter(panel => panel !== null);

          const newActiveTab: string = panels.value[newIndex!]?.name as string;

          if (newActiveTab) {
            nameRef.value = newActiveTab;
            findNodeById(newActiveTab);
            terminalStore.setTerminalConfig('currentTab', newActiveTab);
          }
        },
      }
    );
  }
}

/**
 * @description Switch to the previous Tab
 */
function switchToPreviousTab() {
  const currentIndex = panels.value.findIndex(panel => panel.name === nameRef.value);

  if (currentIndex > 0) {
    nameRef.value = panels.value[currentIndex - 1].name as string;
  } else {
    nameRef.value = panels.value[panels.value.length - 1].name as string;
  }

  findNodeById(nameRef.value);

  terminalStore.setTerminalConfig('currentTab', nameRef.value);
}

/**
 * @description Switch to the next Tab
 */
function switchToNextTab() {
  const currentIndex = panels.value.findIndex(panel => panel.name === nameRef.value);

  if (currentIndex < panels.value.length - 1) {
    nameRef.value = panels.value[currentIndex + 1].name as string;
  } else {
    nameRef.value = panels.value[0].name as string;
  }

  findNodeById(nameRef.value);

  terminalStore.setTerminalConfig('currentTab', nameRef.value);
}

const debouncedSwitchToPreviousTab = useDebounceFn(() => {
  switchToPreviousTab();
}, 200);

const debouncedSwitchToNextTab = useDebounceFn(() => {
  switchToNextTab();
}, 200);

function unloadEvent() {
  mittBus.off('sync-theme');
  mittBus.off('share-user');
  mittBus.off('terminal-search');
  mittBus.off('create-share-url');
  mittBus.off('remove-share-user');
}

onMounted(() => {
  const lunaConfig = terminalStore.getConfig;

  nextTick(() => {
    initializeDraggable();
  });

  mittBus.on('open-setting', () => {
    showDrawer.value = true;
  });

  mittBus.on('connect-terminal', (node: any) => {
    let index;

    // If panels already contains the same k8s_id, treat it as a duplicate connection to the same node
    panels.value.forEach(panel => {
      if (panel.name === node.k8s_id) {
        const newId = uuid();
        node.key = newId;
        node.k8s_id = newId;
      }
    });

    if (node.position || node.position === 0) {
      index = node.position;
    } else {
      index = panels.value.length;
    }

    panels.value.splice(index, 0, {
      ...node,
      // Both are required fields for the component library
      name: node.k8s_id,
      tab: node.label,
    });

    nameRef.value = node.k8s_id;

    nextTick(() => {
      treeStore.setCurrentNode(node);
      terminalStore.setTerminalConfig('currentTab', node.k8s_id);

      unloadEvent();
      updateTabElements(node.k8s_id);

      const el = document.getElementById(node.k8s_id);

      if (el) {
        const { terminal, searchAddon } = createTerminal(el, node.socket, lunaConfig, node, t);

        searchAddonRef.value = searchAddon;

        treeStore.setK8sIdMap(node.k8s_id, {
          ...node,
          terminal,
        });

        const firstSendMessage = {
          id: node.id,
          k8s_id: node.k8s_id,
          namespace: node.namespace || '',
          pod: node.pod || '',
          container: node.container || '',
          type: 'TERMINAL_K8S_INIT',
          data: JSON.stringify({
            cols: terminal.cols,
            rows: terminal.rows,
            code: '',
          }),
        };

        el.addEventListener('mouseleave', () => {
          terminal.blur();
          const currentNode = treeStore.getTerminalByK8sId(nameRef.value);

          if (currentNode) {
            lunaCommunicator.sendLuna(LUNA_MESSAGE_TYPE.TERMINAL_CONTENT_RESPONSE, {
              content: getXTerminalLineContent(10, terminal),
              sessionId: currentNode.k8s_id,
              terminalId: currentNode.id,
            });
          }
        });

        try {
          // Send the initial connection data
          node.socket.send(JSON.stringify(firstSendMessage));
          updateIcon(connectInfo.value);
        } catch (e: any) {
          throw new Error(e);
        }
      }
    });
  });

  mittBus.on('open-search', () => {
    showSearchInput.value = true;
  });
  mittBus.on('alt-shift-left', debouncedSwitchToPreviousTab);
  mittBus.on('alt-shift-right', debouncedSwitchToNextTab);
});

onBeforeUnmount(() => {
  mittBus.off('alt-shift-left', debouncedSwitchToPreviousTab);
  mittBus.off('alt-shift-right', debouncedSwitchToNextTab);
  mittBus.off('connect-terminal');
});
</script>

<template>
  <SearchInput
    v-if="showSearchInput"
    :is-kubernetes="true"
    :search-addon="searchAddonRef"
    @close="showSearchInput = false"
  />
  <n-layout :native-scrollbar="false" content-style="height: 100%" :style="themeColors">
    <n-tabs
      v-model:value="nameRef"
      closable
      size="small"
      type="card"
      tab="show:lazy"
      tab-style="min-width: 80px;"
      class="header-tab relative"
      @close="handleClose"
      @update:value="handleChangeTab"
      @contextmenu.prevent="handleContextMenu"
    >
      <n-tab-pane
        v-for="panel of panels"
        :key="panel.name"
        :tab="panel.tab"
        :name="panel.name"
        display-directive="show:lazy"
        class="pt-0"
      >
        <n-layout :native-scrollbar="false">
          <n-scrollbar trigger="hover">
            <div :id="String(panel.name)" class="k8s-terminal" />
          </n-scrollbar>
        </n-layout>
      </n-tab-pane>
    </n-tabs>
  </n-layout>
  <n-dropdown
    show-arrow
    size="medium"
    trigger="manual"
    placement="bottom-start"
    content-style='font-size: "13px"'
    :x="dropdownX"
    :y="dropdownY"
    :show="showContextMenu"
    :options="contextMenuOption"
    @select="handleContextMenuSelect"
    @clickoutside="handleClickOutside"
  />
</template>

<style scoped lang="scss">
@use './index.scss';
</style>
