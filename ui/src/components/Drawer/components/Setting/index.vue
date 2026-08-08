<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { storeToRefs } from 'pinia';
import xtermTheme from 'xterm-theme';
import { readText } from 'clipboard-polyfill';
import { computed, nextTick, ref, watch } from 'vue';
import { ChevronDown, ChevronLeft, Ellipsis } from 'lucide-vue-next';

import type { OnlineUser } from '@/types/modules/user.type';
import type { SettingConfig } from '@/types/modules/setting.type';

import { formatMessage } from '@/utils';
// import Share from '@/components/Drawer/components/Share/index.vue';
import { useConnectionStore } from '@/store/modules/useConnection';
import { useClipboardAcl } from '@/hooks/useClipboardAcl';
import { FORMATTER_MESSAGE_TYPE } from '@/types/modules/message.type';
import { SITE_ORIGIN } from '@/utils/config';
import General from '@/components/Drawer/components/General/index.vue';
import { useTerminalSettingsStore } from '@/store/modules/terminalSettings';

defineProps<{
  settings: SettingConfig;
}>();

const terminalSettingsStore = useTerminalSettingsStore();
const connectionStore = useConnectionStore();
const { validateClipboardText } = useClipboardAcl();

const { t } = useI18n();
const { theme } = storeToRefs(terminalSettingsStore);

const currentTerminalConn = computed(() => {
  // TODO Default to the 0th item in the map
  const conn = connectionStore;

  return {
    socket: conn.socket,
    terminal: conn.terminal,
    shareId: conn.shareId || '',
    shareCode: conn.shareCode || '',
    sessionId: conn.sessionId || '',
    terminalId: conn.terminalId || '',
    enableShare: conn.enableShare || false,
    userOptions: conn.userOptions || [],
    onlineUsers: conn.onlineUsers || [],
  };
});

const userFilters = computed(() => {
  const users = currentTerminalConn.value.onlineUsers;
  return {
    currentUser: users.find(item => item.primary),
    otherUsers: users.filter(item => !item.primary),
  };
});

const origin = computed(() => SITE_ORIGIN);

const showLeftArrow = ref(false);
const currentTheme = ref(theme?.value);
const themeOptions = ref([
  { label: 'Default', value: 'Default' },
  ...Object.keys(xtermTheme).map(item => ({ label: item, value: item })),
]);

watch(
  () => theme?.value,
  value => {
    currentTheme.value = value;
  }
);

watch(
  () => currentTerminalConn.value.onlineUsers,
  value => {
    showLeftArrow.value = value.length > 1;
  }
);

/**
 * @description Update the theme
 * @param value
 */
function handleUpdateTheme(value: string) {
  currentTheme.value = value;
  terminalSettingsStore.setDefaultTerminalConfig('theme', value);

  nextTick(() => {
    const { socket, terminalId } = currentTerminalConn.value;
    socket?.send(
      formatMessage(
        terminalId,
        'TERMINAL_SYNC_USER_PREFERENCE',
        JSON.stringify({
          terminal_theme_name: value,
        })
      )
    );
  });
}

/**
 * @description Preview the theme
 * @param event
 */
function previewTheme(event: KeyboardEvent) {
  if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
    const currentIndex = themeOptions.value.findIndex(theme => theme.value === currentTheme.value);
    let nextIndex = currentIndex;

    if (event.key === 'ArrowUp') {
      // If the current index is 0, jump to the last option; otherwise move up
      nextIndex = currentIndex === 0 ? themeOptions.value.length - 1 : currentIndex - 1;
    } else if (event.key === 'ArrowDown') {
      // If the current index is the last one, jump to the first option; otherwise move down
      nextIndex = currentIndex === themeOptions.value.length - 1 ? 0 : currentIndex + 1;
    }

    currentTheme.value = themeOptions.value[nextIndex].value;
    terminalSettingsStore.setDefaultTerminalConfig('theme', themeOptions.value[nextIndex].value);
  }

  if (event.key === 'Enter') {
    handleUpdateTheme(currentTheme.value!);
  }
}

/**
 * @description Click the collapse button
 */
function handleItemHeaderClick(data: { name: string | number; expanded: boolean; event: MouseEvent }) {
  showLeftArrow.value = !data.expanded;
}

/**
 * @description Remove an online user
 */
function handlePositiveClick(userMeta: OnlineUser) {
  const { socket, terminalId, sessionId } = currentTerminalConn.value;
  socket?.send(
    formatMessage(
      terminalId,
      FORMATTER_MESSAGE_TYPE.TERMINAL_SHARE_USER_REMOVE,
      JSON.stringify({
        session: sessionId,
        user_meta: userMeta,
      })
    )
  );
}

/**
 * @description Create a share link
 * @param shareLinkRequest
 */
