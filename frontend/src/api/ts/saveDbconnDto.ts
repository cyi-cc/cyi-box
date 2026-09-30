export default interface saveDbconnDto {
  id?:number | null
  name:string
  engine:string
  host?:string | null
  port?:number | null
  username?:string | null
  password?:string | null
  database?:string | null
  params?:string | null
}