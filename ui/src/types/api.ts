// API response types matching backend schema

export interface ApiResponseMeta {
  requestId?: string;
  timestamp?: string;
  ordering?: Record<string, any>;
  pagination?: Pagination;
}

export interface Pagination {
  page: number;
  perPage: number;
  total: number;
  totalPages: number;
}

export interface ApiError {
  code?: number;
  message: string;
  detail?: any;
}

export interface ApiResponse<T = any> {
  success: boolean;
  data?: T;
  error?: ApiError;
  meta?: ApiResponseMeta;
}
