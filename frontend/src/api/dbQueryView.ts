export default interface dbQueryView {
  isSelect:boolean
  columns:string[]
  rowsJson:string
  rowCount:number
  affectedRows:number
  truncated:boolean
  elapsedMs:number
}