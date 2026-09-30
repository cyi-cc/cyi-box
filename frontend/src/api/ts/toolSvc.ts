import { Client, type result, type RequestOptions } from "./client";
import type deleteToolDto from "./deleteToolDto";
import type okView from "./okView";
import type saveToolDto from "./saveToolDto";
import type toolView from "./toolView";

export default class toolSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async adminList(options?: RequestOptions): Promise<result<toolView[]>> {
    return await this.client.request<toolView[]>("toolSvc", "adminList", undefined, options)
  }
  async deleteTool(dto:deleteToolDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("toolSvc", "deleteTool", dto, options)
  }
  async list(options?: RequestOptions): Promise<result<toolView[]>> {
    return await this.client.request<toolView[]>("toolSvc", "list", undefined, options)
  }
  async saveTool(dto:saveToolDto, options?: RequestOptions): Promise<result<toolView>> {
    return await this.client.request<toolView>("toolSvc", "saveTool", dto, options)
  }
}