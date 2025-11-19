import { defineStore } from 'pinia';
import type { Component } from 'vue';

export const useDrawerStore = defineStore('drawer', {
  state: () => ({
    width: 400,
    rightOpen: false,
    formComponent: null as Component | null,
    formProps: {} as Record<string, any>,
  }),

  actions: {
    openRight() {
      // Calculate responsive width
      let widthCoef = 1;
      if (window.innerWidth > 600) {
        widthCoef = 0.7;
      }
      if (window.innerWidth > 900) {
        widthCoef = 0.55;
      }
      if (window.innerWidth > 1200) {
        widthCoef = 0.4;
      }
      this.width = Math.floor(window.innerWidth * widthCoef);
      this.rightOpen = true;
    },

    closeRight() {
      this.rightOpen = false;
      this.resetForm();
    },

    resetForm() {
      this.formComponent = null;
      this.formProps = {};
    },
  },
});
