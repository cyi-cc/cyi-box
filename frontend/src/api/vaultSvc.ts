import { Client, type result, type RequestOptions } from "./client";
import type okView from "./okView";
import type saveVaultDto from "./saveVaultDto";
import type secretView from "./secretView";
import type totpView from "./totpView";
import type vaultIdDto from "./vaultIdDto";
import type vaultItemView from "./vaultItemView";

export default class vaultSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async delete(dto:vaultIdDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("vaultSvc", "delete", dto, options)
  }
  async list(options?: RequestOptions): Promise<result<vaultItemView[]>> {
    return await this.client.request<vaultItemView[]>("vaultSvc", "list", undefined, options)
  }
  async reveal(dto:vaultIdDto, options?: RequestOptions): Promise<result<secretView>> {
    return await this.client.request<secretView>("vaultSvc", "reveal", dto, options)
  }
  async save(dto:saveVaultDto, options?: RequestOptions): Promise<result<vaultItemView>> {
    return await this.client.request<vaultItemView>("vaultSvc", "save", dto, options)
  }
  async totp(dto:vaultIdDto, options?: RequestOptions): Promise<result<totpView>> {
    return await this.client.request<totpView>("vaultSvc", "totp", dto, options)
  }
}