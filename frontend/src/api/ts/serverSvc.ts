import { Client, type result, type RequestOptions } from "./client";
import type okView from "./okView";
import type saveServerDto from "./saveServerDto";
import type serverIdDto from "./serverIdDto";
import type serverMetricsView from "./serverMetricsView";
import type serverView from "./serverView";
import type sftpListView from "./sftpListView";
import type sftpPathDto from "./sftpPathDto";
import type sftpRenameDto from "./sftpRenameDto";

export default class serverSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async delete(dto:serverIdDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("serverSvc", "delete", dto, options)
  }
  async list(options?: RequestOptions): Promise<result<serverView[]>> {
    return await this.client.request<serverView[]>("serverSvc", "list", undefined, options)
  }
  async metrics(dto:serverIdDto, options?: RequestOptions): Promise<result<serverMetricsView>> {
    return await this.client.request<serverMetricsView>("serverSvc", "metrics", dto, options)
  }
  async save(dto:saveServerDto, options?: RequestOptions): Promise<result<serverView>> {
    return await this.client.request<serverView>("serverSvc", "save", dto, options)
  }
  async sftpList(dto:sftpPathDto, options?: RequestOptions): Promise<result<sftpListView>> {
    return await this.client.request<sftpListView>("serverSvc", "sftpList", dto, options)
  }
  async sftpMkdir(dto:sftpPathDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("serverSvc", "sftpMkdir", dto, options)
  }
  async sftpRemove(dto:sftpPathDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("serverSvc", "sftpRemove", dto, options)
  }
  async sftpRename(dto:sftpRenameDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("serverSvc", "sftpRename", dto, options)
  }
}