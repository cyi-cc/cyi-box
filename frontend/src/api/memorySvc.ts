import { Client, type result, type RequestOptions } from "./client";
import type memoryKeyDto from "./memoryKeyDto";
import type memoryListDto from "./memoryListDto";
import type memoryListView from "./memoryListView";
import type memoryMcpView from "./memoryMcpView";
import type memoryProjectsView from "./memoryProjectsView";
import type memorySaveDto from "./memorySaveDto";
import type memorySaveView from "./memorySaveView";
import type memoryView from "./memoryView";
import type okView from "./okView";

export default class memorySvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async delete(dto:memoryKeyDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("memorySvc", "delete", dto, options)
  }
  async get(dto:memoryKeyDto, options?: RequestOptions): Promise<result<memoryView>> {
    return await this.client.request<memoryView>("memorySvc", "get", dto, options)
  }
  async list(dto:memoryListDto, options?: RequestOptions): Promise<result<memoryListView>> {
    return await this.client.request<memoryListView>("memorySvc", "list", dto, options)
  }
  async mcpInfo(options?: RequestOptions): Promise<result<memoryMcpView>> {
    return await this.client.request<memoryMcpView>("memorySvc", "mcpInfo", undefined, options)
  }
  async projects(options?: RequestOptions): Promise<result<memoryProjectsView>> {
    return await this.client.request<memoryProjectsView>("memorySvc", "projects", undefined, options)
  }
  async resetToken(options?: RequestOptions): Promise<result<memoryMcpView>> {
    return await this.client.request<memoryMcpView>("memorySvc", "resetToken", undefined, options)
  }
  async save(dto:memorySaveDto, options?: RequestOptions): Promise<result<memorySaveView>> {
    return await this.client.request<memorySaveView>("memorySvc", "save", dto, options)
  }
}