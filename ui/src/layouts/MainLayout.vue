<template>
  <q-layout view="hHr lpr fFr">
    <!-- Header -->
    <q-header bordered class="bg-white text-black">
      <q-toolbar>
        <q-btn dense flat round icon="menu" @click="toggleLeftDrawer" />

        <q-toolbar-title>
          <div class="flex items-center">
            <img src="/ArqLogo.png" alt="Arqut Edge" style="width: 40px; height: 40px;" />
            <span class="q-ml-sm">ARQUT EDGE</span>
          </div>
        </q-toolbar-title>

        <!-- Extension point for EN features -->
        <slot name="header-actions" />
      </q-toolbar>
    </q-header>

    <!-- Left Navigation Drawer -->
    <q-drawer
      v-model="leftDrawerOpen"
      show-if-above
      side="left"
      bordered
    >
      <q-scroll-area class="fit">
        <q-list padding>
          <q-item
            clickable
            v-ripple
            to="/services"
            active-class="text-primary bg-grey-2"
          >
            <q-item-section avatar>
              <q-icon name="dns" />
            </q-item-section>
            <q-item-section>
              <q-item-label>Services</q-item-label>
            </q-item-section>
          </q-item>

          <!-- Extension point for EN navigation items -->
          <slot name="nav-items" />
        </q-list>
      </q-scroll-area>
    </q-drawer>

    <!-- Right Drawer for Forms -->
    <q-drawer
      v-model="drawerStore.rightOpen"
      side="right"
      overlay
      behavior="mobile"
      bordered
      :width="drawerStore.width"
    >
      <q-scroll-area class="fit">
        <component
          v-if="drawerStore.formComponent"
          :is="drawerStore.formComponent"
          v-bind="drawerStore.formProps"
        />
      </q-scroll-area>
    </q-drawer>

    <!-- Main Content -->
    <q-page-container>
      <router-view v-slot="{ Component }">
        <!-- Extension point for page wrappers -->
        <slot name="page-wrapper" :component="Component">
          <component :is="Component" />
        </slot>
      </router-view>
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { useDrawerStore } from '../stores/drawer';

const leftDrawerOpen = ref(false);
const drawerStore = useDrawerStore();

function toggleLeftDrawer() {
  leftDrawerOpen.value = !leftDrawerOpen.value;
}

// Reset form when drawer closes
watch(() => drawerStore.rightOpen, (isOpen) => {
  if (!isOpen) {
    drawerStore.resetForm();
  }
});
</script>
