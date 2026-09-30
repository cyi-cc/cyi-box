export default interface proxyStatsView {
  total:number
  regions:number
  lastSyncAt:number
  lastCheckAt:number
  lastStatus:string
  checking:boolean
  syncing:boolean
}