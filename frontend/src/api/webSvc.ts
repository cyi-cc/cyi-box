import { Client, type result, type RequestOptions } from "./client";
import type webFetchDto from "./webFetchDto";
import type webFetchView from "./webFetchView";

export default class webSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async fetch(dto:webFetchDto, options?: RequestOptions): Promise<result<webFetchView>> {
    return await this.client.request<webFetchView>("webSvc", "fetch", dto, options)
  }
}