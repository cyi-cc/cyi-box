import { Client } from "./client";
import authSvc from "./authSvc";
import bookmarkSvc from "./bookmarkSvc";
import dashboardSvc from "./dashboardSvc";
import dbMgrSvc from "./dbMgrSvc";
import diskSvc from "./diskSvc";
import licenseSvc from "./licenseSvc";
import memorySvc from "./memorySvc";
import paySvc from "./paySvc";
import proxySvc from "./proxySvc";
import serverSvc from "./serverSvc";
import settingSvc from "./settingSvc";
import toolSvc from "./toolSvc";
import userSvc from "./userSvc";
import vaultSvc from "./vaultSvc";
import webSvc from "./webSvc";

export class defaultApi extends Client {
  constructor(url: string) {
    super(url);
  }
  public authSvc: authSvc = new authSvc(this);
  public bookmarkSvc: bookmarkSvc = new bookmarkSvc(this);
  public dashboardSvc: dashboardSvc = new dashboardSvc(this);
  public dbMgrSvc: dbMgrSvc = new dbMgrSvc(this);
  public diskSvc: diskSvc = new diskSvc(this);
  public licenseSvc: licenseSvc = new licenseSvc(this);
  public memorySvc: memorySvc = new memorySvc(this);
  public paySvc: paySvc = new paySvc(this);
  public proxySvc: proxySvc = new proxySvc(this);
  public serverSvc: serverSvc = new serverSvc(this);
  public settingSvc: settingSvc = new settingSvc(this);
  public toolSvc: toolSvc = new toolSvc(this);
  public userSvc: userSvc = new userSvc(this);
  public vaultSvc: vaultSvc = new vaultSvc(this);
  public webSvc: webSvc = new webSvc(this);
}

export default class api {
  static create(url: string): defaultApi {
    return new defaultApi(url);
  }
}