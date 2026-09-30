export default interface dbQueryDto {
  id:number
  database?:string | null
  sql:string
  maxRows?:number | null
}