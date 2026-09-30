import { Client, type result, type RequestOptions } from "./client";
import type changePasswordDto from "./changePasswordDto";
import type loginDto from "./loginDto";
import type okView from "./okView";
import type sessionView from "./sessionView";

export default class authSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async changePassword(dto:changePasswordDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("authSvc", "changePassword", dto, options)
  }
  async login(dto:loginDto, options?: RequestOptions): Promise<result<sessionView>> {
    return await this.client.request<sessionView>("authSvc", "login", dto, options)
  }
  async logout(options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("authSvc", "logout", undefined, options)
  }
  async me(options?: RequestOptions): Promise<result<sessionView>> {
    return await this.client.request<sessionView>("authSvc", "me", undefined, options)
  }
}