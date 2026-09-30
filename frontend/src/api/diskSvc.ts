import { Client, type result, type RequestOptions } from "./client";
import type createShareDto from "./createShareDto";
import type fileView from "./fileView";
import type idDto from "./idDto";
import type okView from "./okView";
import type shareView from "./shareView";

export default class diskSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async createShare(dto:createShareDto, options?: RequestOptions): Promise<result<shareView>> {
    return await this.client.request<shareView>("diskSvc", "createShare", dto, options)
  }
  async deleteFile(dto:idDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("diskSvc", "deleteFile", dto, options)
  }
  async deleteShare(dto:idDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("diskSvc", "deleteShare", dto, options)
  }
  async list(options?: RequestOptions): Promise<result<fileView[]>> {
    return await this.client.request<fileView[]>("diskSvc", "list", undefined, options)
  }
  async listShares(options?: RequestOptions): Promise<result<shareView[]>> {
    return await this.client.request<shareView[]>("diskSvc", "listShares", undefined, options)
  }
}