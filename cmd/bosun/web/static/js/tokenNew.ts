/// <reference path="0-bosun.ts" />

class NewTokenController {
    public token: Token = new Token();
    public permissions: Array<BitMeta>;
    public roles: Array<BitMeta>;
    public status: string;

    public createdToken: string;

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

    public create() {
        this.token.Role = this.getBits();
        this.status = "Creating..."

        this.$http.post("/api/tokens", this.token).then(
            (resp: ng.IHttpPromiseCallbackArg<string>) => {
                this.status = "";
                this.createdToken = resp.data.replace(/"/g, "")
            },
            (err) => { this.status = 'Unable to load roles: ' + err; }
        )
    }

    public encoded() {
        return encodeURIComponent(this.createdToken)
    }

    static $inject = ['$http', 'authService'];
    constructor(private $http: ng.IHttpService, private auth: IAuthService) {
        var defs = auth.GetRoles();
        this.permissions = defs.Permissions;
        this.roles = defs.Roles;
    }
}

bosunApp.component("newToken", {
    controller: NewTokenController,
    controllerAs: "ct",
    templateUrl: "/partials/tokenNew.html"
})