function handleCreateShareUrl(shareLinkRequest: any) {
  const { socket, terminalId, sessionId } = currentTerminalConn.value;
  socket?.send(
    formatMessage(
      terminalId,
      FORMATTER_MESSAGE_TYPE.TERMINAL_SHARE,
      JSON.stringify({
        origin: origin.value,
        session: sessionId,
        users: shareLinkRequest.users,
        expired_time: shareLinkRequest.expiredTime,
        action_permission: shareLinkRequest.actionPerm,
      })
    )
  );
}

/**
 * @description Search for share users
 * @param query
 */
function handleSearchShareUser(query: string) {
  const { socket, terminalId } = currentTerminalConn.value;
  socket?.send(formatMessage(terminalId, FORMATTER_MESSAGE_TYPE.TERMINAL_GET_SHARE_USER, JSON.stringify({ query })));
}

/**
 * @description Write a command
 * @param command
 */
async function handleWriteCommand(command: string) {
  const { terminal } = currentTerminalConn.value;

  switch (command) {
    case 'Stop':
      terminal?.paste('\x03');
      break;
    case 'Save':
      terminal?.paste('\x13');
      break;
    case 'Undo':
      terminal?.paste('\x1A');
      break;
    case 'Paste':
      {
        const text = await readText();
        if (validateClipboardText('paste', text)) {
          terminal?.paste(text);
        }
      }
      break;
    case 'ArrowUp':
      terminal?.paste('\x1B[A');
      break;
    case 'ArrowDown':
      terminal?.paste('\x1B[B');
      break;
    case 'ArrowLeft':
      terminal?.paste('\x1B[D');
      break;
    case 'ArrowRight':
      terminal?.paste('\x1B[C');
      break;
  }
}
</script>

<template>
  <div v-for="item of settings.items" :key="item.label">
    <n-form-item path="theme" :label-style="item.labelStyle" label-align="top">
      <template #label>
        <n-flex align="center" justify="space-between" class="w-full">
          <n-flex align="center" class="!gap-x-2">
            <component :is="item.labelIcon" size="14" />
            <span class="text-sm">{{ item.label }}</span>
          </n-flex>

          <n-tooltip v-if="item.showMore" size="small">
            <template #trigger>
              <Ellipsis :size="14" class="cursor-pointer focus:outline-none" />
            </template>

            <span> show more </span>
          </n-tooltip>
        </n-flex>
      </template>

      <template v-if="item.type === 'select'">
        <n-select
          size="small"
          :value="currentTheme"
          :options="themeOptions"
          @keydown="previewTheme"
          @update:value="handleUpdateTheme"
        />
      </template>

      <template v-if="item.type === 'list'">
        <n-card size="small">
          <n-flex justify="center" vertical class="w-full">
            <n-flex align="center">
              <n-text> {{ t('CurrentUser') }}: </n-text>
              <n-text depth="1" strong class="text-sm">
                {{ userFilters.currentUser?.user }}
              </n-text>
            </n-flex>

            <n-divider dashed class="!my-2" />

            <n-collapse default-expanded-names="online-user" @item-header-click="handleItemHeaderClick">
              <template #header-extra>
                <ChevronLeft v-if="showLeftArrow" :size="18" class="focus:outline-none" />
                <ChevronDown v-else :size="18" class="focus:outline-none" />
              </template>
              <n-collapse-item :title="`${t('OnlineUser')}:`" name="online-user">
                <template v-if="userFilters.otherUsers.length > 0">
                  <n-flex
                    v-for="usersItem in userFilters.otherUsers"
                    :key="usersItem.user_id"
                    justify="space-between"
                    class="w-full"
                  >
                    <n-tag
                      closable
                      size="small"
                      type="primary"
                      :bordered="false"
                      @close="handlePositiveClick(usersItem)"
                    >
                      <span class="text-xs">{{ usersItem.user }}</span>
                    </n-tag>
                  </n-flex>
                </template>
                <n-empty v-else :description="t('NoOnlineUser')" />
              </n-collapse-item>
            </n-collapse>
          </n-flex>
        </n-card>
      </template>

      <template v-if="item.type === 'create'">
        <n-card size="small">
          <!-- <Share
            :share-id="currentTerminalConn.shareId"
            :share-code="currentTerminalConn.shareCode"
            :share-enable="currentTerminalConn.enableShare"
            :user-options="currentTerminalConn.userOptions"
            @create-share-url="handleCreateShareUrl"
            @search-share-user="handleSearchShareUser"
          /> -->
        </n-card>
      </template>

      <template v-if="item.type === 'keyboard'">
        <General @write-command="handleWriteCommand" />
      </template>
    </n-form-item>
  </div>
</template>

<style scoped lang="scss">
.n-form-item.n-form-item--top-labelled .n-form-item-label {
  align-items: center;
  padding: unset;
}

.n-collapse {
  :deep(.n-collapse-item-arrow) {
    display: none !important;
  }

  :deep(.n-collapse-item__content-inner) {
    padding-top: 5px !important;
  }
}

:deep(.n-form-item-label__text) {
  width: 100%;
}
</style>
