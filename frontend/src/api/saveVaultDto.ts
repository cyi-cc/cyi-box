export default interface saveVaultDto {
  id?:number | null
  title:string
  username?:string | null
  password?:string | null
  url?:string | null
  note?:string | null
  totpSecret?:string | null
}