/// <reference path="moment.d.ts" />
/// <reference path="moment-duration-format.d.ts" />

//Represents an auth token
class Token {
    public Hash: string;
    public Description: string = "";
    public Role: number = 0;
    public User: string = "";
    public LastUsed: Moment;

    public Permissions: string[];
    public RoleName: string;
}

//metadata about a single role or permission
class BitMeta {
    public Bits: number;
    public Name: string;
    public Desc: string;
    public Active: boolean;
}

//all roles/permissions for bosun
class RoleDefs {
    public Permissions: Array<BitMeta>;
    public Roles: Array<BitMeta>;
}

// See models/incident.go Event (can't be event here because JS uses that)
class IncidentEvent {
    // Embedded properties of Result struct
    Value: number;
    Expr: string;
    Status: number;
    Time: string; // moment?
    Unevaluated: boolean;

    constructor(ie) {
        this.Value = ie.Value;
        this.Expr = ie.Expr;
        this.Status = ie.Status;
        this.Time = ie.Time;
        this.Unevaluated = ie.Unevaluated;
    }
}

class Annotation {
    Id: string;
    Message: string;
    StartDate: string; // RFC3999
    EndDate: string; // RFC3999
    CreationUser: string;
    Url: string;
    Source: string;
    Host: string;
    Owner: string;
    Category: string;

    constructor(a?, get?: boolean) {
        a = a || {};
        this.Id = a.Id || "";
        this.Message = a.Message || "";
        this.StartDate = a.StartDate || "";
        this.EndDate = a.EndDate || "";
        this.CreationUser = a.CreationUser || "";
        this.Url = a.Url || "";
        this.Source = a.Source || "bosun-ui";
        this.Host = a.Host || "";
        this.Owner = a.Owner || !get && getOwner() || "";
        this.Category = a.Category || "";
    }
    setTimeUTC() {
        var now = moment().utc().format(timeFormat)
        this.StartDate = now;
        this.EndDate = now;
    }
    setTime() {
        var now = moment().format(timeFormat)
        this.StartDate = now;
        this.EndDate = now;
    }
}

class Result {
    Value: number;
    Expr: string;

    constructor(r) {
        this.Value = r.Value;
        this.Expr = r.Expr;
    }
}

class Action {
    User: string;
    Message: string;
    Time: string; // moment?
    Type: string;
    Deadline: string; // moment?
    Fullfilled: boolean;
    Cancelled: boolean;

    constructor(a) {
        this.User = a.User;
        this.Message = a.Message;
        this.Time = a.Time;
        this.Type = a.Type;
        this.Deadline = a.Deadline;
        this.Cancelled = a.Cancelled;
        this.Fullfilled = a.Fullfilled;
    }
}


// See models/incident.go
class IncidentState {
    Id: number;
    Start: string; // moment object?
    End: string; // Pointer so nullable, also moment?
    AlertKey: string;
    Alert: string;

    // Embedded properties of Result struct
    Value: number;
    Expr: string;

    Events: IncidentEvent[];
    Actions: Action[];
    Tags: string;

    Subject: string;

    NeedAck: boolean;
    Open: boolean;
    Unevaluated: boolean;

    CurrentStatus: string;
    WorstStatus: string;

    LastAbnormalStatus: string;
    LastAbnormalTime: number; // Epoch

    constructor(is) {
        this.Id = is.Id;
        this.Start = is.Start;
        this.End = is.End;
        this.AlertKey = is.AlertKey;
        this.Alert = is.Alert;

        this.Value = is.Value;
        this.Expr = is.Expr;
        this.Events = new Array<IncidentEvent>();
        if (is.Events) {
            for (let e of is.Events) {
                this.Events.push(new IncidentEvent(e))
            }
        }
        this.Actions = new Array<Action>();
        this.Tags = is.Tags;
        if (is.Actions) {
            for (let a of is.Actions) {
                this.Actions.push(new Action(a))
            }
        }
        this.Subject = is.Subject;
        this.NeedAck = is.NeedAck;
        this.Open = is.Open;
        this.Unevaluated = is.Unevaluated;
        this.CurrentStatus = is.CurrentStatus;
        this.WorstStatus = is.WorstStatus;
        this.LastAbnormalStatus = is.LastAbnormalStatus;
        this.LastAbnormalTime = is.LastAbnormalTime;
    }


    IsPendingClose(): boolean {
        for (let action of this.Actions) {
            if (action.Deadline != undefined && !(action.Fullfilled || action.Cancelled)) {
                return true;
            }
        }
        return false;
    }
}

class StateGroup {
    Active: boolean;
    Status: string;
    CurrentStatus: string;
    Silenced: boolean;
    IsError: boolean;
    Subject: string;
    Alert: string;
    AlertKey: string;
    Ago: string;
    State: IncidentState;
    Children: StateGroup[];

    constructor(sg) {
        this.Active = sg.Active;
        this.Status = sg.Status;
        this.CurrentStatus = sg.CurrentStatus;
        this.Silenced = sg.Silenced;
        this.IsError = sg.IsError;
        this.Subject = sg.Subject;
        this.Alert = sg.Alert;
        this.AlertKey = sg.AlertKey;
        this.Ago = sg.Ago;
        if (sg.State) {
            this.State = new IncidentState(sg.State);
        }
        this.Children = new Array<StateGroup>();
        if (sg.Children) {
            for (let c of sg.Children) {
                this.Children.push(new StateGroup(c));
            }
        }
    }
}

class Groups {
    NeedAck: StateGroup[];
    Acknowledged: StateGroup[];

    constructor(g) {
        this.NeedAck = new Array<StateGroup>();
        if (g.NeedAck) {
            for (let sg of g.NeedAck) {
                this.NeedAck.push(new StateGroup(sg));
            }
        }
        this.Acknowledged = new Array<StateGroup>();
        if (g.Acknowledged) {
            for (let sg of g.Acknowledged) {
                this.Acknowledged.push(new StateGroup(sg));
            }
        }
    }
}


class StateGroups {
    Groups: Groups;
    TimeAndDate: number[];
    FailingAlerts: number;
    UnclosedErrors: number;

    constructor(sgs) {
        this.Groups = new Groups(sgs.Groups);
        this.TimeAndDate = sgs.TimeAndDate;
        this.FailingAlerts = sgs.FailingAlerts;
        this.UnclosedErrors = sgs.UnclosedErrors;
    }
}