/// <reference path="0-bosun.ts" />
/// <reference path="tokenList.ts" />

class BitMeta {
    public Bits: number;
    public Name: string;
    public Desc: string;
    public Active: boolean;
}

class NewTokenController {
    public token: Token = new Token();
    public permissions: Array<BitMeta>;
    public roles: Array<BitMeta>;
    public status: string;

    public createdToken:string;

    public hasBits = (bits: number) => {
        return (bits & this.token.Role) != 0;
    }

    public setRole = (bits: number, event: any) => {
        _(this.permissions).each((perm) => {
            if (!event.currentTarget.checked) {
                perm.Active = false;
            } else {
                perm.Active = (perm.Bits & bits) != 0;
            }
        });
    }

    public getBits = () => {
        return _(this.permissions).reduce((sum, p) => sum + (p.Active ? p.Bits : 0), 0)
    }

    private cleanRoles() {
        //fix admin role that has extra bits corresponding to future permissions.
        //causes bit math to go crazy and overflow. 
        //prevents easily  making tokens that grant unknown future perms too.
        _(this.roles).each((role) => {
            var mask = 0;
            _(this.permissions).each((p) => {
                if ((p.Bits & role.Bits) != 0) {
                    mask |= p.Bits
                }
            })
            role.Bits = mask;
        })
    }

    public create(){
        this.token.Role = this.getBits();
        this.status = "Creating..."
        
        this.$http.post("/api/tokens", this.token).then(
            (resp: ng.IHttpPromiseCallbackArg<string>)=>{
                this.status = "";
                this.createdToken = resp.data.replace(/"/g,"")
            },
            (err) => {this.status = 'Unable to load roles: ' + err;}
        )
    }

    public encoded(){
        return encodeURIComponent(this.createdToken)
    }

    static $inject = ['$http'];
    constructor(private $http: ng.IHttpService) {
        this.status = "Loading..."
        this.$http.get("/api/roles").then(
            (resp: ng.IHttpPromiseCallbackArg<any>) => {
                this.status = "";
                this.permissions = resp.data.Permissions;
                this.roles = resp.data.Roles;
                this.cleanRoles();
            }, (err) => {
                this.status = 'Unable to load roles: ' + err;
            }
        )
    }
}

bosunApp.component("newToken", {
    controller: NewTokenController,
    controllerAs: "ct",
    templateUrl: "/partials/tokenNew.html"
})