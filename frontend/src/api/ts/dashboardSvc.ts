import { Client, type result, type RequestOptions } from "./client";
import type statsView from "./statsView";

export default class dashboardSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async stats(options?: RequestOptions): Promise<result<statsView>> {
    return await this.client.request<statsView>("dashboardSvc", "stats", undefined, options)
  }
}