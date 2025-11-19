import { defineStore } from 'pinia';
import { Notify, Dialog, Loading, QSpinnerGears } from 'quasar';
import type { QNotifyCreateOptions } from 'quasar';

export const useUiStore = defineStore('ui', {
  state: () => ({
    title: '',
  }),

  actions: {
    setTitle(title: string) {
      this.title = title;
      document.title = title ? `Arqut Edge - ${title}` : 'Arqut Edge';
    },

    showLoading(message: string | boolean = true) {
      Loading.show({
        message: message === true ? 'Please wait...' : message.toString(),
        boxClass: 'bg-grey-2 text-grey-9',
        spinnerColor: 'primary',
        spinner: QSpinnerGears,
      });
    },

    hideLoading() {
      Loading.hide();
    },

    notify(options: QNotifyCreateOptions | string) {
      if (typeof options === 'string') {
        Notify.create({ message: options });
      } else {
        Notify.create(options);
      }
    },

    notifyError(message: string = 'Error! Please try again.') {
      Notify.create({
        message,
        type: 'negative',
        html: true,
      });
    },

    notifyWarning(message: string) {
      Notify.create({
        message,
        type: 'warning',
        html: true,
      });
    },

    notifySuccess(message: string) {
      Notify.create({
        message,
        type: 'positive',
        html: true,
      });
    },

    alert(message: string) {
      return Dialog.create({
        title: 'Alert',
        html: true,
        message,
      });
    },

    confirm(message: string, cancel = true, persistent = true) {
      return Dialog.create({
        title: 'Confirm',
        html: true,
        message,
        cancel,
        persistent,
      });
    },
  },
});
