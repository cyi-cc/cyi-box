import { Client, type result, type RequestOptions } from "./client";
import type captchaKeySaveDto from "./captchaKeySaveDto";
import type captchaKeyView from "./captchaKeyView";
import type idDto from "./idDto";
import type okView from "./okView";

export default class captchaSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async deleteKey(dto:idDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("captchaSvc", "deleteKey", dto, options)
  }
  async listKeys(options?: RequestOptions): Promise<result<captchaKeyView[]>> {
    return await this.client.request<captchaKeyView[]>("captchaSvc", "listKeys", undefined, options)
  }
  async saveKey(dto:captchaKeySaveDto, options?: RequestOptions): Promise<result<captchaKeyView>> {
    return await this.client.request<captchaKeyView>("captchaSvc", "saveKey", dto, options)
  }
}