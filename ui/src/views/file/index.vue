<script setup lang="ts">
import FileManager from '@/components/Drawer/components/FileManagement/index.vue';

// Get the token from the url params
const token = new URLSearchParams(window.location.search).get('token') || '';
</script>

<template>
  <div class="h-full w-full">
    <FileManager :sftp-token="token" :show-empty="false" />
  </div>
</template>
