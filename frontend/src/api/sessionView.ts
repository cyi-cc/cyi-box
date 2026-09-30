import type userView from "./userView";
export default interface sessionView {
  user?:userView | null
  token?:string | null
}