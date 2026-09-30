import { Client, type result, type RequestOptions } from "./client";
import type licenseAppSaveDto from "./licenseAppSaveDto";
import type licenseAppView from "./licenseAppView";
import type licenseAppsView from "./licenseAppsView";
import type licenseGenDto from "./licenseGenDto";
import type licenseGenView from "./licenseGenView";
import type licenseIdDto from "./licenseIdDto";
import type licenseListDto from "./licenseListDto";
import type licensePageView from "./licensePageView";
import type licenseStatsDto from "./licenseStatsDto";
import type licenseStatsView from "./licenseStatsView";
import type okView from "./okView";

export default class licenseSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async apps(options?: RequestOptions): Promise<result<licenseAppsView>> {
    return await this.client.request<licenseAppsView>("licenseSvc", "apps", undefined, options)
  }
  async delete(dto:licenseIdDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("licenseSvc", "delete", dto, options)
  }
  async deleteApp(dto:licenseIdDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("licenseSvc", "deleteApp", dto, options)
  }
  async generate(dto:licenseGenDto, options?: RequestOptions): Promise<result<licenseGenView>> {
    return await this.client.request<licenseGenView>("licenseSvc", "generate", dto, options)
  }
  async list(dto:licenseListDto, options?: RequestOptions): Promise<result<licensePageView>> {
    return await this.client.request<licensePageView>("licenseSvc", "list", dto, options)
  }
  async saveApp(dto:licenseAppSaveDto, options?: RequestOptions): Promise<result<licenseAppView>> {
    return await this.client.request<licenseAppView>("licenseSvc", "saveApp", dto, options)
  }
  async stats(dto:licenseStatsDto, options?: RequestOptions): Promise<result<licenseStatsView>> {
    return await this.client.request<licenseStatsView>("licenseSvc", "stats", dto, options)
  }
}