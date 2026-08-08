<script setup lang="ts">
import type { FunctionalComponent } from 'vue';

import { useI18n } from 'vue-i18n';
import { useMessage } from 'naive-ui';
import { onMounted, reactive } from 'vue';
import { ArrowDown, ArrowLeft, ArrowRight, ArrowUp } from 'lucide-vue-next';

// import type { TerminalSessionInfo } from '@/types/modules/postmessage.type';
import mittBus from '@/utils/mittBus';
import { useTreeStore } from '@/store/modules/tree';
import { useTerminalStore } from '@/store/modules/terminal';
import { useSessionAdapter } from '@/hooks/useSessionAdapter';
// import { useTerminalEvents } from '@/hooks/useTerminalEvents';
import CardContainer from '@/components/CardContainer/index.vue';

interface KeyboardItem {
  icon?: FunctionalComponent;
  label?: string;
  click: () => void;
}

const { t } = useI18n();
const message = useMessage();
const treeStore = useTreeStore();
const terminalStore = useTerminalStore();
const { isK8sEnvironment } = useSessionAdapter();
// const { onTerminalSession } = useTerminalEvents();

// const assetName = ref('');
// const accontName = ref('');

const keyboardList = reactive<KeyboardItem[]>([
  {
    // icon: Ban,
    label: 'Ctrl+C',
    click: () => {
      writeDataToTerminal('\x03');
    },
  },
  {
    icon: ArrowUp,
    // label: t('UpArrow'),
    click: () => {
      writeDataToTerminal('\x1B[A');
    },
  },
  {
    icon: ArrowDown,
    // label: t('DownArrow'),
    click: () => {
      writeDataToTerminal('\x1B[B');
    },
  },
  {
    icon: ArrowLeft,
    // label: t('LeftArrow'),
    click: () => {
      writeDataToTerminal('\x1B[D');
    },
  },
  {
    icon: ArrowRight,
    // label: t('RightArrow'),
    click: () => {
      writeDataToTerminal('\x1B[C');
    },
  },
]);

function writeDataToTerminal(type: string) {
  if (isK8sEnvironment.value) {
    // K8s environment: get the corresponding terminal instance based on the current tab
    const currentTab = terminalStore.currentTab;

    if (!currentTab) {
      message.error(t('NoActiveTerminalTabFound'));
      return;
    }

    const currentNode = treeStore.getTerminalByK8sId(currentTab);
    const terminal = currentNode?.terminal;

    if (!terminal) {
      message.error(t('TerminalInstanceNotFound'));
      return;
    }

    // Write directly to the currently active terminal
    terminal.paste(type);
    terminal.focus();
  } else {
    // Normal connection: use the existing mittBus event mechanism
    mittBus.emit('write-command', { type });
  }
}

onMounted(() => {
  // const off = onTerminalSession((info: TerminalSessionInfo) => {
  // Terminal session data info is obtained here
  //   console.log('session info:', info);
  //   assetName.value = info.session.asset;
  //   accontName.value = info.session.user;
  // });
});
</script>

<template>
  <n-flex vertical align="center">
    <!-- <CardContainer title="Connection Details">
      <n-descriptions label-placement="left" bordered :column="1">
        <n-descriptions-item label="IP"> Apple </n-descriptions-item>
        <n-descriptions-item label="Asset Name">
          {{ assetName }}
        </n-descriptions-item>
        <n-descriptions-item label="Account Name">
          {{ accontName }}
        </n-descriptions-item>
        <n-descriptions-item label="Max Idle Time"> Apple </n-descriptions-item>
        <n-descriptions-item label="Authorization Expiration Time"> Apple </n-descriptions-item>
        <n-descriptions-item label="Max Session Time"> Apple </n-descriptions-item>
        <n-descriptions-item label="Current Connected Time"> Apple </n-descriptions-item>
      </n-descriptions>
    </CardContainer> -->

    <CardContainer :title="t('AvailableShortcutKey')">
      <n-grid x-gap="8" y-gap="8" :cols="2">
        <n-gi v-for="item in keyboardList" :key="item.label">
          <n-card
            hoverable
            class="cursor-pointer transition-all duration-200 border-transparent hover:border-white/20"
            :content-style="{ padding: '12px' }"
            @click="item.click"
          >
            <template #default>
              <n-flex align="center" justify="center" :size="12" class="!gap-0">
                <component :is="item.icon" :size="16" class="text-white/90 flex-shrink-0" />

                <n-text class="text-xs-plus text-white/90">
                  {{ item.label }}
                </n-text>
              </n-flex>
            </template>
          </n-card>
        </n-gi>
      </n-grid>
    </CardContainer>
  </n-flex>
</template>
