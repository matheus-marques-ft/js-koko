<script setup lang="ts">
import { ref } from 'vue';

interface VirtualKey {
  label: string;
  sequence?: string;
  type: 'modifier' | 'control' | 'char' | 'navigation';
  width?: number; // Key width, in units of 1 standard key
}

// Keyboard state
const keyboardState = ref({
  isShiftPressed: false,
  isCtrlPressed: false,
  isAltPressed: false,
  isCmdPressed: false,
  capsLock: false,
});

// Detect whether the system is Mac
const isMac = navigator.platform.toUpperCase().includes('MAC');

// Main keyboard area
const mainKeyboard = {
  row1: [
    { label: '1', type: 'char' as const },
    { label: '2', type: 'char' as const },
    { label: '3', type: 'char' as const },
    { label: '4', type: 'char' as const },
    { label: '5', type: 'char' as const },
    { label: '6', type: 'char' as const },
    { label: '7', type: 'char' as const },
    { label: '8', type: 'char' as const },
    { label: '9', type: 'char' as const },
    { label: '0', type: 'char' as const },
    { label: '-', type: 'char' as const },
    { label: '=', type: 'char' as const },
    { label: 'Backspace', sequence: '\x7F', type: 'control' as const, width: 2.5 },
    { label: '↑', sequence: '\x1B[A', type: 'navigation' as const },
  ],
  // Row 2: QWERTY row 1
  row2: [
    { label: 'Tab', sequence: '\t', type: 'control' as const, width: 1.5 },
    { label: 'Q', type: 'char' as const },
    { label: 'W', type: 'char' as const },
    { label: 'E', type: 'char' as const },
    { label: 'R', type: 'char' as const },
    { label: 'T', type: 'char' as const },
    { label: 'Y', type: 'char' as const },
    { label: 'U', type: 'char' as const },
    { label: 'I', type: 'char' as const },
    { label: 'O', type: 'char' as const },
    { label: 'P', type: 'char' as const },
    { label: '[', type: 'char' as const },
    { label: ']', type: 'char' as const },
    { label: '\\', type: 'char' as const, width: 1 },
    { label: '↓', sequence: '\x1B[B', type: 'navigation' as const },
  ],
  // Row 3: ASDF row 2
  row3: [
    { label: 'Caps', type: 'control' as const, width: 1.75 },
    { label: 'A', type: 'char' as const },
    { label: 'S', type: 'char' as const },
    { label: 'D', type: 'char' as const },
    { label: 'F', type: 'char' as const },
    { label: 'G', type: 'char' as const },
    { label: 'H', type: 'char' as const },
    { label: 'J', type: 'char' as const },
    { label: 'K', type: 'char' as const },
    { label: 'L', type: 'char' as const },
    { label: ';', type: 'char' as const },
    { label: "'", type: 'char' as const },
    { label: 'Enter', sequence: '\r', type: 'control' as const, width: 1.75 },
    { label: '→', sequence: '\x1B[C', type: 'navigation' as const },
  ],
  // Row 4: ZXCV row 3
  row4: [
    { label: 'Shift', type: 'modifier' as const, width: 2.25 },
    { label: 'Z', type: 'char' as const },
    { label: 'X', type: 'char' as const },
    { label: 'C', type: 'char' as const },
    { label: 'V', type: 'char' as const },
    { label: 'B', type: 'char' as const },
    { label: 'N', type: 'char' as const },
    { label: 'M', type: 'char' as const },
    { label: ',', type: 'char' as const },
    { label: '.', type: 'char' as const },
    { label: '/', type: 'char' as const },
    { label: 'Shift', type: 'modifier' as const, width: 2.25 },
    { label: '←', sequence: '\x1B[D', type: 'navigation' as const },
  ],
  // Row 5: modifier keys and spacebar (adjusted per system)
  row5: [
    { label: 'Ctrl', type: 'modifier' as const, width: 1.25 },
    { label: isMac ? 'Cmd' : 'Alt', type: 'modifier' as const, width: 1.25 },
    { label: 'Alt', type: 'modifier' as const, width: 1.25 },
    { label: 'Space', sequence: ' ', type: 'char' as const, width: 6.75 },
    { label: 'Alt', type: 'modifier' as const, width: 1.25 },
    { label: isMac ? 'Cmd' : 'Ctrl', type: 'modifier' as const, width: 1.25 },
    { label: 'Ctrl', type: 'modifier' as const, width: 1.25 },
  ],
};

