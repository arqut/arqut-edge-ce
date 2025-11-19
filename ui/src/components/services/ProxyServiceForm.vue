<template>
  <q-form @submit="onSubmit" class="q-pa-md">
    <q-card flat>
      <!-- Header -->
      <q-card-section>
        <div class="text-h6">
          {{ isEdit ? `Edit Service: ${formData.name}` : 'Create Service' }}
        </div>
      </q-card-section>

      <q-separator />

      <!-- Form Fields -->
      <q-card-section>
        <div class="q-gutter-md">
          <!-- Service Name -->
          <q-input
            v-model="formData.name"
            label="Service Name"
            outlined
            dense
            :disable="isRunning"
            :rules="[val => !!val || 'Name is required']"
            autofocus
          />

          <!-- Protocol Selector -->
          <q-select
            v-model="formData.protocol"
            label="Protocol"
            :options="['http', 'ws']"
            outlined
            dense
            :disable="isRunning"
          />

          <!-- Host Input -->
          <q-input
            v-model="formData.local_host"
            label="Local Host"
            outlined
            dense
            :disable="isRunning"
            :rules="[val => !!val || 'Host is required']"
            hint="Use 'localhost' for local services"
          />

          <!-- Port Input -->
          <q-input
            v-model.number="formData.local_port"
            type="number"
            label="Local Port"
            outlined
            dense
            :disable="isRunning"
            :rules="[
              val => !!val || 'Port is required',
              val => val > 0 && val < 65536 || 'Port must be between 1 and 65535'
            ]"
          />

          <!-- Service URL Preview -->
          <q-banner v-if="serviceUrl" class="bg-grey-2">
            <div class="text-caption text-grey-8">Service URL:</div>
            <div class="text-body2 text-primary">{{ serviceUrl }}</div>
            <template v-slot:action>
              <q-btn
                flat
                round
                dense
                icon="content_copy"
                @click="copyUrl"
              >
                <q-tooltip>Copy URL</q-tooltip>
              </q-btn>
            </template>
          </q-banner>

          <!-- Extension point for EN form fields -->
          <slot name="additional-fields" :service="formData" />
        </div>
      </q-card-section>

      <q-separator />

      <!-- Actions -->
      <q-card-actions align="right">
        <q-btn
          flat
          label="Cancel"
          color="grey"
          @click="onCancel"
        />
        <q-btn
          type="submit"
          label="Save"
          color="primary"
          :loading="submitting"
          :disable="isRunning"
        />
      </q-card-actions>

      <!-- Warning for running services -->
      <q-card-section v-if="isRunning">
        <q-banner class="bg-warning text-white">
          <template v-slot:avatar>
            <q-icon name="warning" color="white" />
          </template>
          This service is currently running. Stop it before making changes.
        </q-banner>
      </q-card-section>
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
