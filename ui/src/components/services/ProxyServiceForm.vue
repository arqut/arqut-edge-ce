<template>
  <q-form @submit="onSubmit" class="service-form q-pa-md">
    <q-card flat>
      <!-- Header -->
      <q-card-section>
        <div class="row items-center justify-between">
          <div class="text-title-large">
            {{ isEdit ? 'Edit Service' : 'Create Service' }}
          </div>
          <q-btn
            flat
            round
            icon="close"
            size="sm"
            @click="onCancel"
          />
        </div>
        <div v-if="isEdit" class="text-body-medium text-on-surface-variant q-mt-xs">
          {{ formData.name }}
        </div>
      </q-card-section>

      <q-separator />

      <!-- Form Fields -->
      <q-card-section>
        <div class="q-gutter-lg">
          <!-- Service Name -->
          <q-input
            v-model="formData.name"
            label="Service Name"
            outlined
            :disable="isRunning"
            :rules="[val => !!val || 'Name is required']"
            autofocus
          >
            <template v-slot:prepend>
              <q-icon name="label" />
            </template>
          </q-input>

          <!-- Protocol Selector -->
          <q-select
            v-model="formData.protocol"
            label="Protocol"
            :options="protocolOptions"
            outlined
            emit-value
            map-options
            :disable="isRunning"
          >
            <template v-slot:prepend>
              <q-icon name="http" />
            </template>
          </q-select>

          <!-- Host Input -->
          <q-input
            v-model="formData.local_host"
            label="Local Host"
            outlined
            :disable="isRunning"
            :rules="[val => !!val || 'Host is required']"
            hint="Use 'localhost' for local services"
          >
            <template v-slot:prepend>
              <q-icon name="computer" />
            </template>
          </q-input>

          <!-- Port Input -->
          <q-input
            v-model.number="formData.local_port"
            type="number"
            label="Local Port"
            outlined
            :disable="isRunning"
            :rules="[
              val => !!val || 'Port is required',
              val => val > 0 && val < 65536 || 'Port must be between 1 and 65535'
            ]"
          >
            <template v-slot:prepend>
              <q-icon name="lan" />
            </template>
          </q-input>

          <!-- Service URL Preview -->
          <div v-if="serviceUrl" class="url-preview q-pa-md">
            <div class="text-label-medium text-on-surface-variant q-mb-xs">
              Service URL
            </div>
            <div class="row items-center justify-between">
              <code class="text-body-medium text-primary">{{ serviceUrl }}</code>
              <q-btn
                flat
                round
                dense
                icon="content_copy"
                size="sm"
                @click="copyUrl"
              >
                <q-tooltip>Copy URL</q-tooltip>
              </q-btn>
            </div>
          </div>

          <!-- Extension point for EN form fields -->
          <slot name="additional-fields" :service="formData" />
        </div>
      </q-card-section>

      <!-- Warning for running services -->
      <q-card-section v-if="isRunning" class="q-pt-none">
        <q-banner class="warning-banner">
          <template v-slot:avatar>
            <q-icon name="warning" />
          </template>
          <div class="text-label-large">Service is running</div>
          <div class="text-body-small">Stop the service before making changes</div>
        </q-banner>
      </q-card-section>

      <q-separator />

      <!-- Actions -->
      <q-card-actions class="q-pa-md" align="right">
        <q-btn
          flat
          no-caps
          label="Cancel"
          class="action-btn"
          @click="onCancel"
        />
        <q-btn
          unelevated
          no-caps
          type="submit"
          label="Save"
          color="primary"
          class="action-btn"
          :loading="submitting"
          :disable="isRunning"
        />
      </q-card-actions>
    </q-card>
  </q-form>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { copyToClipboard } from 'quasar';
import { useProxyServicesStore } from '../../stores/proxyServices';
import { useDrawerStore } from '../../stores/drawer';
import { useUiStore } from '../../stores/ui';
import type { ProxyService } from '../../types/models';

interface Props {
  service: ProxyService;
}

const props = defineProps<Props>();

const proxyServices = useProxyServicesStore();
const drawer = useDrawerStore();
const ui = useUiStore();

const formData = ref<ProxyService>({ ...props.service });
const submitting = ref(false);

const protocolOptions = [
  { label: 'HTTP', value: 'http' },
  { label: 'WebSocket', value: 'ws' },
];

const isEdit = computed(() => !!props.service.id);
const isRunning = computed(() => formData.value.enabled);
const serviceUrl = computed(() => {
  if (!formData.value.protocol || !formData.value.local_host || !formData.value.local_port) {
    return '';
  }
  return `${formData.value.protocol}://${formData.value.local_host}:${formData.value.local_port}`;
});

function onCancel() {
  drawer.closeRight();
}

async function onSubmit() {
  submitting.value = true;
  try {
    const res = isEdit.value
      ? await proxyServices.updateService(formData.value)
      : await proxyServices.createService(formData.value);

    if (res.error) {
      ui.notifyError(res.message || 'Error saving service');
    } else {
      ui.notifySuccess(
        isEdit.value ? 'Service updated successfully' : 'Service created successfully'
      );
      drawer.closeRight();
    }
  } finally {
    submitting.value = false;
  }
}

function copyUrl() {
  copyToClipboard(serviceUrl.value)
    .then(() => {
      ui.notifySuccess('URL copied to clipboard');
    })
    .catch(() => {
      ui.notifyError('Failed to copy URL');
    });
}
</script>

<style lang="scss">
.service-form {
  min-width: 360px;

  .url-preview {
    background-color: #EAEBE5;
    border-radius: 8px;

    code {
      font-family: 'Roboto Mono', monospace;
      word-break: break-all;
    }
  }

  .warning-banner {
    background-color: #FFF3E0 !important;
    color: #E65100 !important;
    border-radius: 12px;
  }

  .action-btn {
    min-height: 40px;
    padding: 0 16px;
    min-width: 80px;
  }
}

body.body--dark .service-form {
  .url-preview {
    background-color: #282B25;
  }

  .warning-banner {
    background-color: #3D2600 !important;
    color: #FFB74D !important;
  }
}
</style>
