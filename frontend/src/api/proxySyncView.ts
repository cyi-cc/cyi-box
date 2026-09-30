export default interface proxySyncView {
  id:number
  startedAt:number
  finishedAt:number
  status:string
  source:string
  total:number
  added:number
  removed:number
  updated:number
  detailJson:string
  error:string
}