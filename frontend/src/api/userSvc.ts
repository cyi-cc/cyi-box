import { Client, type result, type RequestOptions } from "./client";
import type deleteUserDto from "./deleteUserDto";
import type listUsersDto from "./listUsersDto";
import type okView from "./okView";
import type saveUserDto from "./saveUserDto";
import type userPageView from "./userPageView";
import type userView from "./userView";

export default class userSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async deleteUser(dto:deleteUserDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("userSvc", "deleteUser", dto, options)
  }
  async listUsers(dto:listUsersDto, options?: RequestOptions): Promise<result<userPageView>> {
    return await this.client.request<userPageView>("userSvc", "listUsers", dto, options)
  }
  async saveUser(dto:saveUserDto, options?: RequestOptions): Promise<result<userView>> {
    return await this.client.request<userView>("userSvc", "saveUser", dto, options)
  }
}