import type payTrendPoint from "./payTrendPoint";
import type recentMemoryView from "./recentMemoryView";
import type recentOrderView from "./recentOrderView";
import type trendPoint from "./trendPoint";
export default interface statsView {
  totalUsers:number
  activeSessions:number
  todayLogins:number
  uptimeSeconds:number
  loginTrend:trendPoint[]
  servers:number
  proxies:number
  aliveProxies:number
  todayOrders:number
  todayMoney:number
  totalMoney:number
  pendingOrders:number
  licenseApps:number
  licenseCards:number
  licenseActive:number
  memories:number
  memoryProjects:number
  bookmarks:number
  vaultItems:number
  dbconns:number
  files:number
  payTrend:payTrendPoint[]
  recentOrders:recentOrderView[]
  recentMemories:recentMemoryView[]
}