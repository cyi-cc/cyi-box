export default interface saveServerDto {
  id?:number | null
  name:string
  host:string
  port?:number | null
  username:string
  authType:string
  secret?:string | null
  note?:string | null
}