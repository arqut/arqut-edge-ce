<template>
  <q-page class="services-page q-pa-md">
    <!-- Header -->
    <div class="row justify-between items-center q-mb-lg">
      <div class="text-title-large text-primary">Services</div>
      <div class="row q-gutter-sm">
        <!-- Extension point for EN header actions -->
        <slot name="header-actions" />

        <q-btn
          unelevated
          no-caps
          color="primary"
          icon="add"
          label="Create Service"
          @click="createService"
        />
      </div>
    </div>

    <!-- Services Card -->
    <q-card flat bordered class="services-card">
      <!-- Table Header -->
      <div class="table-header row q-px-md q-py-sm items-center">
        <div class="col-4 row items-center">
          <span class="text-label-large">Name</span>
        </div>
        <div v-if="isDesktop" class="col-2 row items-center justify-center">
          <span class="text-label-large">Status</span>
        </div>
        <div v-if="isDesktop" class="col-2 row items-center justify-center">
          <span class="text-label-large">Created</span>
        </div>
        <div class="col-4 row items-center justify-end">
          <q-btn
            flat
            round
            icon="refresh"
            :loading="proxyServices.loading"
            @click="proxyServices.loadServices()"
          >
            <q-tooltip>Refresh</q-tooltip>
          </q-btn>
        </div>
      </div>

      <q-separator />

      <!-- Loading State -->
      <div
        v-if="proxyServices.loading && services.length === 0"
        class="q-pa-md"
      >
        <q-skeleton type="rect" height="72px" class="q-mb-sm" />
        <q-skeleton type="rect" height="72px" class="q-mb-sm" />
        <q-skeleton type="rect" height="72px" />
      </div>

      <!-- Services List -->
      <q-list v-else-if="services.length > 0" separator class="services-list">
        <q-item
          v-for="service in services"
          :key="service.id"
          clickable
          class="service-item q-py-md"
        >
          <!-- Mobile Avatar -->
          <q-item-section v-if="isMobile" avatar top>
            <q-avatar
              :icon="service.enabled ? 'check_circle' : 'cancel'"
              :color="service.enabled ? 'positive' : 'grey-6'"
              text-color="white"
              size="40px"
            />
          </q-item-section>

          <!-- Service Name -->
          <q-item-section :class="isDesktop ? 'col-4' : ''">
            <q-item-label class="text-title-medium">
              {{ service.name }}
            </q-item-label>
            <q-item-label v-if="isMobile" caption class="text-body-small">
              {{ service.protocol }}://{{ service.local_host }}:{{
                service.local_port
              }}
            </q-item-label>
            <q-item-label v-if="isMobile && service.created_at" caption class="text-body-small">
              Created: {{ formatDate(service.created_at) }}
            </q-item-label>
          </q-item-section>

          <!-- Status Badge (Desktop) -->
          <q-item-section v-if="isDesktop" class="col-2 text-center ml-0">
            <div>
              <q-badge
                :color="service.enabled ? 'positive' : 'grey-6'"
                text-color="white"
              >
                {{ service.enabled ? 'Active' : 'Inactive' }}
              </q-badge>
            </div>
          </q-item-section>

          <!-- Created Date (Desktop) -->
          <q-item-section
            v-if="isDesktop && service.created_at"
            class="col-2 text-center ml-0"
          >
            <q-item-label caption class="text-body-small">
              {{ formatDate(service.created_at) }}
            </q-item-label>
          </q-item-section>

          <!-- Actions (Desktop) -->
          <q-item-section v-if="isDesktop" class="col-4 ml-0">
            <div class="row justify-end action-buttons">
              <!-- Extension point for EN service actions -->
              <slot name="service-actions" :service="service" />

              <q-btn
                flat
                round
                :icon="service.enabled ? 'stop_circle' : 'play_circle'"
                size="sm"
                @click.stop="toggleService(service)"
              >
                <q-tooltip>{{ service.enabled ? 'Stop' : 'Start' }}</q-tooltip>
              </q-btn>
              <q-btn
                flat
                round
                icon="edit"
                size="sm"
                @click.stop="updateService(service)"
              >
                <q-tooltip>Edit</q-tooltip>
              </q-btn>
              <q-btn
                flat
                round
                icon="delete_outline"
                size="sm"
                class="text-negative"
                @click.stop="deleteService(service)"
              >
                <q-tooltip>Delete</q-tooltip>
              </q-btn>
            </div>
          </q-item-section>

          <!-- Actions Menu (Mobile) -->
          <q-item-section v-if="isMobile" side>
            <q-btn dense flat round icon="more_vert" size="sm">
              <q-menu auto-close>
                <q-list dense style="min-width: 180px">
                  <q-item clickable @click="toggleService(service)">
                    <q-item-section avatar>
                      <q-icon :name="service.enabled ? 'stop_circle' : 'play_circle'" />
                    </q-item-section>
                    <q-item-section>
                      {{ service.enabled ? 'Stop' : 'Start' }}
                    </q-item-section>
                  </q-item>
                  <q-item clickable @click="updateService(service)">
                    <q-item-section avatar>
                      <q-icon name="edit" />
                    </q-item-section>
                    <q-item-section>Edit</q-item-section>
                  </q-item>

                  <!-- Extension point for EN mobile menu items -->
                  <slot name="mobile-menu-items" :service="service" />

                  <q-separator />
                  <q-item clickable @click="deleteService(service)">
                    <q-item-section avatar>
                      <q-icon name="delete_outline" color="negative" />
                    </q-item-section>
                    <q-item-section class="text-negative">Delete</q-item-section>
                  </q-item>
                </q-list>
              </q-menu>
            </q-btn>
          </q-item-section>
        </q-item>
      </q-list>

      <!-- Empty State -->
      <div v-else class="empty-state text-center q-pa-xl">
        <q-icon name="dns" size="64px" class="q-mb-md text-on-surface-variant" style="opacity: 0.5" />
        <div class="text-title-medium q-mb-sm">No services found</div>
        <div class="text-body-medium text-on-surface-variant q-mb-lg">
          Create your first service to get started
        </div>
        <q-btn
          unelevated
          no-caps
          color="primary"
          icon="add"
          label="Create Service"
          @click="createService"
        />
      </div>
    </q-card>

    <!-- Extension point for additional panels (EN analytics, etc.) -->
    <slot name="additional-panels" />
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, markRaw } from 'vue';
import { useQuasar } from 'quasar';
import { storeToRefs } from 'pinia';
import { useProxyServicesStore } from '../stores/proxyServices';
import { useDrawerStore } from '../stores/drawer';
import { useUiStore } from '../stores/ui';
import { formatDate } from '../utils/format';
import type { ProxyService } from '../types/models';
import ProxyServiceForm from '../components/services/ProxyServiceForm.vue';

