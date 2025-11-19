// API client utility

import axios, { AxiosInstance } from 'axios';
import type { ApiResponse } from '../types/api';

class ApiClient {
  private client: AxiosInstance;

  constructor(baseURL = '/api') {
    this.client = axios.create({
      baseURL,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Request interceptor
    this.client.interceptors.request.use(
      (config) => {
        // Add auth token if available
        const token = this.getToken();
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
      },
      (error) => Promise.reject(error)
    );

    // Response interceptor
    this.client.interceptors.response.use(
      (response) => response,
      (error) => {
        if (error.response?.status === 401) {
          // Handle unauthorized
          this.clearToken();
          window.location.href = '/#/login';
        }
        return Promise.reject(error);
      }
    );
  }

  private getToken(): string | null {
    // Check cookie for token
    const cookies = document.cookie.split(';');
    const tokenCookie = cookies.find((c) => c.trim().startsWith('arq_edge_token='));
    return tokenCookie ? tokenCookie.split('=')[1] : null;
  }

  private clearToken(): void {
    document.cookie = 'arq_edge_token=;path=/;max-age=0';
  }

  // Generic request methods
  async get<T = any>(url: string): Promise<ApiResponse<T>> {
    try {
      const { data } = await this.client.get<ApiResponse<T>>(url);
      return data;
    } catch (error: any) {
      return this.handleError(error);
    }
  }

  async post<T = any>(url: string, payload?: any): Promise<ApiResponse<T>> {
    try {
      const { data } = await this.client.post<ApiResponse<T>>(url, payload);
      return data;
    } catch (error: any) {
      return this.handleError(error);
    }
  }

  async put<T = any>(url: string, payload?: any): Promise<ApiResponse<T>> {
    try {
      const { data } = await this.client.put<ApiResponse<T>>(url, payload);
      return data;
    } catch (error: any) {
      return this.handleError(error);
    }
  }

  async patch<T = any>(url: string, payload?: any): Promise<ApiResponse<T>> {
    try {
      const { data } = await this.client.patch<ApiResponse<T>>(url, payload);
      return data;
    } catch (error: any) {
      return this.handleError(error);
    }
  }

  async delete<T = any>(url: string): Promise<ApiResponse<T>> {
    try {
      const { data } = await this.client.delete<ApiResponse<T>>(url);
      return data;
    } catch (error: any) {
      return this.handleError(error);
    }
  }

  private handleError(error: any): ApiResponse {
    const response = error.response?.data || {};
    return {
      success: false,
      error: {
        code: error.response?.status,
        message: response.error?.message || error.message || 'An error occurred',
        detail: response.error?.detail,
      },
    };
  }
}

export const api = new ApiClient();
