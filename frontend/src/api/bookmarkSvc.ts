import { Client, type result, type RequestOptions } from "./client";
import type bookmarkIdDto from "./bookmarkIdDto";
import type bookmarkView from "./bookmarkView";
import type okView from "./okView";
import type saveBookmarkDto from "./saveBookmarkDto";

export default class bookmarkSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async delete(dto:bookmarkIdDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("bookmarkSvc", "delete", dto, options)
  }
  async list(options?: RequestOptions): Promise<result<bookmarkView[]>> {
    return await this.client.request<bookmarkView[]>("bookmarkSvc", "list", undefined, options)
  }
  async save(dto:saveBookmarkDto, options?: RequestOptions): Promise<result<bookmarkView>> {
    return await this.client.request<bookmarkView>("bookmarkSvc", "save", dto, options)
  }
}