import { defineStore } from 'pinia';
import { ref } from 'vue';
import { api } from '../utils/api';
import type { ProxyService } from '../types/models';
import type { ApiResponse } from '../types/api';

export const useProxyServicesStore = defineStore('proxyServices', () => {
  const services = ref<ProxyService[]>([]);
  const loading = ref(false);

  async function loadServices(): Promise<ApiResponse<ProxyService[]>> {
    loading.value = true;
    try {
      const res = await api.get<ProxyService[]>('/services');
      if (res.success && res.data) {
        services.value = res.data;
      }
      return res;
    } finally {
      loading.value = false;
    }
  }

  async function createService(
    service: Partial<ProxyService>,
  ): Promise<ApiResponse<ProxyService>> {
    const res = await api.post<ProxyService>('/services', service);
    if (res.success) {
      await loadServices();
    }
    return res;
  }

  async function updateService(
    service: ProxyService,
  ): Promise<ApiResponse<ProxyService>> {
    const res = await api.put<ProxyService>(`/services/${service.id}`, service);
    if (res.success) {
      await loadServices();
    }
    return res;
  }

  async function deleteService(
    service: ProxyService,
  ): Promise<ApiResponse<void>> {
    const res = await api.delete<void>(`/services/${service.id}`);
    if (res.success) {
      await loadServices();
    }
    return res;
  }

  async function toggleService(
    service: ProxyService,
  ): Promise<ApiResponse<ProxyService>> {
    const action = service.enabled ? 'disable' : 'enable';
    const res = await api.patch<ProxyService>(
      `/services/${service.id}/${action}`,
    );
    if (res.success) {
      await loadServices();
    }
    return res;
  }

  return {
    services,
    loading,
    loadServices,
    createService,
    updateService,
    deleteService,
    toggleService,
  };
});
