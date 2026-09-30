import { Client, type result, type RequestOptions } from "./client";
import type dbDatabasesView from "./dbDatabasesView";
import type dbQueryDto from "./dbQueryDto";
import type dbQueryView from "./dbQueryView";
import type dbScopeDto from "./dbScopeDto";
import type dbTablesView from "./dbTablesView";
import type dbconnIdDto from "./dbconnIdDto";
import type dbconnView from "./dbconnView";
import type okView from "./okView";
import type saveDbconnDto from "./saveDbconnDto";

export default class dbMgrSvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async databases(dto:dbScopeDto, options?: RequestOptions): Promise<result<dbDatabasesView>> {
    return await this.client.request<dbDatabasesView>("dbMgrSvc", "databases", dto, options)
  }
  async delete(dto:dbconnIdDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("dbMgrSvc", "delete", dto, options)
  }
  async list(options?: RequestOptions): Promise<result<dbconnView[]>> {
    return await this.client.request<dbconnView[]>("dbMgrSvc", "list", undefined, options)
  }
  async query(dto:dbQueryDto, options?: RequestOptions): Promise<result<dbQueryView>> {
    return await this.client.request<dbQueryView>("dbMgrSvc", "query", dto, options)
  }
  async save(dto:saveDbconnDto, options?: RequestOptions): Promise<result<dbconnView>> {
    return await this.client.request<dbconnView>("dbMgrSvc", "save", dto, options)
  }
  async tables(dto:dbScopeDto, options?: RequestOptions): Promise<result<dbTablesView>> {
    return await this.client.request<dbTablesView>("dbMgrSvc", "tables", dto, options)
  }
  async test(dto:dbconnIdDto, options?: RequestOptions): Promise<result<okView>> {
    return await this.client.request<okView>("dbMgrSvc", "test", dto, options)
  }
}