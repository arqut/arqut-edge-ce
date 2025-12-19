<template>
  <q-page class="q-pa-md">
    <!-- Header -->
    <div class="row justify-between items-center q-mb-md">
      <div class="text-h6 text-primary">Services</div>
      <div class="row q-gutter-sm">
        <!-- Extension point for EN header actions -->
        <slot name="header-actions" />

        <q-btn
          no-caps
          color="primary"
          icon="add"
          label="Create Service"
          @click="createService"
        />
      </div>
    </div>

    <!-- Services List -->
    <q-card flat bordered class="">
      <!-- Table Header -->
      <div class="row q-px-md q-py-sm">
        <div class="col-4 row items-center">
          <span class="text-subtitle2">Name</span>
        </div>
        <div class="col-2 row items-center justify-center desktop-only">
          <span class="text-subtitle2">Status</span>
        </div>
        <div class="col-2 row items-center justify-center desktop-only">
          <span class="text-subtitle2">Created</span>
        </div>
        <div class="col-4 row items-center justify-end">
          <q-btn
            flat
            round
            icon="refresh"
            :loading="proxyServices.loading"
            @click="proxyServices.loadServices()"
          />
        </div>
      </div>

      <q-separator />

      <!-- Loading State -->
      <div
        v-if="proxyServices.loading && services.length === 0"
        class="q-pa-md"
      >
        <q-skeleton type="rect" height="60px" class="q-mb-md" />
        <q-skeleton type="rect" height="60px" class="q-mb-md" />
        <q-skeleton type="rect" height="60px" />
      </div>

      <!-- Services List -->
      <q-list v-else-if="services.length > 0" separator>
        <q-item
          v-for="service in services"
          :key="service.id"
          clickable
          class="service-item q-py-md"
        >
          <!-- Mobile Avatar -->
          <q-item-section avatar top class="mobile-only">
            <q-avatar
              :icon="service.enabled ? 'check_circle' : 'cancel'"
              :color="service.enabled ? 'positive' : 'grey'"
              text-color="white"
            />
          </q-item-section>

          <!-- Service Name -->
          <q-item-section class="col-md-4">
            <q-item-label class="text-subtitle2">
              {{ service.name }}
            </q-item-label>
            <q-item-label caption class="mobile-only">
              {{ service.protocol }}://{{ service.local_host }}:{{
                service.local_port
              }}
            </q-item-label>
            <q-item-label v-if="service.created_at" caption class="mobile-only">
              Created: {{ formatDate(service.created_at) }}
            </q-item-label>
          </q-item-section>

          <!-- Status Badge (Desktop) -->
          <q-item-section class="col-2 text-center desktop-only ml-0">
            <div>
              <q-badge
                :color="service.enabled ? 'positive' : 'grey'"
                text-color="white"
              >
                {{ service.enabled ? 'Active' : 'Inactive' }}
              </q-badge>
            </div>
          </q-item-section>

          <!-- Created Date (Desktop) -->
          <q-item-section
            v-if="service.created_at"
            class="col-2 text-center desktop-only ml-0"
          >
            <q-item-label caption>
              {{ formatDate(service.created_at) }}
            </q-item-label>
          </q-item-section>

          <!-- Actions (Desktop) -->
          <q-item-section class="col-4 desktop-only ml-0">
            <div class="row justify-end action-buttons">
              <!-- Extension point for EN service actions -->
              <slot name="service-actions" :service="service" />

              <q-btn
                flat
                round
                :icon="service.enabled ? 'stop_circle' : 'play_circle'"
                color="grey"
                size="sm"
                @click.stop="toggleService(service)"
              >
                <q-tooltip>{{ service.enabled ? 'Stop' : 'Start' }}</q-tooltip>
              </q-btn>
              <q-btn
                flat
                round
                icon="edit"
                color="grey"
                size="sm"
                @click.stop="updateService(service)"
              >
                <q-tooltip>Edit</q-tooltip>
              </q-btn>
              <q-btn
                flat
                round
                icon="delete_forever"
                color="grey"
                size="sm"
                @click.stop="deleteService(service)"
              >
                <q-tooltip>Delete</q-tooltip>
              </q-btn>
            </div>
          </q-item-section>

          <!-- Actions Menu (Mobile) -->
          <q-item-section side class="mobile-only">
            <q-btn dense flat round icon="more_vert" size="sm">
              <q-menu auto-close>
                <q-list dense style="min-width: 150px">
                  <q-item clickable @click="toggleService(service)">
                    <q-item-section>
                      {{ service.enabled ? 'Stop' : 'Start' }}
                    </q-item-section>
                  </q-item>
                  <q-item clickable @click="updateService(service)">
                    <q-item-section>Edit</q-item-section>
                  </q-item>

                  <!-- Extension point for EN mobile menu items -->
                  <slot name="mobile-menu-items" :service="service" />

                  <q-separator />
                  <q-item clickable @click="deleteService(service)">
                    <q-item-section class="text-negative"
                      >Delete</q-item-section
                    >
                  </q-item>
                </q-list>
              </q-menu>
            </q-btn>
          </q-item-section>
        </q-item>
      </q-list>

      <!-- Empty State -->
      <div v-else class="text-center text-grey text-italic q-pa-lg">
        No services found. Create your first service to get started.
      </div>
    </q-card>

    <!-- Extension point for additional panels (EN analytics, etc.) -->
    <slot name="additional-panels" />
  </q-page>
</template>

<script setup lang="ts">
import { onMounted, markRaw } from 'vue';
import { storeToRefs } from 'pinia';
import { useProxyServicesStore } from '../stores/proxyServices';
import { useDrawerStore } from '../stores/drawer';
import { useUiStore } from '../stores/ui';
import { formatDate } from '../utils/format';
import type { ProxyService } from '../types/models';
import ProxyServiceForm from '../components/services/ProxyServiceForm.vue';

const proxyServices = useProxyServicesStore();
const drawer = useDrawerStore();
const ui = useUiStore();

const { services } = storeToRefs(proxyServices);

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
<style scoped>
.ml-0 {
  margin-left: 0px !important;
}
</style>
