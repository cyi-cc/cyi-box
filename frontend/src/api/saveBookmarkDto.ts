export default interface saveBookmarkDto {
  id?:number | null
  title:string
  url:string
  icon?:string | null
  sort?:number | null
}