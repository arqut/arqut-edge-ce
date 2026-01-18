import { defineStore } from 'pinia';
import { ref } from 'vue';
import { api } from '../utils/api';

export const useIntegrationsStore = defineStore('integrations', () => {
  const networkSettings = ref<{ subnets: string[] } | null>(null);

  const fetchNetworkSettings = async () => {
    if (!networkSettings.value) {
      const res = await api.get<{ subnets: string[] }>('/integrations/network');
      if (res.success && res.data) {
        networkSettings.value = res.data;
        if (networkSettings.value?.subnets) {
          // Remove duplicates
          networkSettings.value.subnets = [
            ...new Set(networkSettings.value.subnets),
          ];
        }
      }
    }
  };

  const exposeHAAddon = async () => {
    return api.post('/config/haaddon/expose');
  };

  return {
    networkSettings,
    fetchNetworkSettings,
    exposeHAAddon,
  };
});
