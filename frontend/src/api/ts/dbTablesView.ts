import type dbColumnView from "./dbColumnView";
import type dbTableView from "./dbTableView";
export default interface dbTablesView {
  tables:dbTableView[]
  columns:dbColumnView[]
}