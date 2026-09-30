export default interface saveToolDto {
  id?:number | null
  name:string
  icon?:string | null
  url:string
  description?:string | null
  sort?:number | null
  enabled?:boolean | null
}