// Get the key style
const getKeyStyle = (key: VirtualKey) => {
  const width = key.width || 1;
  return {
    width: `${width * 2.5}rem`,
    minWidth: `${width * 2.5}rem`,
  };
};

// Handle key clicks
const handleKeyClick = (_key: VirtualKey) => {
  // Actual key-handling logic can be added here,
  // such as sending to the terminal, triggering shortcuts, etc.
  // console.log('Key clicked:', _key);
};

// Handle modifier key state
const handleModifierKey = (key: VirtualKey) => {
  switch (key.label) {
    case 'Shift':
      keyboardState.value.isShiftPressed = !keyboardState.value.isShiftPressed;
      break;
    case 'Ctrl':
      keyboardState.value.isCtrlPressed = !keyboardState.value.isCtrlPressed;
      break;
    case 'Alt':
      keyboardState.value.isAltPressed = !keyboardState.value.isAltPressed;
      break;
    case 'Cmd':
      keyboardState.value.isCmdPressed = !keyboardState.value.isCmdPressed;
      break;
    case 'Caps':
      keyboardState.value.capsLock = !keyboardState.value.capsLock;
      break;
  }
};

// Handle combined key click events
const onKeyClick = (key: VirtualKey) => {
  if (key.type === 'modifier' || key.label === 'Caps') {
    handleModifierKey(key);
  }
  handleKeyClick(key);
};
</script>

<template>
  <n-card :style="{ width: '1000px' }" bordered>
    <div class="main-keyboard flex-1">
      <!-- Row 1: number keys -->
      <div class="flex gap-1 mb-1 justify-start">
        <n-button
          v-for="key in mainKeyboard.row1"
          :key="`row1-${key.label}`"
          tertiary
          size="small"
          class="h-10 text-sm flex-shrink-0"
          :style="getKeyStyle(key)"
          @click="onKeyClick(key)"
        >
          {{ key.label }}
        </n-button>
      </div>

      <!-- Row 2: QWERTY row 1 -->
      <div class="flex gap-1 mb-1 justify-start">
        <n-button
          v-for="key in mainKeyboard.row2"
          :key="`row2-${key.label}`"
          tertiary
          size="small"
          class="h-10 text-sm flex-shrink-0"
          :style="getKeyStyle(key)"
          @click="onKeyClick(key)"
        >
          {{ key.label }}
        </n-button>
      </div>

      <!-- Row 3: ASDF row 2 -->
      <div class="flex gap-1 mb-1 justify-start">
        <n-button
          v-for="key in mainKeyboard.row3"
          :key="`row3-${key.label}`"
          tertiary
          size="small"
          class="h-10 text-sm flex-shrink-0"
          :style="getKeyStyle(key)"
          @click="onKeyClick(key)"
        >
          {{ key.label }}
        </n-button>
      </div>

      <!-- Row 4: ZXCV row 3 -->
      <div class="flex gap-1 mb-1 justify-start">
        <n-button
          v-for="key in mainKeyboard.row4"
          :key="`row4-${key.label}`"
          tertiary
          size="small"
          class="h-10 text-sm flex-shrink-0"
          :style="getKeyStyle(key)"
          @click="onKeyClick(key)"
        >
          {{ key.label }}
        </n-button>
      </div>

      <!-- Row 5: modifier keys and spacebar -->
      <div class="flex gap-1 mb-1 justify-start">
        <n-button
          v-for="key in mainKeyboard.row5"
          :key="`row5-${key.label}`"
          tertiary
          size="small"
          class="h-10 text-sm flex-shrink-0"
          :style="getKeyStyle(key)"
          @click="onKeyClick(key)"
        >
          {{ key.label }}
        </n-button>
      </div>
    </div>
  </n-card>
</template>
