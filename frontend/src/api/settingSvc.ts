import { Client, type result, type RequestOptions } from "./client";
import type okView from "./okView";
import type settingDto from "./settingDto";
import type settingView from "./settingView";

export default class settingSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async get(dto:settingDto, options?: RequestOptions): Promise<result<settingView>> {
    return await this.client.request<settingView>("settingSvc", "get", dto, options)
  }
  async set(dto:settingDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("settingSvc", "set", dto, options)
  }
}