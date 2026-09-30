import type sftpEntryView from "./sftpEntryView";
export default interface sftpListView {
  path:string
  entries:sftpEntryView[]
}