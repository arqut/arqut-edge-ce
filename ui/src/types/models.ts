// Core data models

export interface ProxyService {
  id: string;
  name: string;
  local_host: string;
  local_port: number;
  protocol: 'http' | 'ws';
  enabled: boolean;
  created_at?: string;
  updated_at?: string;
  public_port?: number;
}

export interface User {
  id: string;
  username: string;
  role?: string;
}

export interface AuthInfo {
  userId: string;
  userName: string;
  token?: string;
}
