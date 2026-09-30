export default interface serverView {
  id:number
  name:string
  host:string
  port:number
  username:string
  authType:string
  hasSecret:boolean
  note?:string | null
  createdAt:number
}