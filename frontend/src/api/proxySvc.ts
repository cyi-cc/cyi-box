import { Client, type result, type RequestOptions } from "./client";
import type idDto from "./idDto";
import type okView from "./okView";
import type proxyKeySaveDto from "./proxyKeySaveDto";
import type proxyKeyView from "./proxyKeyView";
import type proxyListDto from "./proxyListDto";
import type proxyListView from "./proxyListView";
import type proxyRegionView from "./proxyRegionView";
import type proxyStatsView from "./proxyStatsView";
import type proxySyncView from "./proxySyncView";

export default class proxySvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async checkNow(options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("proxySvc", "checkNow", undefined, options)
  }
  async createKey(dto:proxyKeySaveDto, options?: RequestOptions): Promise<result<proxyKeyView>> {
    return await this.client.request<proxyKeyView>("proxySvc", "createKey", dto, options)
  }
  async deleteKey(dto:idDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("proxySvc", "deleteKey", dto, options)
  }
  async list(dto:proxyListDto, options?: RequestOptions): Promise<result<proxyListView>> {
    return await this.client.request<proxyListView>("proxySvc", "list", dto, options)
  }
  async listKeys(options?: RequestOptions): Promise<result<proxyKeyView[]>> {
    return await this.client.request<proxyKeyView[]>("proxySvc", "listKeys", undefined, options)
  }
  async regions(options?: RequestOptions): Promise<result<proxyRegionView[]>> {
    return await this.client.request<proxyRegionView[]>("proxySvc", "regions", undefined, options)
  }
  async stats(options?: RequestOptions): Promise<result<proxyStatsView>> {
    return await this.client.request<proxyStatsView>("proxySvc", "stats", undefined, options)
  }
  async syncNow(options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("proxySvc", "syncNow", undefined, options)
  }
  async syncs(options?: RequestOptions): Promise<result<proxySyncView[]>> {
    return await this.client.request<proxySyncView[]>("proxySvc", "syncs", undefined, options)
  }
}