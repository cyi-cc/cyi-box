import { Client, type result, type RequestOptions, type StreamOptions } from "./client";
import type payCashierDto from "./payCashierDto";
import type payCashierStatusView from "./payCashierStatusView";
import type payCashierView from "./payCashierView";
import type payOrderListDto from "./payOrderListDto";
import type payOrderPageView from "./payOrderPageView";
import type payStatsView from "./payStatsView";
import type payTestPayDto from "./payTestPayDto";
import type payTestPayView from "./payTestPayView";
import type payUpstreamDto from "./payUpstreamDto";
import type payUpstreamStatusView from "./payUpstreamStatusView";

export default class paySvc {
  private client: Client;
  constructor(client: Client) {
    this.client = client;
  }
  async cashierInfo(dto:payCashierDto, options?: RequestOptions): Promise<result<payCashierView>> {
    return await this.client.request<payCashierView>("paySvc", "cashierInfo", dto, options)
  }
  async cashierStatus(dto:payCashierDto, options?: RequestOptions): Promise<result<payCashierStatusView>> {
    return await this.client.request<payCashierStatusView>("paySvc", "cashierStatus", dto, options)
  }
  async list(dto:payOrderListDto, options?: RequestOptions): Promise<result<payOrderPageView>> {
    return await this.client.request<payOrderPageView>("paySvc", "list", dto, options)
  }
  async saveUpstream(dto:payUpstreamDto, options?: RequestOptions): Promise<result<payUpstreamStatusView>> {
    return await this.client.request<payUpstreamStatusView>("paySvc", "saveUpstream", dto, options)
  }
  async stats(options?: RequestOptions): Promise<result<payStatsView>> {
    return await this.client.request<payStatsView>("paySvc", "stats", undefined, options)
  }
  async testPay(dto:payTestPayDto, options?: RequestOptions): Promise<result<payTestPayView>> {
    return await this.client.request<payTestPayView>("paySvc", "testPay", dto, options)
  }
  async upstreamStatus(options?: RequestOptions): Promise<result<payUpstreamStatusView>> {
    return await this.client.request<payUpstreamStatusView>("paySvc", "upstreamStatus", undefined, options)
  }
  async watch(dto:payCashierDto | (() => payCashierDto), onMessage: (data: payCashierStatusView) => unknown, options?: StreamOptions): Promise<result<void>> {
    return await this.client.stream<payCashierStatusView>("paySvc", "watch", dto, onMessage, options)
  }
}