const $q = useQuasar();
const proxyServices = useProxyServicesStore();
const drawer = useDrawerStore();
const ui = useUiStore();

const { services } = storeToRefs(proxyServices);

// Responsive breakpoints using Quasar's reactive screen object
// Desktop: >= 1024px (md breakpoint and above)
// Mobile: < 1024px (below md breakpoint)
const isDesktop = computed(() => $q.screen.gt.sm);
const isMobile = computed(() => $q.screen.lt.md);

onMounted(() => {
  ui.setTitle('Services');
  void proxyServices.loadServices();
});

function createService() {
  drawer.formComponent = markRaw(ProxyServiceForm);
  drawer.formProps = {
    service: {
      id: '',
      name: '',
      local_host: 'localhost',
      local_port: 80,
      protocol: 'http',
      enabled: false,
    } as ProxyService,
  };
  drawer.openRight();
}

async function toggleService(service: ProxyService) {
  const res = await proxyServices.toggleService(service);
  if (res.success) {
    ui.notifySuccess(service.enabled ? 'Service stopped' : 'Service started');
  } else {
    ui.notifyError(res.error?.message || 'Error toggling service');
  }
}

function updateService(service: ProxyService) {
  drawer.formComponent = markRaw(ProxyServiceForm);
  drawer.formProps = { service };
  drawer.openRight();
}

function deleteService(service: ProxyService) {
  ui.confirm(`Are you sure you want to delete "${service.name}"?`).onOk(() => {
    void proxyServices.deleteService(service).then((res) => {
      if (res.success) {
        ui.notifySuccess('Service deleted successfully');
      } else {
        ui.notifyError(res.error?.message || 'Error deleting service');
      }
    });
  });
}
</script>

<style lang="scss" scoped>
.services-page {
  max-width: 1200px;
  margin: 0 auto;
}

.services-card {
  overflow: hidden;
}

.table-header {
  min-height: 48px;
}

.services-list {
  .q-item {
    min-height: 72px;
  }
}

.empty-state {
  padding: 48px 24px;
}

.ml-0 {
  margin-left: 0px !important;
}
</style>
