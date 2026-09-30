import type diskMountView from "./diskMountView";
export default interface serverMetricsView {
  hostname:string
  os:string
  kernel:string
  uptimeSeconds:number
  cpuPercent:number
  load1:string
  load5:string
  load15:string
  memTotal:number
  memUsed:number
  memAvail:number
  swapTotal:number
  swapUsed:number
  diskTotal:number
  diskUsed:number
  diskAvail:number
  netRxRate:number
  netTxRate:number
  netRxTotal:number
  netTxTotal:number
  disks:diskMountView[]
  collectedAt:number
}