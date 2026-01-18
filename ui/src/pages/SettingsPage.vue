<template>
  <main class="q-page q-layout-padding">
    <div class="row justify-between q-mb-md">
      <div class="text-h6 text-primary">Settings</div>
    </div>
    <q-card flat bordered class="round-25">
      <q-tabs
        v-model="tab"
        class="text-grey"
        active-color="primary"
        indicator-color="primary"
        align="justify"
        narrow-indicator
      >
        <q-tab name="integrations" label="Integrations" />
      </q-tabs>
      <q-separator />
      <q-tab-panels v-model="tab" animated>
        <q-tab-panel name="integrations">
          <q-list>
            <q-item-label header>
              <span class="text-weight-bold">Home Assistant Integration</span>
            </q-item-label>
            <q-item>
              <q-btn
                icon="private_connectivity"
                color="primary"
                label="Expose Home Assistant"
                size="md"
                :loading="exposing"
                @click="exposeHAAddon"
              >
                <template v-slot:loading>
                  <q-spinner-ios />
                </template>
              </q-btn>
            </q-item>

            <q-item>
              <q-item-section>
                <q-item-label>
                  <span>
                    By clicking the <b>Expose Home Assistant</b> button, the
                    next network configuration
                  </span>
                </q-item-label>
                <q-item-label class="text-weight-light">
                  <div class="code-block">
                    <pre>http:
  use_x_forwarded_for: true
  trusted_proxies:<template v-for="(subnet, index) in networkSubnet" :key="index">
    - {{ subnet }}</template></pre>
                  </div>
                </q-item-label>
                <q-item-label>
                  <span>
                    <p>
                      will be automatically added to your
                      <b>Home Assistant</b> configuration file
                      (configuration.yaml).
                    </p>
                    <div class="restart-instructions">
                      <p>
                        To make the tunnel functional, it is strongly
                        recommended to restart Home Assistant so that the new
                        settings are applied properly.
                      </p>
                      <p>How to Restart Home Assistant:</p>
                      <ol style="list-style-type: decimal">
                        <li>
                          Go to the
                          <a
                            href="http://homeassistant.local:8123/config/system"
                            target="_blank"
                            rel="noopener noreferrer"
                            ><b>Home Assistant UI</b></a
                          >.
                        </li>
                        <li>Click on <b>Settings</b> in the left sidebar.</li>
                        <li>
                          Navigate to <b>System &gt;</b>
                          <span style="vertical-align: middle">
                            <svg
                              xmlns="http://www.w3.org/2000/svg"
                              height="20px"
                              viewBox="0 -960 960 960"
                              width="20px"
                              fill="#1f1f1f"
                            >
                              <path
                                d="M480.21-480q-15.21 0-25.71-10.35T444-516v-312q0-15.3 10.29-25.65Q464.58-864 479.79-864t25.71 10.35Q516-843.3 516-828v312q0 15.3-10.29 25.65Q495.42-480 480.21-480ZM480-144q-70 0-131.13-26.6-61.14-26.6-106.4-71.87-45.27-45.26-71.87-106.4Q144-410 144-480q0-58.57 20-113.79Q184-649 221-694q10-13 25-14.5t26 9.5q10 11 11 25.5t-7 25.5q-29.45 36-44.73 79Q216-526 216-480q0 110.31 76.78 187.16 76.78 76.84 187 76.84T667-292.84q77-76.85 77-187.16 0-47-16-89.5T683-649q-9-11-8-25.5t12-25.5q10-11 25-9t25.78 14.36Q776-649 796-594t20 114q0 70-26.6 131.13-26.6 61.14-71.87 106.4-45.26 45.27-106.4 71.87Q550-144 480-144Z"
                              />
                            </svg>
                          </span>
                          (on the top-right).
                        </li>
                        <li>
                          Click <b>Restart Home Assistant</b> and confirm when
                          prompted.
                        </li>
                      </ol>
                      <br />
                      After Home Assistant restarts, the tunnel should be active
                      and ready to use.
                    </div>
                  </span>
                </q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
        </q-tab-panel>
      </q-tab-panels>
    </q-card>
  </main>
</template>

<script setup lang="ts">
import { useQuasar } from 'quasar';
import { onMounted, computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useIntegrationsStore } from '../stores/integrations';

const integrations = useIntegrationsStore();
const q = useQuasar();
const router = useRouter();

const tab = ref('integrations');
const exposing = ref(false);

const networkSubnet = computed(
  () => integrations.networkSettings?.subnets || [],
);

onMounted(async () => {
  await integrations.fetchNetworkSettings();
});

const exposeHAAddon = async () => {
  exposing.value = true;
  try {
    const res = await integrations.exposeHAAddon();
    if (res.success) {
      q.notify({
        color: 'positive',
        message: 'Home Assistant has been exposed via tunnel successfully.',
      });
      void router.push('/services');
    } else if (res.error?.code === 409) {
      q.notify({
        color: 'warning',
        message: 'Home Assistant has already been exposed via tunnel.',
      });
    } else {
      q.notify({
        color: 'negative',
        message: res.error?.message || 'Failed to expose Home Assistant.',
      });
    }
  } finally {
    exposing.value = false;
  }
};
</script>

<style scoped>
.code-block {
  position: relative;
  background: #f5f5f5;
  border: 1px solid #e0e0e0;
  border-radius: 0.5rem;
  padding: 1rem;
  overflow: auto;
}

.code-block pre {
  margin: 0;
  color: #2d2d2d;
  font-family: SFMono-Regular, Consolas, Menlo, monospace;
}

.restart-instructions {
  margin-top: 1rem;
}

.restart-instructions ol {
  padding-left: 1.5rem;
}

.restart-instructions li {
  margin-bottom: 0.5rem;
}
</style>
