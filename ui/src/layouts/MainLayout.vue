<template>
  <q-layout view="hHr lpr fFr">
    <!-- Header -->
    <q-header bordered class="main-header">
      <q-toolbar>
        <q-btn
          dense
          flat
          round
          icon="menu"
          aria-label="Menu"
          @click="toggleLeftDrawer"
        />

        <q-toolbar-title>
          <div class="flex items-center">
            <img
              :src="logoSrc"
              alt="Arqut Edge"
              class="header-logo"
            />
            <div class="q-ml-sm">
              <div class="header-title">ARQUT EDGE</div>
              <div class="header-subtitle">Community Edition</div>
            </div>
          </div>
        </q-toolbar-title>

        <!-- Dark Mode Toggle -->
        <q-btn
          flat
          round
          :icon="$q.dark.isActive ? 'light_mode' : 'dark_mode'"
          aria-label="Toggle dark mode"
          @click="toggleDarkMode"
        >
          <q-tooltip>{{ $q.dark.isActive ? 'Light mode' : 'Dark mode' }}</q-tooltip>
        </q-btn>

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
      class="nav-drawer"
      :width="280"
    >
      <q-scroll-area class="fit">

        <q-list padding class="nav-list">
          <q-item
            clickable
            v-ripple
            to="/services"
            exact
            class="nav-item"
          >
            <q-item-section avatar>
              <q-icon name="dns" />
            </q-item-section>
            <q-item-section>
              <q-item-label class="text-label-large">Services</q-item-label>
            </q-item-section>
          </q-item>

          <!-- Settings - only shown in docker/HA addon mode -->
          <q-item
            v-if="isDocker"
            clickable
            v-ripple
            to="/settings"
            class="nav-item"
          >
            <q-item-section avatar>
              <q-icon name="settings" />
            </q-item-section>
            <q-item-section>
              <q-item-label class="text-label-large">Settings</q-item-label>
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
      class="form-drawer"
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
import { ref, computed, watch, onMounted } from 'vue';
import { useQuasar } from 'quasar';
import { useDrawerStore } from '../stores/drawer';

const $q = useQuasar();
const leftDrawerOpen = ref(false);
const drawerStore = useDrawerStore();

// Check if running in docker/HA addon mode
const isDocker = process.env.TARGET === 'docker';

// Use different logo for dark/light theme
const logoSrc = computed(() => $q.dark.isActive ? '/ArqLogoDark.png' : '/ArqLogo.png');

// Initialize dark mode from localStorage
onMounted(() => {
  const savedDarkMode = localStorage.getItem('darkMode');
  if (savedDarkMode !== null) {
    $q.dark.set(savedDarkMode === 'true');
  } else {
    // Check system preference
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    $q.dark.set(prefersDark);
  }
});

function toggleLeftDrawer() {
  leftDrawerOpen.value = !leftDrawerOpen.value;
}

function toggleDarkMode() {
  $q.dark.toggle();
  localStorage.setItem('darkMode', String($q.dark.isActive));
}

// Reset form when drawer closes
watch(() => drawerStore.rightOpen, (isOpen) => {
  if (!isOpen) {
    drawerStore.resetForm();
  }
});
</script>

<style lang="scss" scoped>
.header-logo {
  width: 40px;
  height: 40px;
}

.header-title {
  font-weight: 500;
  letter-spacing: 0.5px;
  line-height: 1.2;
}

.header-subtitle {
  font-size: 10px;
  opacity: 0.7;
  letter-spacing: 0.3px;
}

.nav-list {
  padding: 8px;
}

.nav-item {
  border-radius: 100px;
  margin: 4px 8px;
  min-height: 56px;
}
</style>
