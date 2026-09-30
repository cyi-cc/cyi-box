export default interface vaultItemView {
  id:number
  title:string
  username?:string | null
  url?:string | null
  note?:string | null
  hasPassword:boolean
  hasTotp:boolean
  updatedAt:number
}